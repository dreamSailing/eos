package session

import (
	"strings"
	"github.com/dreamSailing/vb-coding/internal/ai"
)

// Clear 清除上下文
func (c *ContextManager) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recent = nil
	c.tools = nil
	c.toolObs = nil
	c.ephem = nil
	c.lastPlan = ""
}

// SetLastPlan 设置最后计划
func (c *ContextManager) SetLastPlan(plan string) {
	s := strings.TrimSpace(plan)
	if s == "" {
		return
	}
	if len(s) > 12000 {
		s = s[:12000] + "\n…trimmed"
	}
	c.mu.Lock()
	c.lastPlan = s
	cb := c.onPlanUpdate
	c.mu.Unlock()
	if cb != nil {
		cb(s)
	}
}

// LastPlan 获取最后计划
func (c *ContextManager) LastPlan() string {
	c.mu.RLock()
	v := c.lastPlan
	c.mu.RUnlock()
	return v
}

// AddTokenRecord 添加 Token 记录（预留接口）
func (c *ContextManager) AddTokenRecord(p, c2, t int) {
	// 预留接口，如需记录历史 token 消耗可在此实现
}

// ExportTools 导出工具观察/摘要供外部渲染
func (c *ContextManager) ExportTools() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.toolObs) == 0 {
		return nil
	}
	out := make([]string, len(c.toolObs))
	copy(out, c.toolObs)
	return out
}

// GetSnapshots 获取压缩历史快照
func (c *ContextManager) GetSnapshots() []ContextSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]ContextSnapshot, len(c.snapshots))
	copy(out, c.snapshots)
	return out
}

// ClearSnapshots 清除压缩历史快照
func (c *ContextManager) ClearSnapshots() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshots = nil
}

// GetCurrentUsage 获取当前上下文使用情况
func (c *ContextManager) GetCurrentUsage() (chars, tokens int, usageRatio float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	chars = c.estimateTotalCharsLocked()
	tokens = c.estimateTotalTokensLocked()
	if c.maxPromptTokens > 0 {
		usageRatio = float64(tokens) / float64(c.maxPromptTokens)
	}
	return
}

// GetConversationUsage 获取对话上下文使用情况（不包含 pinned 系统消息）
func (c *ContextManager) GetConversationUsage() (chars, tokens int, usageRatio float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	msgs := make([]ai.Message, 0, len(c.recent)+len(c.tools)+len(c.toolObs)+len(c.ephem)+len(c.currentFull))
	for _, e := range c.ephem {
		if strings.TrimSpace(e) != "" {
			msgs = append(msgs, ai.Message{Role: "system", Content: e})
		}
	}
	for _, m := range c.recent {
		if strings.TrimSpace(m.Content) != "" {
			msgs = append(msgs, m)
			chars += len(m.Content)
		}
	}
	if len(c.currentFull) > 0 {
		for _, m := range c.currentFull {
			if strings.TrimSpace(m.Content) != "" {
				msgs = append(msgs, m)
				chars += len(m.Content)
			}
		}
	}
	for _, t := range c.toolObs {
		if strings.TrimSpace(t) != "" {
			msgs = append(msgs, ai.Message{Role: "system", Content: t})
			chars += len(t)
		}
	}
	for _, t := range c.tools {
		if strings.TrimSpace(t) != "" {
			msgs = append(msgs, ai.Message{Role: "system", Content: t})
			chars += len(t)
		}
	}

	tokens = c.estimateMessagesTokensLocked(msgs)
	if c.maxPromptTokens > 0 {
		usageRatio = float64(tokens) / float64(c.maxPromptTokens)
	}
	return
}

// GetCompressionThreshold 获取压缩阈值（token 数）
func (c *ContextManager) GetCompressionThreshold() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.maxPromptTokens * 95 / 100
}

func (c *ContextManager) SetConversationSummary(summary string) {
	s := strings.TrimSpace(summary)
	if s == "" {
		return
	}
	if len(s) > 4000 {
		s = s[:4000] + "\n…trimmed"
	}

	const prefix = "CONVERSATION_SUMMARY_AI:\n"
	content := prefix + s

	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.pinned {
		if c.pinned[i].Role == "system" && strings.HasPrefix(c.pinned[i].Content, prefix) {
			c.pinned[i].Content = content
			return
		}
	}
	c.pinned = append([]ai.Message{{Role: "system", Content: content}}, c.pinned...)
}

func (c *ContextManager) SetPinnedDoc(id string, body string, limit int) {
	prefix := "DOC:" + strings.TrimSpace(id) + ":\n"
	s := strings.TrimSpace(body)
	if s == "" {
		return
	}
	if limit > 0 && len(s) > limit {
		s = s[:limit] + "\n…trimmed"
	}
	content := prefix + s

	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.pinned {
		if c.pinned[i].Role == "system" && strings.HasPrefix(c.pinned[i].Content, prefix) {
			c.pinned[i].Content = content
			return
		}
	}
	c.pinned = append([]ai.Message{{Role: "system", Content: content}}, c.pinned...)
}

func (c *ContextManager) DrainAndClearToolContext() (toolObs []string, toolSummaries []string, full []ai.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.toolObs) > 0 {
		toolObs = make([]string, len(c.toolObs))
		copy(toolObs, c.toolObs)
	}
	if len(c.tools) > 0 {
		toolSummaries = make([]string, len(c.tools))
		copy(toolSummaries, c.tools)
	}
	if len(c.currentFull) > 0 {
		full = make([]ai.Message, len(c.currentFull))
		copy(full, c.currentFull)
	}

	c.toolObs = nil
	c.tools = nil
	c.currentFull = nil
	return
}

func (c *ContextManager) AppendTaskSummary(entry string) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return
	}
	if len(entry) > 1200 {
		entry = entry[:1200] + "\n…trimmed"
	}

	const prefix = "TASK_SUMMARY_HISTORY:\n"
	c.mu.Lock()
	defer c.mu.Unlock()

	var blocks []string
	for i := range c.pinned {
		m := c.pinned[i]
		if m.Role != "system" || !strings.HasPrefix(m.Content, prefix) {
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(m.Content, prefix))
		if body != "" {
			blocks = strings.Split(body, "\n\n")
		}
		blocks = append([]string{entry}, blocks...)
		if len(blocks) > 20 {
			blocks = blocks[:20]
		}
		c.pinned[i].Content = prefix + strings.Join(blocks, "\n\n")
		return
	}

	blocks = []string{entry}
	c.pinned = append([]ai.Message{{Role: "system", Content: prefix + strings.Join(blocks, "\n\n")}}, c.pinned...)
}
