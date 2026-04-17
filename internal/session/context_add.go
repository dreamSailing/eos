package session

import (
	"encoding/json"
	"fmt"
	"strings"
	"github.com/dreamSailing/eos/internal/ai"
	codectx "github.com/dreamSailing/eos/internal/context"
	"github.com/dreamSailing/eos/internal/tools"
)

// AddPinned 添加固定消息
func (c *ContextManager) AddPinned(msg ai.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pinned = append(c.pinned, msg)
}

// AddUser 添加用户消息
func (c *ContextManager) AddUser(content string) {
	c.AddUserWithImages(content, nil)
}

// AddUserWithImages 添加带图片的用户消息
func (c *ContextManager) AddUserWithImages(content string, imagePaths []string) {
	processor := codectx.NewInputProcessor()
	_, cleaned, hint := processor.ProcessInput(content)

	c.mu.Lock()
	defer c.mu.Unlock()

	msg := ai.Message{
		Role:       "user",
		Content:    cleaned,
		ImagePaths: imagePaths,
	}
	c.recent = append(c.recent, msg)

	if hint != "" {
		c.ephem = append(c.ephem, hint)
	}
}

// AddAssistant 添加助手消息
func (c *ContextManager) AddAssistant(content string) {
	if strings.TrimSpace(content) == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recent = append(c.recent, ai.Message{Role: "assistant", Content: content})
}

// AddEphemeral 添加临时消息
func (c *ContextManager) AddEphemeral(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ephem = append(c.ephem, text)
}

// AddToolSummary 添加工具摘要
func (c *ContextManager) AddToolSummary(raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.tools) >= c.toolLimit {
		return
	}
	if tools.ShouldCompress(raw) {
		ct := guessToolCompressType(raw)
		compressed, ok := tools.CompressToolOutput(raw, ct)
		if ok {
			raw = tools.FormatCompressedOutput(raw, compressed, ct)
		}
	}
	if len(raw) > 4000 {
		raw = tools.TruncateOutput(raw, 4000)
	}
	c.tools = append(c.tools, raw)
}

// AddToolFull 添加完整工具内容
func (c *ContextManager) AddToolFull(raw string) {
	if raw == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.currentFull = append(c.currentFull, ai.Message{Role: "system", Content: raw})
	// 检查是否需要自动压缩
	c.autoCompactIfNeededLocked()
}

// AddToolObservation 序列化对象为紧凑 JSON 并添加为系统观察
func (c *ContextManager) AddToolObservation(obj interface{}) {
	if obj == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addToolObservationLocked(obj)
}

// addToolObservationLocked 内部方法（调用前需持有锁）
// 格式化工具结果为人类可读文本，便于 AI 理解
func (c *ContextManager) addToolObservationLocked(obj interface{}) {
	var s string
	switch v := obj.(type) {
	case tools.ToolResult:
		s = formatToolResultReadable(v)
	default:
		bs, _ := json.Marshal(obj)
		s = string(bs)
	}

	maxLen := 4000
	if strings.Contains(s, "❌") || strings.Contains(s, "失败") || strings.Contains(s, "error") {
		maxLen = 16000
	}
	if len(s) > maxLen {
		s = tools.TruncateOutput(s, maxLen)
	}
	c.toolObs = append(c.toolObs, s)
	// 检查是否需要自动压缩
	c.autoCompactIfNeededLocked()
}

// formatToolResultReadable 将 ToolResult 格式化为人类可读文本
func formatToolResultReadable(r tools.ToolResult) string {
	var sb strings.Builder

	// 工具名 + 状态
	if r.Status == "success" {
		sb.WriteString("[" + r.Tool + "] ✅ 成功")
	} else {
		sb.WriteString("[" + r.Tool + "] ❌ 失败")
	}

	// 错误信息（优先显示）
	if e := strings.TrimSpace(r.Error); e != "" {
		sb.WriteString(": " + e)
	}

	// 显示信息
	if d := strings.TrimSpace(r.Display); d != "" {
		sb.WriteString("\n  " + d)
	}

	// 关键数据（仅显示重要字段）
	if len(r.Data) > 0 {
		for _, key := range []string{"path", "file", "mode", "matches", "count", "lines", "status", "candidate"} {
			if val, ok := r.Data[key]; ok {
				sb.WriteString("\n  " + key + ": " + formatDataValue(val))
			}
		}
		// 如果失败，显示所有数据帮助调试
		if r.Status != "success" {
			for k, val := range r.Data {
				// 跳过已显示的 key
				switch k {
				case "path", "file", "mode", "matches", "count", "lines", "status", "candidate":
					continue
				}
				sb.WriteString("\n  " + k + ": " + formatDataValue(val))
			}
		}
	}

	return sb.String()
}

// formatDataValue 格式化数据字段的值
func formatDataValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		if len(val) > 200 {
			return val[:197] + "..."
		}
		return val
	case float64:
		if val == float64(int(val)) {
			return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", val), "0"), ".")
		}
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		bs, _ := json.Marshal(val)
		s := string(bs)
		if len(s) > 200 {
			return s[:197] + "..."
		}
		return s
	}
}

func guessToolCompressType(raw string) string {
	if strings.HasPrefix(strings.TrimSpace(raw), "@") {
		return "file"
	}
	if strings.Contains(raw, "*** Begin Patch") || strings.Contains(raw, "diff --git") {
		return "diff"
	}
	if strings.Contains(raw, "Search results") || strings.Contains(raw, "files_with_matches") {
		return "search"
	}
	return "generic"
}
