package tools

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
)

// snippedMessages tracks message IDs that should be snipped (trimmed from context)
var (
	snippedMessages   map[string]string // messageID -> reason
	snippedMessagesMu sync.RWMutex
)

func init() {
	snippedMessages = make(map[string]string)
}

// snipStructured marks a message as snippable for context compression
func (m *Manager) snipStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	messageID, _ := params["message_id"].(string)
	reason, _ := params["reason"].(string)

	if messageID == "" {
		return ToolResult{Type: "tool_result", Tool: ToolSnip, Status: "error", Error: "message_id parameter is required"}
	}

	snippedMessagesMu.Lock()
	snippedMessages[messageID] = reason
	snippedMessagesMu.Unlock()

	slog.Debug("tools.snip.marked", "component", utils.ComponentTool, "message_id", messageID, "reason", reason)

	display := fmt.Sprintf("Marked message %s for snipping", messageID)
	if reason != "" {
		display += ": " + reason
	}
	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolSnip,
		Status:  "success",
		Data:    map[string]interface{}{"message_id": messageID, "reason": reason},
		Display: display,
	}
}

// IsMessageSnipped checks if a message has been marked for snipping
func IsMessageSnipped(messageID string) bool {
	snippedMessagesMu.RLock()
	defer snippedMessagesMu.RUnlock()
	_, ok := snippedMessages[messageID]
	return ok
}

// GetSnipReason returns the reason a message was snipped
func GetSnipReason(messageID string) string {
	snippedMessagesMu.RLock()
	defer snippedMessagesMu.RUnlock()
	return snippedMessages[messageID]
}

// ClearSnippedMessages clears all snipped message markers
func ClearSnippedMessages() {
	snippedMessagesMu.Lock()
	defer snippedMessagesMu.Unlock()
	snippedMessages = make(map[string]string)
}

// GetSnippedMessageIDs returns all snipped message IDs
func GetSnippedMessageIDs() []string {
	snippedMessagesMu.RLock()
	defer snippedMessagesMu.RUnlock()
	ids := make([]string, 0, len(snippedMessages))
	for id := range snippedMessages {
		ids = append(ids, id)
	}
	return ids
}
