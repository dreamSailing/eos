package session

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"strings"
)

const (
	// maxToolOutputLen is the threshold for trimming tool outputs
	maxToolOutputLen = 2000
	// headKeepLen is how many chars to keep from the start
	headKeepLen = 800
	// tailKeepLen is how many chars to keep from the end
	tailKeepLen = 400
)

// SnipToolOutputs trims excessively long tool outputs in recent messages.
// This preserves conversation structure while reducing token usage.
func (c *ContextManager) SnipToolOutputs() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snipToolOutputsLocked()
}

// snipToolOutputsLocked trims long tool outputs (caller must hold lock)
func (c *ContextManager) snipToolOutputsLocked() int {
	snipped := 0

	for i := range c.recent {
		m := &c.recent[i]
		if m.Role != "tool" && m.Role != "assistant" {
			continue
		}

		if len(m.Content) <= maxToolOutputLen {
			continue
		}

		trimmed := snipString(m.Content, headKeepLen, tailKeepLen)
		if trimmed != m.Content {
			m.Content = trimmed
			snipped++
		}
	}

	// Also trim in currentFull
	for i := range c.currentFull {
		m := &c.currentFull[i]
		if len(m.Content) <= maxToolOutputLen {
			continue
		}
		m.Content = snipString(m.Content, headKeepLen, tailKeepLen)
		snipped++
	}

	return snipped
}

// SnipAllToolOutputs aggressively trims all tool observation strings
func (c *ContextManager) SnipAllToolOutputs() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	snipped := 0
	for i, s := range c.toolObs {
		if len(s) > maxToolOutputLen {
			c.toolObs[i] = snipString(s, headKeepLen, tailKeepLen)
			snipped++
		}
	}
	for i, s := range c.tools {
		if len(s) > maxToolOutputLen {
			c.tools[i] = snipString(s, headKeepLen, tailKeepLen)
			snipped++
		}
	}

	return snipped
}

// CompactWithSnip first tries standard compression, then trims tool outputs
func (c *ContextManager) CompactWithSnip() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.compactLocked()
	c.snipToolOutputsLocked()
}

// HasOverlongOutputs checks if any messages exceed the trimming threshold
func (c *ContextManager) HasOverlongOutputs() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, m := range c.recent {
		if len(m.Content) > maxToolOutputLen {
			return true
		}
	}
	for _, m := range c.currentFull {
		if len(m.Content) > maxToolOutputLen {
			return true
		}
	}
	return false
}

// EstimateSnippableTokens estimates how many tokens could be saved by trimming
func (c *ContextManager) EstimateSnippableTokens() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	saved := 0
	for _, m := range c.recent {
		if len(m.Content) > maxToolOutputLen {
			original := c.estimateTextTokensLocked(m.Content)
			trimmed := snipString(m.Content, headKeepLen, tailKeepLen)
			saved += original - c.estimateTextTokensLocked(trimmed)
		}
	}
	return saved
}

func snipString(s string, headLen, tailLen int) string {
	if len(s) <= headLen+tailLen+100 {
		return s
	}

	var marker strings.Builder
	marker.WriteString("\n...[output trimmed, original: ")
	marker.WriteString(formatSnipInt(len(s)))
	marker.WriteString(" chars]...\n")

	head := s[:headLen]
	tail := s[len(s)-tailLen:]

	var sb strings.Builder
	sb.Grow(headLen + tailLen + 64)
	sb.WriteString(head)
	sb.WriteString(marker.String())
	sb.WriteString(tail)
	return sb.String()
}

// formatSnipInt formats an int without importing strconv
func formatSnipInt(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
