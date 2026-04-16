package tools

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
)

// snippedMessages tracks message content hashes that should be snipped (trimmed from context)
// Using content hash as key to match the SetSnipChecker(callback by content) signature
var (
	snippedMessages   map[string]string // contentHash -> reason
	snippedMessagesMu sync.RWMutex
)

func init() {
	snippedMessages = make(map[string]string)
}

// snipStructured marks a message as snippable for context compression
func (m *Manager) snipStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	content, _ := params["content"].(string)
	reason, _ := params["reason"].(string)

	if content == "" {
		return ToolResult{Type: "tool_result", Tool: ToolSnip, Status: "error", Error: "content parameter is required"}
	}

	// Use content itself as the key (matching SetSnipChecker callback signature)
	snippedMessagesMu.Lock()
	snippedMessages[content] = reason
	snippedMessagesMu.Unlock()

	slog.Debug("tools.snip.marked", "component", utils.ComponentTool, "content_len", len(content), "reason", reason)

	display := fmt.Sprintf("Marked content (%d bytes) for snipping", len(content))
	if reason != "" {
		display += ": " + reason
	}
	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolSnip,
		Status:  "success",
		Data:    map[string]interface{}{"content_length": len(content), "reason": reason},
		Display: display,
	}
}

// IsMessageSnipped checks if a message content has been marked for snipping
// This function is used as the snipCheck callback in SetSnipChecker
func IsMessageSnipped(content string) bool {
	if content == "" {
		return false
	}
	snippedMessagesMu.RLock()
	defer snippedMessagesMu.RUnlock()
	_, ok := snippedMessages[content]
	return ok
}

// GetSnipReason returns the reason a message content was snipped
// This function is used as the snipReasonFor callback in SetSnipChecker
func GetSnipReason(content string) string {
	if content == "" {
		return ""
	}
	snippedMessagesMu.RLock()
	defer snippedMessagesMu.RUnlock()
	return snippedMessages[content]
}

// ClearSnippedMessages clears all snipped message markers
func ClearSnippedMessages() {
	snippedMessagesMu.Lock()
	defer snippedMessagesMu.Unlock()
	snippedMessages = make(map[string]string)
}

// GetSnippedMessageIDs returns all snipped message IDs (for debugging)
func GetSnippedMessageIDs() []string {
	snippedMessagesMu.RLock()
	defer snippedMessagesMu.RUnlock()
	ids := make([]string, 0, len(snippedMessages))
	// Note: we can't return full content as IDs, so we return a count
	// The actual content is the key itself
	for range snippedMessages {
		ids = append(ids, "<content>")
	}
	return ids
}
