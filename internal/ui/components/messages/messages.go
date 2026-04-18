package messages

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dreamSailing/eos/internal/ui/styles"

	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

// MessageType 消息类型
type MessageType string

const (
	MsgTypeUser     MessageType = "user"
	MsgTypeAI       MessageType = "ai"
	MsgTypeTool     MessageType = "tool"
	MsgTypeAgent    MessageType = "agent"
	MsgTypePlan     MessageType = "plan"
	MsgTypeThinking MessageType = "thinking"
	MsgTypeSystem   MessageType = "system"
	MsgTypeError    MessageType = "error"
	MsgTypeWarning  MessageType = "warning"
	MsgTypeInfo     MessageType = "info"
)

// Message 消息接口
type Message interface {
	Type() MessageType
	Render(s *styles.Styles, width int) string
}

// UserMessage 用户消息
type UserMessage struct {
	Content   string
	Timestamp time.Time
}

func (m *UserMessage) Type() MessageType { return MsgTypeUser }

func (m *UserMessage) Render(s *styles.Styles, width int) string {
	bw := bubbleWidth(width)

	wrappedLines := wrapText(m.Content, bw-4)

	var result strings.Builder
	for i, line := range wrappedLines {
		result.WriteString(s.MsgUser.Render(line))
		if i < len(wrappedLines)-1 {
			result.WriteString("\n")
		}
	}

	bubble := s.MsgUserBorder.Render(result.String())
	return lipgloss.Place(width, lipgloss.Height(bubble), lipgloss.Right, lipgloss.Top, bubble)
}

// AIMessage AI消息
type AIMessage struct {
	Content   string
	Timestamp time.Time
	Tokens    int
	Duration  time.Duration
	Done      bool
	CopyLabel string
}

func (m *AIMessage) Type() MessageType { return MsgTypeAI }

func (m *AIMessage) Render(s *styles.Styles, width int) string {
	var result strings.Builder
	bw := bubbleWidth(width)

	// 头部
	header := s.MsgAIHeader.Render(fmt.Sprintf("🤖 Assistant ─── %s", m.Timestamp.Format("15:04:05")))
	result.WriteString(header)
	result.WriteString("\n")

	// 内容
	if m.Content != "" {
		content := wrapText(m.Content, bw-4)
		for _, line := range content {
			result.WriteString(s.MsgAI.Render(line))
			result.WriteString("\n")
		}
	}

	// 底部信息（如果完成）
	if m.Done {
		var footerParts []string
		footerParts = append(footerParts, s.TextSuccess.Render("✓ Done"))
		if m.Tokens > 0 {
			footerParts = append(footerParts, s.TextInfo.Render(fmt.Sprintf("%d tokens", m.Tokens)))
		}
		if m.Duration > 0 {
			footerParts = append(footerParts, s.TextWarning.Render(fmt.Sprintf("%.1fs", m.Duration.Seconds())))
		}
		if len(footerParts) > 0 {
			footer := s.MsgAIFooter.Render(strings.Join(footerParts, " · "))
			result.WriteString(footer)
		}
		if strings.TrimSpace(m.Content) != "" {
			label := strings.TrimSpace(m.CopyLabel)
			if label == "" {
				label = "Copy"
			}
			result.WriteString("\n")
			result.WriteString(lipgloss.PlaceHorizontal(bw-4, lipgloss.Right, s.MsgCopyButton.Render(label)))
		}
	}

	bubble := s.MsgAIBorder.Render(result.String())
	return lipgloss.Place(width, lipgloss.Height(bubble), lipgloss.Left, lipgloss.Top, bubble)
}

type AgentBubbleMessage struct {
	Name      string
	Label     string
	IsMain    bool
	PreStyled bool
	Content   string
	Timestamp time.Time
	Tokens    int
	Duration  time.Duration
	Done      bool
	CopyLabel string
}

func (m *AgentBubbleMessage) Type() MessageType { return MsgTypeAgent }

func (m *AgentBubbleMessage) Render(s *styles.Styles, width int) string {
	var result strings.Builder
	bw := bubbleWidth(width)

	dot := s.TextSuccess.Render("●")
	if m.IsMain {
		if m.Done {
			dot = s.TextSuccess.Render("●")
		} else {
			dot = s.TextInfo.Render("●")
		}
	}
	name := s.MsgAgentHeader.Render(m.Name)
	label := ""
	if m.Label != "" {
		label = s.TextMuted.Render(m.Label)
	}
	ts := ""
	if !m.Timestamp.IsZero() {
		ts = s.TextMuted.Render(m.Timestamp.Format("15:04:05"))
	}

	headerParts := []string{dot, name}
	if label != "" {
		headerParts = append(headerParts, label)
	}
	if ts != "" {
		headerParts = append(headerParts, ts)
	}
	result.WriteString(strings.Join(headerParts, " "))
	result.WriteString("\n")

	if m.Content != "" {
		if m.PreStyled {
			lines := splitAndWrapANSI(m.Content, bw-4)
			for i, line := range lines {
				result.WriteString(line)
				if i < len(lines)-1 {
					result.WriteString("\n")
				}
			}
		} else {
			lines := wrapText(m.Content, bw-4)
			for i, line := range lines {
				result.WriteString(s.MsgAgent.Render(line))
				if i < len(lines)-1 {
					result.WriteString("\n")
				}
			}
		}
	}

	if m.Done && (m.Tokens > 0 || m.Duration > 0) {
		var parts []string
		if m.Tokens > 0 {
			parts = append(parts, s.TextMuted.Render(fmt.Sprintf("%d tokens", m.Tokens)))
		}
		if m.Duration > 0 {
			parts = append(parts, s.TextMuted.Render(fmt.Sprintf("%.1fs", m.Duration.Seconds())))
		}
		if len(parts) > 0 {
			result.WriteString("\n")
			result.WriteString(s.TextMuted.Render(strings.Join(parts, " · ")))
		}
	}

	if m.Done && strings.TrimSpace(m.Content) != "" {
		label := strings.TrimSpace(m.CopyLabel)
		if label == "" {
			label = "Copy"
		}
		result.WriteString("\n")
		result.WriteString(lipgloss.PlaceHorizontal(bw-4, lipgloss.Right, s.MsgCopyButton.Render(label)))
	}

	return s.MsgAgentBorder.Render(result.String())
}

func splitAndWrapANSI(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		out = append(out, wrapLine(line, maxWidth)...)
	}
	return out
}

type AgentDispatchMessage struct {
	AgentName string
	Task      string
	Timestamp time.Time
}

func (m *AgentDispatchMessage) Type() MessageType { return MsgTypeAgent }

func (m *AgentDispatchMessage) Render(s *styles.Styles, width int) string {
	var result strings.Builder
	bw := bubbleWidth(width)

	dot := s.TextInfo.Render("●")
	name := s.MsgAgentHeader.Render(m.AgentName)
	ts := ""
	if !m.Timestamp.IsZero() {
		ts = s.TextMuted.Render(m.Timestamp.Format("15:04:05"))
	}

	headerParts := []string{dot, name, s.TextMuted.Render("已分配")}
	if ts != "" {
		headerParts = append(headerParts, ts)
	}
	result.WriteString(strings.Join(headerParts, " "))
	result.WriteString("\n")

	taskText := strings.TrimSpace(m.Task)
	if taskText != "" {
		lines := wrapText(taskText, bw-4)
		for i, line := range lines {
			result.WriteString(s.MsgAgent.Render(line))
			if i < len(lines)-1 {
				result.WriteString("\n")
			}
		}
	}

	return s.MsgAgentBorder.Render(result.String())
}

// ToolCallMessage 工具调用消息
type ToolCallMessage struct {
	Name      string
	Params    map[string]any
	Status    string // "running", "success", "error"
	Result    string
	Duration  time.Duration
	Timestamp time.Time
}

func (m *ToolCallMessage) Type() MessageType { return MsgTypeTool }

func (m *ToolCallMessage) Render(s *styles.Styles, width int) string {
	var result strings.Builder
	bw := bubbleWidth(width)
	contentW := bw - 4
	if contentW < 10 {
		contentW = width - 4
	}
	toolLower := strings.ToLower(strings.TrimSpace(m.Name))

	// 头部
	icon := "🔧"
	statusStr := ""
	border := s.MsgToolBorder
	switch m.Status {
	case "running":
		statusStr = s.TextInfo.Render("⏳ Executing...")
	case "success":
		icon = "✅"
		statusStr = s.MsgToolSuccess.Render(fmt.Sprintf("✓ Success · %s", formatDuration(m.Duration)))
		if s.Theme != nil {
			border = border.BorderForeground(s.Theme.Success)
		}
	case "error":
		icon = "❌"
		statusStr = s.MsgToolError.Render(fmt.Sprintf("✗ Failed · %s", formatDuration(m.Duration)))
		if s.Theme != nil {
			border = border.BorderForeground(s.Theme.Error)
		}
	}

	headerLeft := s.MsgToolHeader.Render(fmt.Sprintf("%s Tool: %s", icon, displayToolName(m.Name)))
	result.WriteString(headerLeft)
	result.WriteString("\n")

	blockStyle := s.ToolCall.MarginLeft(1)
	resultStyle := s.ToolResult.MarginLeft(1)
	if m.Status == "error" {
		resultStyle = s.MsgToolError.MarginLeft(1)
	}
	dividerW := contentW
	if dividerW > 18 {
		dividerW = 18
	}
	if dividerW < 6 {
		dividerW = 6
	}
	divider := s.TextMuted.Render(strings.Repeat("─", dividerW))

	type detailBlock struct {
		label string
		value string
		kind  string
	}
	var blocks []detailBlock
	shown := map[string]bool{}
	getStr := func(keys ...string) string {
		for _, k := range keys {
			if m.Params == nil {
				continue
			}
			if v, ok := m.Params[k]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					shown[k] = true
					return s
				}
			}
		}
		return ""
	}
	getInt := func(key string) (int, bool) {
		if m.Params == nil {
			return 0, false
		}
		v, ok := m.Params[key]
		if !ok {
			return 0, false
		}
		switch n := v.(type) {
		case int:
			shown[key] = true
			return n, true
		case int64:
			shown[key] = true
			return int(n), true
		case float64:
			shown[key] = true
			return int(n), true
		default:
			return 0, false
		}
	}
	getStrList := func(keys ...string) string {
		for _, k := range keys {
			if m.Params == nil {
				continue
			}
			v, ok := m.Params[k]
			if !ok || v == nil {
				continue
			}
			switch vv := v.(type) {
			case []string:
				if len(vv) == 0 {
					continue
				}
				shown[k] = true
				max := 3
				if len(vv) < max {
					max = len(vv)
				}
				out := strings.Join(vv[:max], "\n")
				if len(vv) > max {
					out += fmt.Sprintf("\n...(+%d)", len(vv)-max)
				}
				return out
			case []any:
				if len(vv) == 0 {
					continue
				}
				var ss []string
				for _, it := range vv {
					if s, ok := it.(string); ok && strings.TrimSpace(s) != "" {
						ss = append(ss, s)
					}
					if len(ss) >= 3 {
						break
					}
				}
				if len(ss) == 0 {
					continue
				}
				shown[k] = true
				out := strings.Join(ss, "\n")
				if len(vv) > len(ss) {
					out += fmt.Sprintf("\n...(+%d)", len(vv)-len(ss))
				}
				return out
			}
		}
		return ""
	}

	switch toolLower {
	case "read", "functions.read":
		if p := getStr("file_path", "path", "file"); p != "" {
			blocks = append(blocks, detailBlock{label: "Path", value: p, kind: "path"})
		}
		if off, okOff := getInt("offset"); okOff {
			if lim, okLim := getInt("limit"); okLim {
				blocks = append(blocks, detailBlock{label: "Range", value: fmt.Sprintf("offset=%d limit=%d", off, lim), kind: "meta"})
			} else {
				blocks = append(blocks, detailBlock{label: "Range", value: fmt.Sprintf("offset=%d", off), kind: "meta"})
			}
		} else if lim, okLim := getInt("limit"); okLim {
			blocks = append(blocks, detailBlock{label: "Range", value: fmt.Sprintf("limit=%d", lim), kind: "meta"})
		}
	case "grep", "functions.grep":
		if pat := getStr("pattern"); pat != "" {
			blocks = append(blocks, detailBlock{label: "Pattern", value: pat, kind: "pattern"})
		}
		if p := getStr("path"); p != "" {
			blocks = append(blocks, detailBlock{label: "In", value: p, kind: "path"})
		}
		if g := getStr("glob"); g != "" {
			blocks = append(blocks, detailBlock{label: "Glob", value: g, kind: "meta"})
		}
	case "glob", "functions.glob":
		if pat := getStr("pattern"); pat != "" {
			blocks = append(blocks, detailBlock{label: "Pattern", value: pat, kind: "pattern"})
		}
		if p := getStr("path"); p != "" {
			blocks = append(blocks, detailBlock{label: "In", value: p, kind: "path"})
		}
	case "searchcodebase", "functions.searchcodebase":
		if req := getStr("information_request"); req != "" {
			blocks = append(blocks, detailBlock{label: "Query", value: req, kind: "pattern"})
		}
	case "runcommand", "functions.runcommand":
		if cmd := getStr("command"); cmd != "" {
			cmd = strings.ReplaceAll(cmd, "\r\n", "\n")
			blocks = append(blocks, detailBlock{label: "Command", value: cmd, kind: "command"})
		}
		if cwd := getStr("cwd"); cwd != "" {
			blocks = append(blocks, detailBlock{label: "Cwd", value: cwd, kind: "path"})
		}
		if ct := getStr("command_type"); ct != "" {
			blocks = append(blocks, detailBlock{label: "Type", value: ct, kind: "meta"})
		}
		if b, ok := m.Params["blocking"].(bool); ok {
			shown["blocking"] = true
			blocks = append(blocks, detailBlock{label: "Blocking", value: fmt.Sprintf("%v", b), kind: "meta"})
		}
	case "webfetch", "functions.webfetch":
		if url := getStr("url"); url != "" {
			blocks = append(blocks, detailBlock{label: "URL", value: url, kind: "path"})
		}
	case "websearch", "functions.websearch":
		if q := getStr("query"); q != "" {
			blocks = append(blocks, detailBlock{label: "Query", value: q, kind: "pattern"})
		}
	case "deletefile", "functions.deletefile":
		if fp := getStrList("file_paths", "paths", "files"); fp != "" {
			blocks = append(blocks, detailBlock{label: "Paths", value: fp, kind: "path"})
		}
	default:
		if cmd := getStr("command"); cmd != "" {
			blocks = append(blocks, detailBlock{label: "Command", value: cmd, kind: "command"})
		} else if p := getStr("path", "file_path", "file"); p != "" {
			blocks = append(blocks, detailBlock{label: "Path", value: p, kind: "path"})
		} else if fp := getStrList("file_paths", "paths", "files"); fp != "" {
			blocks = append(blocks, detailBlock{label: "Paths", value: fp, kind: "path"})
		}
	}

	if len(blocks) > 0 {
		var summary string
		summaryFromLabel := ""
		for _, b := range blocks {
			switch b.kind {
			case "command", "path", "pattern":
				summary = strings.TrimSpace(b.value)
				summaryFromLabel = b.label
			default:
				continue
			}
			if summary != "" {
				break
			}
		}
		if summary != "" {
			summaryLine := summary
			if i := strings.IndexByte(summaryLine, '\n'); i >= 0 {
				summaryLine = summaryLine[:i]
			}
			if len(summaryLine) > 64 {
				summaryLine = summaryLine[:64] + "..."
			}
			result.WriteString(s.TextMuted.Render(summaryLine))
			result.WriteString("\n")
		}
		skipBlocks := false
		if toolLower == "fs" && summaryFromLabel == "Path" {
			skipBlocks = true
		}
		if toolLower == "read" || toolLower == "functions.read" {
			skipBlocks = true
		}
		needDivider := len(m.Params) > 0 || strings.TrimSpace(m.Result) != ""
		if needDivider {
			result.WriteString(divider)
			result.WriteString("\n")
		}
		if !skipBlocks {
			for _, b := range blocks {
				result.WriteString(s.TextMuted.Render("• " + b.label + ":"))
				result.WriteString("\n")
				val := strings.TrimSpace(b.value)
				if b.kind == "command" && !strings.Contains(val, "\n") && val != "" {
					val = "$ " + val
				}
				if len(val) > 800 {
					val = val[:800] + "..."
				}
				for _, line := range wrapText(val, contentW-2) {
					result.WriteString(blockStyle.Render(line))
					result.WriteString("\n")
				}
			}
		}
	}

	// 参数
	if len(m.Params) > 0 {
		var keys []string
		for k := range m.Params {
			if shown[k] {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(keys) > 0 {
			result.WriteString(s.TextMuted.Render("Parameters:"))
			result.WriteString("\n")
		}
		for _, k := range keys {
			v := m.Params[k]
			paramLine := fmt.Sprintf("  %s: %v", k, v)
			if len(paramLine) > 240 {
				paramLine = paramLine[:240] + "..."
			}
			wrapped := wrapText(paramLine, contentW)
			for _, line := range wrapped {
				result.WriteString(line)
				result.WriteString("\n")
			}
		}
	}

	// 结果
	if m.Result != "" {
		if len(blocks) > 0 || len(m.Params) > 0 {
			result.WriteString(divider)
			result.WriteString("\n")
		}
		result.WriteString(s.TextMuted.Render("Result:"))
		result.WriteString("\n")
		out := strings.TrimSpace(m.Result)
		out = strings.ReplaceAll(out, "\r\n", "\n")
		out = strings.ReplaceAll(out, "\r", "")
		out = strings.ReplaceAll(out, "\t", "    ")
		limit := 1200
		if toolLower == "bash" {
			limit = 200000
		}
		if len(out) > limit {
			out = out[:limit] + "\n…trimmed"
		}
		for _, line := range wrapText(out, contentW-2) {
			result.WriteString(resultStyle.Render(line))
			result.WriteString("\n")
		}
	}

	// 状态
	if statusStr != "" {
		result.WriteString(statusStr)
	}

	return border.Render(result.String())
}

func displayToolName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if i := strings.LastIndex(raw, "."); i >= 0 && i+1 < len(raw) {
		raw = raw[i+1:]
	}
	if !strings.Contains(raw, "_") && !strings.Contains(raw, "-") {
		if raw != strings.ToLower(raw) {
			return raw
		}
	}
	l := strings.ToLower(raw)
	known := map[string]string{
		"fs":             "FS",
		"mcp":            "MCP",
		"lsp":            "LSP",
		"ui":             "UI",
		"time_now":       "TimeNow",
		"runcommand":     "RunCommand",
		"searchcodebase": "SearchCodebase",
		"websearch":      "WebSearch",
		"webfetch":       "WebFetch",
	}
	if v, ok := known[l]; ok {
		return v
	}
	raw = strings.ReplaceAll(raw, "-", "_")
	parts := strings.Split(raw, "_")
	var b strings.Builder
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		pl := strings.ToLower(p)
		switch pl {
		case "fs", "mcp", "lsp", "ui", "url", "http", "https", "json", "yaml", "xml", "sql":
			b.WriteString(strings.ToUpper(pl))
		default:
			b.WriteString(strings.ToUpper(pl[:1]))
			if len(pl) > 1 {
				b.WriteString(pl[1:])
			}
		}
	}
	if b.Len() == 0 {
		return raw
	}
	return b.String()
}

// AgentMessage 子Agent消息（调度任务 - 蓝色圆点）
type AgentMessage struct {
	Name       string
	Task       string
	Goal       string
	Progress   int // 0-100
	Step       int
	TotalSteps int
	Status     string // "running", "completed", "failed"
	Duration   time.Duration
	Results    []string
}

func (m *AgentMessage) Type() MessageType { return MsgTypeAgent }

func (m *AgentMessage) Render(s *styles.Styles, width int) string {
	var result strings.Builder

	// 头部 - 蓝色圆点表示调度任务
	dot := s.TextInfo.Render("●")
	name := s.MsgAgentHeader.Render(m.Name)
	if m.Status == "completed" {
		dot = s.TextSuccess.Render("●")
	}
	if m.Status == "failed" {
		dot = s.TextError.Render("●")
	}
	result.WriteString(dot + " " + name)
	result.WriteString("\n")

	// 任务信息
	if m.Task != "" {
		result.WriteString(fmt.Sprintf("📋 %s\n", m.Task))
	}
	if m.Goal != "" {
		result.WriteString(fmt.Sprintf("🎯 %s\n", m.Goal))
	}

	// 进度条
	if m.Status == "running" && m.TotalSteps > 0 {
		progressBar := renderProgressBar(m.Progress, width-6, s)
		result.WriteString(progressBar)
		result.WriteString("\n")
		result.WriteString(s.MsgAgentRunning.Render(fmt.Sprintf("⏳ Step %d/%d", m.Step, m.TotalSteps)))
		result.WriteString("\n")
	}

	// 结果
	if len(m.Results) > 0 && m.Status == "completed" {
		result.WriteString(s.MsgAgentDone.Render(fmt.Sprintf("✓ Completed %d items", len(m.Results))))
		result.WriteString("\n")
	}

	// 耗时
	if m.Duration > 0 {
		result.WriteString(s.TextMuted.Render(fmt.Sprintf("⏱ %s", formatDuration(m.Duration))))
	}

	return s.MsgAgentBorder.Render(result.String())
}

// AgentFinalMessage 子Agent最终输出（绿色圆点）
type AgentFinalMessage struct {
	AgentName string
	Content   string
}

func (m *AgentFinalMessage) Type() MessageType { return MsgTypeAgent }

func (m *AgentFinalMessage) Render(s *styles.Styles, width int) string {
	var result strings.Builder

	// 头部 - 绿色圆点表示最终结果
	result.WriteString(s.TextSuccess.Render("●") + " " + s.TextSuccess.Bold(true).Render(m.AgentName))
	result.WriteString("\n")

	// 内容
	if m.Content != "" {
		lines := wrapText(m.Content, width-4)
		for _, line := range lines {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}

	return s.MsgAgentBorder.Render(result.String())
}

// PlanMessage 计划消息
type PlanMessage struct {
	Title       string
	Description string
	Steps       []PlanStep
	Status      string // "pending", "running", "completed"
}

// PlanStep 计划步骤
type PlanStep struct {
	Number      int
	Description string
	Status      string // "pending", "running", "completed", "failed"
}

func (m *PlanMessage) Type() MessageType { return MsgTypePlan }

func (m *PlanMessage) Render(s *styles.Styles, width int) string {
	var result strings.Builder

	// 头部
	header := s.MsgPlanHeader.Render(fmt.Sprintf("📋 Plan: %s", m.Title))
	result.WriteString(header)
	result.WriteString("\n")

	// 描述
	if m.Description != "" {
		result.WriteString(m.Description)
		result.WriteString("\n\n")
	}

	// 步骤
	for _, step := range m.Steps {
		statusIcon := "⏸"
		statusStyle := s.TextMuted
		switch step.Status {
		case "completed":
			statusIcon = "✓"
			statusStyle = s.TextSuccess
		case "running":
			statusIcon = "🔄"
			statusStyle = s.TextInfo
		case "failed":
			statusIcon = "✗"
			statusStyle = s.TextError
		}

		stepLine := fmt.Sprintf("%s %d. %s", statusStyle.Render(statusIcon), step.Number, step.Description)
		result.WriteString(s.MsgPlanStep.Render(stepLine))
		result.WriteString("\n")
	}

	return s.MsgPlanBorder.Render(result.String())
}

// ThinkingMessage 思考过程消息
type ThinkingMessage struct {
	Content  string
	Duration time.Duration
	Expanded bool
	Steps    []ThinkingStep
}

// ThinkingStep 思考步骤
type ThinkingStep struct {
	Description string
	Status      string // "done", "doing", "pending"
}

func (m *ThinkingMessage) Type() MessageType { return MsgTypeThinking }

func (m *ThinkingMessage) Render(s *styles.Styles, width int) string {
	var result strings.Builder

	// 头部
	expandIcon := "[+]"
	if m.Expanded {
		expandIcon = "[-]"
	}
	header := s.MsgThinkingHeader.Render(fmt.Sprintf("💭 Thinking · %.1fs ─── %s", m.Duration.Seconds(), expandIcon))
	result.WriteString(header)

	// 展开时显示内容
	if m.Expanded {
		result.WriteString("\n")

		// 步骤
		for _, step := range m.Steps {
			statusIcon := "○"
			switch step.Status {
			case "done":
				statusIcon = s.TextSuccess.Render("✓")
			case "doing":
				statusIcon = s.TextInfo.Render("●")
			}
			result.WriteString(fmt.Sprintf("  %s %s\n", statusIcon, step.Description))
		}

		// 详细内容
		if m.Content != "" {
			result.WriteString("\n")
			lines := wrapText(m.Content, width-6)
			for _, line := range lines {
				result.WriteString("  " + s.TextMuted.Render(line))
				result.WriteString("\n")
			}
		}
	}

	return s.MsgThinkingBorder.Render(result.String())
}

// SystemMessage 系统消息
type SystemMessage struct {
	Content string
	Level   string // "info", "warning", "error", "success"
}

func (m *SystemMessage) Type() MessageType {
	switch m.Level {
	case "error":
		return MsgTypeError
	case "warning":
		return MsgTypeWarning
	case "info":
		return MsgTypeInfo
	default:
		return MsgTypeSystem
	}
}

func (m *SystemMessage) Render(s *styles.Styles, width int) string {
	icon := "ℹ️"
	style := s.MsgInfo

	switch m.Level {
	case "error":
		icon = "❌"
		style = s.MsgError
	case "warning":
		icon = "⚠️"
		style = s.MsgWarning
	case "success":
		icon = "✅"
		style = s.TextSuccess
	}
	lines := wrapText(fmt.Sprintf("%s %s", icon, m.Content), width)
	var out strings.Builder
	for i, line := range lines {
		out.WriteString(style.Render(line))
		if i < len(lines)-1 {
			out.WriteString("\n")
		}
	}
	return out.String()
}

// 辅助函数

func wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}

	var lines []string
	for _, src := range strings.Split(text, "\n") {
		if src == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, wrapLine(src, maxWidth)...)
	}
	return lines
}

func bubbleWidth(width int) int {
	if width < 24 {
		return width
	}
	if width > 96 {
		return 96
	}
	return width
}

func wrapLine(s string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{s}
	}
	if xansi.StringWidth(s) <= maxWidth {
		return []string{s}
	}

	type tok struct {
		s     string
		width int
		space bool
	}
	var toks []tok
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			j := i + 1
			if j < len(s) && s[j] == '[' {
				j++
				for j < len(s) {
					c := s[j]
					if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
						j++
						break
					}
					j++
				}
				toks = append(toks, tok{s: s[i:j], width: 0})
				i = j
				continue
			}
			toks = append(toks, tok{s: s[i : i+1], width: 0})
			i++
			continue
		}
		r, size := rune(s[i]), 1
		if r >= 0x80 {
			r, size = utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size == 1 {
				size = 1
			}
		}
		w := runewidth.RuneWidth(r)
		space := r == ' ' || r == '\t'
		if r == '\t' {
			w = 4
		}
		toks = append(toks, tok{s: s[i : i+size], width: w, space: space})
		i += size
	}

	var out []string
	curW := 0
	lastSpace := -1
	widthAtSpace := 0
	start := 0

	emit := func(end int) {
		if end < start {
			end = start
		}
		partToks := toks[start:end]
		for len(partToks) > 0 && partToks[len(partToks)-1].space {
			partToks = partToks[:len(partToks)-1]
		}
		var b strings.Builder
		for _, t := range partToks {
			b.WriteString(t.s)
		}
		out = append(out, b.String())
		start = end
		for start < len(toks) && toks[start].space {
			start++
		}
		curW = 0
		lastSpace = -1
		widthAtSpace = 0
		for i := start; i < len(toks); i++ {
			if toks[i].space {
				lastSpace = i
				widthAtSpace = curW
			}
			curW += toks[i].width
			if curW >= maxWidth {
				break
			}
		}
	}

	curW = 0
	for i := 0; i < len(toks); i++ {
		curW += toks[i].width
		if toks[i].space {
			lastSpace = i
			widthAtSpace = curW - toks[i].width
		}
		if curW > maxWidth && i > start {
			if lastSpace >= start && widthAtSpace > 0 {
				emit(lastSpace)
				i = start - 1
				curW = 0
				continue
			}
			emit(i)
			i = start - 1
			curW = 0
			continue
		}
	}
	if start < len(toks) {
		var b strings.Builder
		for _, t := range toks[start:] {
			b.WriteString(t.s)
		}
		out = append(out, strings.TrimRight(b.String(), " "))
	}
	return out
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func renderProgressBar(progress int, width int, s *styles.Styles) string {
	if width < 3 {
		return ""
	}

	filled := (progress * (width - 2)) / 100
	if filled < 0 {
		filled = 0
	}
	if filled > width-2 {
		filled = width - 2
	}

	empty := (width - 2) - filled

	bar := s.TextSuccess.Render(strings.Repeat("█", filled)) +
		s.TextMuted.Render(strings.Repeat("░", empty))

	return fmt.Sprintf("[%s]", bar)
}
