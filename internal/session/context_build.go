package session

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"strings"
	"github.com/dreamSailing/eos/internal/ai"
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
		if strings.TrimSpace(m.Content) != "" && !c.shouldSnip(m) {
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
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		if c.shouldSnip(m) {
			// Replace snipped messages with a placeholder
			reason := c.snipReason(m)
			msgs = append(msgs, ai.Message{Role: m.Role, Content: "[snipped" + reason + "]"})
			continue
		}
		msgs = append(msgs, m)
	}
	if len(c.currentFull) > 0 {
		for _, m := range c.currentFull {
			if strings.TrimSpace(m.Content) != "" && !c.shouldSnip(m) {
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

// SetSnipChecker sets the callback for checking if messages should be snipped
func (c *ContextManager) SetSnipChecker(check func(content string) bool, reason func(content string) string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snipCheck = check
	c.snipReasonFor = reason
}

// shouldSnip checks if a message should be snipped from context
func (c *ContextManager) shouldSnip(m ai.Message) bool {
	if c.snipCheck == nil {
		return false
	}
	return c.snipCheck(m.Content)
}

// snipReason returns the reason for snipping a message
func (c *ContextManager) snipReason(m ai.Message) string {
	if c.snipReasonFor == nil {
		return ""
	}
	reason := c.snipReasonFor(m.Content)
	if reason != "" {
		return ": " + reason
	}
	return ""
}
