package session

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"strings"

	"github.com/dreamSailing/eos/internal/ai"
)

const (
	// aiCompressKeepRounds is the number of recent rounds to keep intact during AI compression
	aiCompressKeepRounds = 4
	// aiSummaryPrefix marks AI-generated summaries
	aiSummaryPrefix = "[AI Summary] "
)

// AICompress performs AI-driven intelligent compression.
// It keeps recent N rounds intact and replaces older messages with an AI-generated summary.
func (c *ContextManager) AICompress(summarizeFunc func(string) (string, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.recent) <= aiCompressKeepRounds {
		return
	}

	head := c.recent[:len(c.recent)-aiCompressKeepRounds]

	// Build text from old messages for AI summarization
	var sb strings.Builder
	sb.WriteString("Please summarize the following conversation history concisely, preserving key decisions, file changes, and important context:\n\n")
	for _, m := range head {
		role := "Assistant"
		if m.Role == "user" {
			role = "User"
		}
		content := m.Content
		if len(content) > 2000 {
			content = content[:1000] + "\n...[truncated]...\n" + content[len(content)-500:]
		}
		sb.WriteString(role + ": " + content + "\n\n")
	}

	if summarizeFunc != nil {
		summary, err := summarizeFunc(sb.String())
		if err == nil && strings.TrimSpace(summary) != "" {
			// Replace old messages with AI summary
			summaryMsg := ai.Message{
				Role:    "system",
				Content: aiSummaryPrefix + summary,
			}
			c.recent = append([]ai.Message{summaryMsg}, c.recent[len(c.recent)-aiCompressKeepRounds:]...)

			// Update compression stats
			c.compressionStats.TotalCompressions++
			c.compressionStats.Strategy = CompressionBalanced
			c.compressionStats.LastCompressedAt = c.estimateTimestamp()
			return
		}
	}

	// Fallback to standard compact if AI summarization fails
	c.compactLocked()
}
