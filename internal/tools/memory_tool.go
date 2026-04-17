package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PendingMemorySuggestion holds a pending memory suggestion awaiting user confirmation
type PendingMemorySuggestion struct {
	ID      string
	File    string
	Content string
	Section string
	Accepted bool
	Rejected bool
	mu      sync.Mutex
	done    chan struct{}
}

var (
	pendingSuggestions   = make(map[string]*PendingMemorySuggestion)
	pendingSuggestionsMu sync.Mutex
)

// suggestMemoryStructured handles the suggest_memory tool
func (m *Manager) suggestMemoryStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	file, _ := params["file"].(string)
	content, _ := params["content"].(string)
	section, _ := params["section"].(string)

	if strings.TrimSpace(content) == "" {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolSuggestMemory,
			Status: "error",
			Error:  "content is required",
			Display: "错误：content 参数为必填项",
		}
	}

	// Default file
	if file == "" {
		file = "EOS.md"
	}

	// Validate file name
	allowedFiles := map[string]bool{
		"EOS.md":         true,
		".eos/Rules.md":  true,
	}
	if !allowedFiles[file] {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolSuggestMemory,
			Status: "error",
			Error:  fmt.Sprintf("file must be one of: EOS.md, .eos/Rules.md (got: %s)", file),
			Display: fmt.Sprintf("错误：无效的文件名 '%s'", file),
		}
	}

	// If OnMemorySuggestion callback is set, delegate to the UI for confirmation
	if OnMemorySuggestion != nil {
		suggestion := &PendingMemorySuggestion{
			ID:      fmt.Sprintf("mem-%d", len(pendingSuggestions)+1),
			File:    file,
			Content: content,
			Section: section,
			done:    make(chan struct{}),
		}

		pendingSuggestionsMu.Lock()
		pendingSuggestions[suggestion.ID] = suggestion
		pendingSuggestionsMu.Unlock()

		// Call the UI callback
		accepted := OnMemorySuggestion(suggestion.ID, file, content, section)

		if accepted {
			err := writeMemoryToFile(file, content, section)
			if err != nil {
				return ToolResult{
					Type:   "tool_result",
					Tool:   ToolSuggestMemory,
					Status: "error",
					Error:  err.Error(),
					Display: fmt.Sprintf("错误：写入 %s 失败：%s", file, err.Error()),
				}
			}
			return ToolResult{
				Type:   "tool_result",
				Tool:   ToolSuggestMemory,
				Status: "success",
				Data:   map[string]interface{}{"file": file, "content_length": len(content)},
				Display: fmt.Sprintf("记忆建议已接受并保存到 %s", file),
			}
		}

		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolSuggestMemory,
			Status: "success",
			Data:   map[string]interface{}{"rejected": true},
			Display: "记忆建议已被用户拒绝",
		}
	}

	// No callback set — auto-accept (for headless/CI mode)
	err := writeMemoryToFile(file, content, section)
	if err != nil {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolSuggestMemory,
			Status: "error",
			Error:  err.Error(),
			Display: fmt.Sprintf("错误：写入 %s 失败：%s", file, err.Error()),
		}
	}

	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolSuggestMemory,
		Status: "success",
		Data:   map[string]interface{}{"file": file, "content_length": len(content), "auto_accepted": true},
		Display: fmt.Sprintf("记忆已保存到 %s（自动接受，无 UI）", file),
	}
}

// writeMemoryToFile appends content to the specified memory file
func writeMemoryToFile(file, content, section string) error {
	// Determine path relative to working directory
	path := file
	if !filepath.IsAbs(path) {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		path = filepath.Join(dir, path)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	var sb strings.Builder

	// If file exists, read existing content
	existing, err := os.ReadFile(path)
	if err == nil && len(existing) > 0 {
		sb.WriteString("\n\n")
	}

	// Add section header if specified
	if section != "" {
		sb.WriteString("## " + section + "\n\n")
	}

	sb.WriteString(content)
	sb.WriteString("\n")

	// Append to file
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(sb.String())
	return err
}

// OnMemorySuggestion is called when the AI suggests adding to a memory file.
// Returns true if accepted, false if rejected.
var OnMemorySuggestion func(id, file, content, section string) bool

// typedMemoryStructured handles typed memory storage (user/feedback/project/reference)
func (m *Manager) typedMemoryStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	memType, _ := params["type"].(string)
	content, _ := params["content"].(string)
	file, _ := params["file"].(string)
	section, _ := params["section"].(string)

	if strings.TrimSpace(content) == "" {
		return ToolResult{
			Type:    "tool_result",
			Tool:    "typed_memory",
			Status:  "error",
			Error:   "content is required",
			Display: "错误：content 参数为必填项",
		}
	}

	// Import the memory types package logic inline
	mt := parseMemoryType(memType)
	if file == "" {
		file = mt.defaultFile()
	}

	// Delegate to the existing write logic
	err := writeMemoryToFile(file, content, section)
	if err != nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    "typed_memory",
			Status:  "error",
			Error:   err.Error(),
			Display: fmt.Sprintf("错误：写入 %s 失败：%s", file, err.Error()),
		}
	}

	return ToolResult{
		Type:    "tool_result",
		Tool:    "typed_memory",
		Status:  "success",
		Data:    map[string]interface{}{"type": string(mt), "file": file, "content_length": len(content)},
		Display: fmt.Sprintf("记忆已保存到 %s（类型：%s）", file, mt),
	}
}

// parseMemoryType converts a string to a typed memory type
func parseMemoryType(s string) memoryType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "feedback":
		return memoryTypeFeedback
	case "project":
		return memoryTypeProject
	case "reference":
		return memoryTypeReference
	default:
		return memoryTypeUser
	}
}

type memoryType string

const (
	memoryTypeUser      memoryType = "user"
	memoryTypeFeedback  memoryType = "feedback"
	memoryTypeProject   memoryType = "project"
	memoryTypeReference memoryType = "reference"
)

func (t memoryType) defaultFile() string {
	switch t {
	case memoryTypeProject:
		return ".eos/Rules.md"
	default:
		return "EOS.md"
	}
}
