package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/dreamSailing/eos/internal/memory"
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
	memType, _ := params["type"].(string)

	if strings.TrimSpace(content) == "" {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolSuggestMemory,
			Status: "error",
			Error:  "content is required",
			Display: "错误：content 参数为必填项",
		}
	}

	rootDir := workspaceRootOrPWD(ctx)
	store := memory.NewStore(rootDir)
	targetType := inferMemoryType(file, memType)
	writeRes, err := store.Upsert(memory.MemoryEntry{
		Type:    targetType,
		File:    resolveLegacyMemoryTarget(rootDir, file, targetType),
		Content: content,
		Section: section,
		Source:  "suggest_memory",
	})
	if err != nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolSuggestMemory,
			Status:  "error",
			Error:   err.Error(),
			Display: fmt.Sprintf("错误：写入记忆失败：%s", err.Error()),
		}
	}

	display := fmt.Sprintf("记忆已写入 %s", writeRes.Path)
	if writeRes.Deduped {
		display = fmt.Sprintf("记忆已存在，跳过重复写入：%s", writeRes.Path)
	}

	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolSuggestMemory,
		Status: "success",
		Data: map[string]interface{}{
			"file":           writeRes.Path,
			"content_length": len(content),
			"type":           string(targetType),
			"deduped":        writeRes.Deduped,
			"index_file":     writeRes.IndexPath,
		},
		Display: display,
	}
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

	rootDir := workspaceRootOrPWD(ctx)
	store := memory.NewStore(rootDir)
	mt := parseMemoryType(memType)
	resolvedType := memory.ParseMemoryType(string(mt))
	result, err := store.Upsert(memory.MemoryEntry{
		Type:    resolvedType,
		File:    resolveLegacyMemoryTarget(rootDir, file, resolvedType),
		Content: content,
		Section: section,
		Source:  "typed_memory",
	})
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
		Data: map[string]interface{}{
			"type":           string(resolvedType),
			"file":           result.Path,
			"content_length": len(content),
			"deduped":        result.Deduped,
			"index_file":     result.IndexPath,
		},
		Display: fmt.Sprintf("记忆已保存到 %s（类型：%s）", result.Path, resolvedType),
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
	memoryTypeUser      memoryType = "global"
	memoryTypeFeedback  memoryType = "global"
	memoryTypeProject   memoryType = "project"
	memoryTypeReference memoryType = "global"
)

func inferMemoryType(file string, requested string) memory.MemoryType {
	if mt := strings.ToLower(strings.TrimSpace(requested)); mt != "" {
		return memory.ParseMemoryType(mt)
	}
	file = strings.ToLower(strings.TrimSpace(file))
	switch file {
	case ".eos/rules.md", ".eos/memory/project.md", "project", "project.md":
		return memory.MemoryTypeProject
	case "eos.md", "~/.eos/rules.md", "~/.eos/memory/user.md", "global", "user", "user.md":
		return memory.MemoryTypeGlobal
	default:
		return memory.MemoryTypeProject
	}
}

func resolveLegacyMemoryTarget(rootDir string, file string, memType memory.MemoryType) string {
	file = strings.TrimSpace(file)
	switch strings.ToLower(file) {
	case "", "project", "project.md", ".eos/rules.md", ".eos/memory/project.md":
		return memory.ProjectMemoryPath(rootDir)
	case "global", "user", "user.md", "eos.md", "~/.eos/rules.md", "~/.eos/memory/user.md":
		return memory.GlobalMemoryPath()
	default:
		if strings.TrimSpace(file) != "" && strings.HasPrefix(file, "/") {
			return file
		}
		return memType.DefaultPath(rootDir)
	}
}

func workspaceRootOrPWD(ctx context.Context) string {
	if root := WorkspaceRootFromContext(ctx); strings.TrimSpace(root) != "" {
		return strings.TrimSpace(root)
	}
	return "."
}
