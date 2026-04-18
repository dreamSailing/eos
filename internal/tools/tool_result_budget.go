package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/dreamSailing/eos/internal/pkg/utils"
)

// ToolResultBudget manages aggregate size of tool results per turn.
// When the total exceeds MaxTotalBytes, the largest results are persisted
// to disk and replaced with a preview (first PreviewBytes bytes).
const (
	// MaxTotalBytes is the aggregate budget for all tool results in a single turn (200KB)
	MaxTotalBytes = 200_000
	// PreviewBytes is how many bytes of the original result to keep inline
	PreviewBytes = 2000
	// PersistDir is the subdirectory under .vb for persisted tool results
	PersistDir = "tool_results"
)

// ToolResultBudget tracks and enforces the aggregate tool result size budget
type ToolResultBudget struct {
	mu          sync.Mutex
	totalBytes  int
	resultSizes map[string]int // toolCallID -> size
	persistDir  string
}

// NewToolResultBudget creates a new budget tracker
func NewToolResultBudget(workspaceRoot string) *ToolResultBudget {
	dir := filepath.Join(workspaceRoot, ".eos", PersistDir)
	return &ToolResultBudget{
		resultSizes: make(map[string]int),
		persistDir:  dir,
	}
}

// Reset clears the budget for a new turn
func (b *ToolResultBudget) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.totalBytes = 0
	b.resultSizes = make(map[string]int)
}

// CheckAndEnforce checks if adding a tool result would exceed the budget.
// If so, it persists the result to disk and returns a preview replacement.
// Returns the (possibly modified) content and true if it was truncated.
func (b *ToolResultBudget) CheckAndEnforce(toolCallID string, content string) (string, bool) {
	if b == nil {
		return content, false
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	contentSize := len(content)
	b.totalBytes += contentSize
	b.resultSizes[toolCallID] = contentSize

	if b.totalBytes <= MaxTotalBytes {
		return content, false
	}

	// Budget exceeded: find the largest result and persist it
	largestID := ""
	largestSize := 0
	for id, size := range b.resultSizes {
		if size > largestSize {
			largestSize = size
			largestID = id
		}
	}

	if largestID == "" || largestSize <= PreviewBytes {
		// Nothing big enough to offload
		return content, false
	}

	// Persist the largest result
	_ = b.persistLocked(largestID, content)

	// Return preview for the current if it's the largest, otherwise just mark it
	if largestID == toolCallID {
		preview := content
		if len(preview) > PreviewBytes {
			preview = preview[:PreviewBytes]
		}
		preview += fmt.Sprintf("\n\n...[result persisted to disk, original: %d bytes]...\n", contentSize)
		b.totalBytes -= (contentSize - len(preview))
		return preview, true
	}

	return content, false
}

// persistLocked saves the full result to disk (caller must hold lock)
func (b *ToolResultBudget) persistLocked(toolCallID, content string) error {
	if err := os.MkdirAll(b.persistDir, 0755); err != nil {
		slog.Warn("tools.budget.persist.mkdir_failed", "component", utils.ComponentTool, "error", err.Error())
		return err
	}

	path := filepath.Join(b.persistDir, toolCallID+".json")
	data := map[string]string{
		"id":      toolCallID,
		"content": content,
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, raw, 0644); err != nil {
		slog.Warn("tools.budget.persist.write_failed", "component", utils.ComponentTool, "error", err.Error())
		return err
	}

	slog.Info("tools.budget.persisted", "component", utils.ComponentTool, "id", toolCallID, "size", len(content), "path", path)
	return nil
}

// LoadPersistedResult loads a previously persisted tool result from disk
func LoadPersistedResult(workspaceRoot, toolCallID string) (string, error) {
	path := filepath.Join(workspaceRoot, ".eos", PersistDir, toolCallID+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("persisted result not found: %w", err)
	}
	var data map[string]string
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", err
	}
	return data["content"], nil
}

// TotalBytes returns the current total bytes tracked
func (b *ToolResultBudget) TotalBytes() int {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.totalBytes
}
