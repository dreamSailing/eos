package session

import (
	"log/slog"
	"strings"

	"github.com/dreamSailing/eos/internal/pkg/utils"
)

const (
	// microCompactThreshold is the size threshold for large tool results
	microCompactThreshold = 4000
	// microCompactHeadLines is how many lines to keep from the start
	microCompactHeadLines = 20
	// microCompactTailLines is how many lines to keep from the end
	microCompactTailLines = 10
)

// MicroCompactResult holds the result of a micro-compact pass
type MicroCompactResult struct {
	CompressedCount int
	SavedBytes      int
}

// MicroCompact compresses large tool results in current messages without
// running a full compaction. It only targets "compressible" content —
// large tool outputs — replacing them with head + tail previews.
func (c *ContextManager) MicroCompact() MicroCompactResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.microCompactLocked()
}

func (c *ContextManager) microCompactLocked() MicroCompactResult {
	var result MicroCompactResult

	compress := func(content string) string {
		if len(content) <= microCompactThreshold {
			return content
		}

		lines := strings.Split(content, "\n")
		if len(lines) <= microCompactHeadLines+microCompactTailLines+5 {
			return content
		}

		head := lines[:microCompactHeadLines]
		tail := lines[len(lines)-microCompactTailLines:]

		var sb strings.Builder
		for _, l := range head {
			sb.WriteString(l)
			sb.WriteByte('\n')
		}
		sb.WriteString("...[truncated, ")
		sb.WriteString(formatSnipInt(len(lines)-microCompactHeadLines-microCompactTailLines))
		sb.WriteString(" lines omitted]...\n")
		for _, l := range tail {
			sb.WriteString(l)
			sb.WriteByte('\n')
		}

		newContent := sb.String()
		saved := len(content) - len(newContent)
		if saved > 0 {
			result.CompressedCount++
			result.SavedBytes += saved
		}
		return newContent
	}

	for i := range c.recent {
		m := &c.recent[i]
		if m.Role != "tool" && m.Role != "assistant" {
			continue
		}
		m.Content = compress(m.Content)
	}

	for i := range c.currentFull {
		m := &c.currentFull[i]
		m.Content = compress(m.Content)
	}

	if result.CompressedCount > 0 {
		slog.Debug("session.micro_compact.completed",
			"component", utils.ComponentSystem,
			"compressed_count", result.CompressedCount,
			"saved_bytes", result.SavedBytes,
		)
	}

	return result
}

// ShouldMicroCompact checks if micro-compact would be beneficial
func (c *ContextManager) ShouldMicroCompact() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, m := range c.recent {
		if len(m.Content) > microCompactThreshold {
			return true
		}
	}
	for _, m := range c.currentFull {
		if len(m.Content) > microCompactThreshold {
			return true
		}
	}
	return false
}
