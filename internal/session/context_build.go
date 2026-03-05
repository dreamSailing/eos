package session

import (
	"strings"
	"github.com/dreamSailing/vb-coding/internal/ai"
)

// Build 构建上下文消息列表
func (c *ContextManager) Build() []ai.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buildLocked()
}

// buildLocked 内部构建方法（调用前需持有锁）
func (c *ContextManager) buildLocked() []ai.Message {
	msgs := make([]ai.Message, 0, len(c.pinned)+len(c.recent)+len(c.tools)+len(c.ephem))
	for _, m := range c.pinned {
		if strings.TrimSpace(m.Content) != "" {
			msgs = append(msgs, m)
		}
	}
	if len(c.ephem) > 0 {
		for _, e := range c.ephem {
			if strings.TrimSpace(e) != "" {
				msgs = append(msgs, ai.Message{Role: "system", Content: e})
			}
		}
		c.ephem = nil
	}
	for _, m := range c.recent {
		if strings.TrimSpace(m.Content) != "" {
			msgs = append(msgs, m)
		}
	}
	if len(c.currentFull) > 0 {
		for _, m := range c.currentFull {
			if strings.TrimSpace(m.Content) != "" {
				msgs = append(msgs, m)
			}
		}
	}
	for _, t := range c.toolObs {
		if strings.TrimSpace(t) != "" {
			msgs = append(msgs, ai.Message{Role: "system", Content: t})
		}
	}
	for _, t := range c.tools {
		if strings.TrimSpace(t) != "" {
			msgs = append(msgs, ai.Message{Role: "system", Content: t})
		}
	}
	if c.estimateMessagesTokensLocked(msgs) > c.maxPromptTokens {
		c.aggressiveCompactLocked(c.maxPromptTokens)
		msgs = c.buildPreviewLocked()
		if c.estimateMessagesTokensLocked(msgs) > c.maxPromptTokens {
			msgs = NewSmartTrimmer(c.maxPromptTokens).
				WithStrategy(TrimStrategySmart).
				WithModel(c.modelName, c.tokenCache).
				Trim(msgs)
		}
	}
	return msgs
}

// BuildPreview 构建预览消息列表
func (c *ContextManager) BuildPreview() []ai.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	msgs := make([]ai.Message, 0, len(c.pinned)+len(c.recent)+len(c.tools)+len(c.ephem)+len(c.currentFull))
	msgs = append(msgs, c.pinned...)
	for _, e := range c.ephem {
		msgs = append(msgs, ai.Message{Role: "system", Content: e})
	}
	msgs = append(msgs, c.recent...)
	if len(c.currentFull) > 0 {
		msgs = append(msgs, c.currentFull...)
	}
	for _, t := range c.toolObs {
		msgs = append(msgs, ai.Message{Role: "system", Content: t})
	}
	for _, t := range c.tools {
		msgs = append(msgs, ai.Message{Role: "system", Content: t})
	}
	return msgs
}

// buildPreviewLocked 内部预览构建方法（调用前需持有锁）
func (c *ContextManager) buildPreviewLocked() []ai.Message {
	msgs := make([]ai.Message, 0, len(c.pinned)+len(c.recent)+len(c.tools)+len(c.ephem)+len(c.currentFull))
	msgs = append(msgs, c.pinned...)
	for _, e := range c.ephem {
		msgs = append(msgs, ai.Message{Role: "system", Content: e})
	}
	msgs = append(msgs, c.recent...)
	if len(c.currentFull) > 0 {
		msgs = append(msgs, c.currentFull...)
	}
	for _, t := range c.toolObs {
		msgs = append(msgs, ai.Message{Role: "system", Content: t})
	}
	for _, t := range c.tools {
		msgs = append(msgs, ai.Message{Role: "system", Content: t})
	}
	return msgs
}

// EstimateTokens 估算 Token 数量
func (c *ContextManager) EstimateTokens(lastAssistantText string) (input, reply, total int) {
	msgs := c.BuildPreview()
	c.mu.RLock()
	model := c.modelName
	cache := c.tokenCache
	c.mu.RUnlock()

	for _, m := range msgs {
		input += estimateTextTokens(model, cache, m.Content)
		if len(m.ImagePaths) > 0 {
			input += len(m.ImagePaths) * 256
		}
	}
	reply = estimateTextTokens(model, cache, lastAssistantText)
	total = input + reply
	return
}

// EstimateMessageTokens 估算消息 Token 数量
func (c *ContextManager) EstimateMessageTokens(msgs []ai.Message) int {
	c.mu.RLock()
	model := c.modelName
	cache := c.tokenCache
	c.mu.RUnlock()

	total := 0
	for _, m := range msgs {
		total += estimateTextTokens(model, cache, m.Content)
		if len(m.ImagePaths) > 0 {
			total += len(m.ImagePaths) * 256
		}
	}
	return total
}
