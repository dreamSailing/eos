package session

import (
	"strconv"
	"strings"
	"time"
	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/tools"
)

// Compact 压缩上下文
func (c *ContextManager) Compact() {
	c.CompactWithTrigger("manual", "")
}

func (c *ContextManager) CompactWithTrigger(trigger string, customInstructions string) {
	c.mu.RLock()
	cb := c.onPreCompact
	c.mu.RUnlock()
	if cb != nil {
		go cb(strings.TrimSpace(trigger), strings.TrimSpace(customInstructions))
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.recent) <= c.recentRounds {
		return
	}
	head := c.recent[:len(c.recent)-c.recentRounds]
	var sb strings.Builder
	for _, m := range head {
		sb.WriteString(formatCompactLine(m))
		sb.WriteString("\n")
	}
	c.recent = append([]ai.Message{{Role: "system", Content: "Conversation summary:\n" + sb.String()}}, c.recent[len(c.recent)-c.recentRounds:]...)
}

// compactLocked 内部压缩方法（调用前需持有锁）
func (c *ContextManager) compactLocked() {
	if len(c.recent) <= c.recentRounds {
		return
	}

	// 保存压缩前快照
	originalChars := c.estimateTotalCharsLocked()
	originalTokens := c.estimateTotalTokensLocked()
	snapshot := ContextSnapshot{
		Timestamp: c.estimateTimestamp(),
		Messages:  c.copyMessagesLocked(c.recent),
	}

	head := c.recent[:len(c.recent)-c.recentRounds]
	var sb strings.Builder
	for _, m := range head {
		sb.WriteString(formatCompactLine(m))
		sb.WriteString("\n")
	}
	c.recent = append([]ai.Message{{Role: "system", Content: "Conversation summary:\n" + sb.String()}}, c.recent[len(c.recent)-c.recentRounds:]...)

	// 更新压缩统计
	compressedChars := c.estimateTotalCharsLocked()
	compressedTokens := c.estimateTotalTokensLocked()
	c.compressionStats.TotalCompressions++
	c.compressionStats.LastCompressedAt = snapshot.Timestamp
	c.compressionStats.Strategy = c.compressionStrategy
	c.compressionStats.OriginalChars = originalChars
	c.compressionStats.CompressedChars = compressedChars
	c.compressionStats.OriginalTokens = originalTokens
	c.compressionStats.CompressedTokens = compressedTokens
	if originalTokens > 0 {
		c.compressionStats.SavedRatio = float64(originalTokens-compressedTokens) / float64(originalTokens)
	} else if originalChars > 0 {
		c.compressionStats.SavedRatio = float64(originalChars-compressedChars) / float64(originalChars)
	}

	// 保存快照
	snapshot.Stats = c.compressionStats
	c.addSnapshotLocked(snapshot)

	// Notify post-compact callback
	if c.onPostCompact != nil {
		savedTokens := originalTokens - compressedTokens
		go c.onPostCompact("compact", originalTokens, savedTokens)
	}
}

// AutoCompactIfNeeded 检查当前上下文大小并在超过阈值时自动压缩
func (c *ContextManager) AutoCompactIfNeeded() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.autoCompactIfNeededLocked()
}

// EstimateCurrentTokens returns the estimated token count for the current context
func (c *ContextManager) EstimateCurrentTokens() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.estimateTotalTokensLocked()
}

// autoCompactIfNeededLocked 内部自动压缩方法（调用前需持有锁）
func (c *ContextManager) autoCompactIfNeededLocked() {
	if !c.autoCompressEnabled {
		return
	}
	totalTokens := c.estimateTotalTokensLocked()
	limit := int(float64(c.maxPromptTokens) * c.compressThreshold)
	if totalTokens > limit {
		if c.onPreCompact != nil {
			go c.onPreCompact("auto", "")
		}
		c.compactLocked()
	}
}

// estimateTotalCharsLocked 估算当前上下文总字符数（调用前需持有锁）
func (c *ContextManager) estimateTotalCharsLocked() int {
	total := 0
	for _, s := range c.ephem {
		total += len(s)
	}
	for _, m := range c.pinned {
		total += len(m.Content)
	}
	for _, m := range c.recent {
		total += len(m.Content)
	}
	for _, s := range c.tools {
		total += len(s)
	}
	for _, s := range c.toolObs {
		total += len(s)
	}
	for _, m := range c.currentFull {
		total += len(m.Content)
	}
	return total
}

func (c *ContextManager) estimateTotalTokensLocked() int {
	total := 0
	for _, s := range c.ephem {
		total += c.estimateTextTokensLocked(s)
	}
	for _, m := range c.pinned {
		total += c.estimateTextTokensLocked(m.Content)
		if len(m.ImagePaths) > 0 {
			total += len(m.ImagePaths) * 256
		}
	}
	for _, m := range c.recent {
		total += c.estimateTextTokensLocked(m.Content)
		if len(m.ImagePaths) > 0 {
			total += len(m.ImagePaths) * 256
		}
	}
	for _, s := range c.tools {
		total += c.estimateTextTokensLocked(s)
	}
	for _, s := range c.toolObs {
		total += c.estimateTextTokensLocked(s)
	}
	for _, m := range c.currentFull {
		total += c.estimateTextTokensLocked(m.Content)
		if len(m.ImagePaths) > 0 {
			total += len(m.ImagePaths) * 256
		}
	}
	return total
}

// AggressiveCompact 激进压缩到目标 Token 数
func (c *ContextManager) AggressiveCompact(targetTokens int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.aggressiveCompactLocked(targetTokens)
}

// aggressiveCompactLocked 内部激进压缩方法（调用前需持有锁）
func (c *ContextManager) aggressiveCompactLocked(targetTokens int) {
	for i := 0; i < 6; i++ {
		msgs := c.buildPreviewLocked()
		if c.estimateMessagesTokensLocked(msgs) <= targetTokens {
			break
		}
		// step 1: compact recent
		c.compactLocked()
		msgs = c.buildPreviewLocked()
		if c.estimateMessagesTokensLocked(msgs) <= targetTokens {
			break
		}
		// step 2: demote full
		c.demoteFullToSummaryLocked()
		msgs = c.buildPreviewLocked()
		if c.estimateMessagesTokensLocked(msgs) <= targetTokens {
			break
		}
		// step 3: reduce tool summaries
		if len(c.tools) > c.toolLimit {
			keep := c.toolLimit
			if keep > 8 {
				keep = 8
			}
			if keep > 5 {
				keep = 5
			}
			if keep < 3 {
				keep = 3
			}
			if keep < len(c.tools) {
				c.tools = c.tools[len(c.tools)-keep:]
			}
		}
		msgs = c.buildPreviewLocked()
		if c.estimateMessagesTokensLocked(msgs) <= targetTokens {
			break
		}
		// step 4: lower recentRounds
		if c.recentRounds > 1 {
			if c.recentRounds > 4 {
				c.recentRounds = 4
			} else if c.recentRounds > 2 {
				c.recentRounds = 2
			} else {
				c.recentRounds = 1
			}
		}
		c.compactLocked()
		msgs = c.buildPreviewLocked()
		if c.estimateMessagesTokensLocked(msgs) <= targetTokens {
			break
		}
		// step 5: truncate very old recent
		if len(c.recent) > c.recentRounds {
			head := c.recent[:len(c.recent)-c.recentRounds]
			var sb strings.Builder
			for _, m := range head {
				if m.Role == "user" {
					sb.WriteString("[user] ")
				} else {
					sb.WriteString("[assistant] ")
				}
				s := m.Content
				if len(s) > 150 {
					s = s[:150] + "…"
				}
				sb.WriteString(s)
				sb.WriteString("\n")
			}
			c.recent = append([]ai.Message{{Role: "system", Content: "Conversation summary (aggressive):\n" + sb.String()}}, c.recent[len(c.recent)-c.recentRounds:]...)
		}
	}
}

// DemoteFullToSummary 将当前完整工具内容转换为摘要
func (c *ContextManager) DemoteFullToSummary() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.demoteFullToSummaryLocked()
}

// demoteFullToSummaryLocked 内部降级方法（调用前需持有锁）
func (c *ContextManager) demoteFullToSummaryLocked() {
	if len(c.currentFull) == 0 {
		return
	}
	for _, m := range c.currentFull {
		s := m.Content
		if tools.ShouldCompress(s) {
			ct := guessToolCompressType(s)
			compressed, ok := tools.CompressToolOutput(s, ct)
			if ok {
				s = tools.FormatCompressedOutput(s, compressed, ct)
			}
		}
		// 增大保留量，并采用头尾保留策略
		if len(s) > 6000 {
			s = s[:4000] + "\n\u2026[省略 " + strconv.Itoa(len(s)-6000) + " 字符]…\n" + s[len(s)-2000:]
		}
		c.tools = append(c.tools, s)
	}
	c.currentFull = nil
}

// CurrentFull 获取当前完整内容
func (c *ContextManager) CurrentFull() []ai.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentFull
}

// ClearCurrentFull 清除当前完整内容
func (c *ContextManager) ClearCurrentFull() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.currentFull = nil
}

// copyMessagesLocked 复制消息列表（调用前需持有锁）
func (c *ContextManager) copyMessagesLocked(msgs []ai.Message) []ai.Message {
	cpy := make([]ai.Message, len(msgs))
	for i, m := range msgs {
		cpy[i] = ai.Message{
			Role:       m.Role,
			Content:    m.Content,
			ImagePaths: append([]string{}, m.ImagePaths...),
		}
	}
	return cpy
}

// estimateTimestamp 估算当前时间戳（秒级）
func (c *ContextManager) estimateTimestamp() int64 {
	return time.Now().Unix()
}

func formatCompactLine(m ai.Message) string {
	role := "[assistant] "
	if m.Role == "user" {
		role = "[user] "
	}
	s := strings.TrimSpace(m.Content)
	if s == "" {
		return role + "(empty)"
	}

	// 检测工具调用和代码内容
	if strings.Contains(s, "```") || strings.Contains(s, "*** Begin Patch") || strings.Contains(s, "diff --git") {
		role = role + "[code] "
	}

	// 提取工具调用名称（如 [read], [search], [edit] 等）
	toolNames := extractToolNames(s)
	if len(toolNames) > 0 {
		role = role + "[tools:" + strings.Join(toolNames, ",") + "] "
	}

	// 保留错误/失败信息标记
	if strings.Contains(s, "❌") || strings.Contains(s, "失败") || strings.Contains(s, "error") || strings.Contains(s, "not found") {
		role = role + "[has-error] "
	}

	// 保留更多内容：头部 + 尾部，中间省略
	if len(s) > 800 {
		head := s[:500]
		tail := s[len(s)-200:]
		lines := 1 + strings.Count(s, "\n")
		return role + head + "\n\u2026[省略 " + strconv.Itoa(lines) + " 行]…\n" + tail
	}

	if idx := strings.IndexByte(s, '\n'); idx > 0 {
		first := strings.TrimSpace(s[:idx])
		if len(first) > 400 {
			first = first[:400] + "\u2026"
		}
		lines := 1 + strings.Count(s, "\n")
		return role + first + " \u2026(" + strconv.Itoa(lines) + " lines)"
	}
	if len(s) > 600 {
		s = s[:600] + "\u2026"
	}
	return role + s
}

// extractToolNames 从消息内容中提取工具调用名称
func extractToolNames(content string) []string {
	knownTools := []string{"read", "search", "edit", "fs", "bash", "bash_session", "git_", "ProjectStructure", "plan_steps", "todo_", "skill", "tool_search"}
	var found []string
	seen := make(map[string]bool)
	for _, t := range knownTools {
		if strings.Contains(content, "["+t+"]") || strings.Contains(content, "\""+t+"\"") || strings.Contains(content, "`"+t+"`") {
			if !seen[t] {
				found = append(found, t)
				seen[t] = true
			}
		}
	}
	return found
}

// addSnapshotLocked 添加压缩快照（调用前需持有锁）
func (c *ContextManager) addSnapshotLocked(snapshot ContextSnapshot) {
	c.snapshots = append(c.snapshots, snapshot)
	if len(c.snapshots) > c.maxSnapshots {
		c.snapshots = c.snapshots[len(c.snapshots)-c.maxSnapshots:]
	}
}
