package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// ipynbCell represents a single cell in a Jupyter notebook
type ipynbCell struct {
	ID         string   `json:"id,omitempty"`
	CellType   string   `json:"cell_type"`
	Source     []string `json:"source"`
	Outputs    []any    `json:"outputs,omitempty"`
	ExecutionCount *int `json:"execution_count,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// ipynbNotebook represents a Jupyter notebook
type ipynbNotebook struct {
	Cells                []ipynbCell    `json:"cells"`
	Metadata             map[string]any `json:"metadata"`
	NBFormat             int            `json:"nbformat"`
	NBFormatMinor        int            `json:"nbformat_minor"`
}

// notebookEditStructured handles the notebook_edit tool
func (m *Manager) notebookEditStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	path, _ := params["path"].(string)
	if path == "" {
		return ToolResult{
			Type: "tool_result", Tool: ToolNotebookEdit, Status: "error",
			Error: "path is required",
		}
	}

	// Validate path is within workspace
	if !filepath.IsAbs(path) {
		wsRoot := WorkspaceRootFromContext(ctx)
		if wsRoot != "" {
			path = filepath.Join(wsRoot, path)
		}
	}

	editMode, _ := params["edit_mode"].(string)
	if editMode == "" {
		editMode = "replace"
	}
	switch editMode {
	case "replace", "insert", "delete":
	default:
		return ToolResult{
			Type: "tool_result", Tool: ToolNotebookEdit, Status: "error",
			Error: fmt.Sprintf("invalid edit_mode: %s (valid: replace, insert, delete)", editMode),
		}
	}

	// Read notebook
	data, err := os.ReadFile(path)
	if err != nil {
		return ToolResult{
			Type: "tool_result", Tool: ToolNotebookEdit, Status: "error",
			Error: fmt.Sprintf("failed to read notebook: %s", err),
		}
	}

	var nb ipynbNotebook
	if err := json.Unmarshal(data, &nb); err != nil {
		return ToolResult{
			Type: "tool_result", Tool: ToolNotebookEdit, Status: "error",
			Error: fmt.Sprintf("failed to parse notebook: %s", err),
		}
	}

	sourceStr, _ := params["source"].(string)
	cellID, _ := params["cell_id"].(string)
	cellType, _ := params["cell_type"].(string)
	insertAfter, _ := params["insert_after"].(string)

	// Convert source string to []string (one line per element with newline)
	var sourceLines []string
	if sourceStr != "" {
		lines := strings.Split(sourceStr, "\n")
		for i, line := range lines {
			if i < len(lines)-1 {
				sourceLines = append(sourceLines, line+"\n")
			} else {
				sourceLines = append(sourceLines, line)
			}
		}
	}

	switch editMode {
	case "replace":
		if cellID == "" {
			return ToolResult{
				Type: "tool_result", Tool: ToolNotebookEdit, Status: "error",
				Error: "cell_id is required for replace mode",
			}
		}
		found := false
		for i := range nb.Cells {
			if nb.Cells[i].ID == cellID {
				nb.Cells[i].Source = sourceLines
				if cellType != "" {
					nb.Cells[i].CellType = cellType
				}
				found = true
				break
			}
		}
		if !found {
			return ToolResult{
				Type: "tool_result", Tool: ToolNotebookEdit, Status: "error",
				Error: fmt.Sprintf("cell_id %s not found", cellID),
			}
		}

	case "insert":
		newCell := ipynbCell{
			ID:       uuid.New().String(),
			CellType: cellType,
			Source:   sourceLines,
			Metadata: map[string]any{},
		}
		if newCell.CellType == "" {
			newCell.CellType = "code"
		}
		if newCell.CellType == "code" {
			newCell.Outputs = []any{}
			zero := 0
			newCell.ExecutionCount = &zero
		}

		if insertAfter == "" {
			// Insert at beginning
			nb.Cells = append([]ipynbCell{newCell}, nb.Cells...)
		} else {
			inserted := false
			for i := range nb.Cells {
				if nb.Cells[i].ID == insertAfter {
					// Insert after this cell
					nb.Cells = append(nb.Cells[:i+1], append([]ipynbCell{newCell}, nb.Cells[i+1:]...)...)
					inserted = true
					break
				}
			}
			if !inserted {
				return ToolResult{
					Type: "tool_result", Tool: ToolNotebookEdit, Status: "error",
					Error: fmt.Sprintf("insert_after cell_id %s not found", insertAfter),
				}
			}
		}

	case "delete":
		if cellID == "" {
			return ToolResult{
				Type: "tool_result", Tool: ToolNotebookEdit, Status: "error",
				Error: "cell_id is required for delete mode",
			}
		}
		found := false
		for i := range nb.Cells {
			if nb.Cells[i].ID == cellID {
				nb.Cells = append(nb.Cells[:i], nb.Cells[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			return ToolResult{
				Type: "tool_result", Tool: ToolNotebookEdit, Status: "error",
				Error: fmt.Sprintf("cell_id %s not found", cellID),
			}
		}
	}

	// Write back
	out, err := json.MarshalIndent(nb, "", "  ")
	if err != nil {
		return ToolResult{
			Type: "tool_result", Tool: ToolNotebookEdit, Status: "error",
			Error: fmt.Sprintf("failed to marshal notebook: %s", err),
		}
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return ToolResult{
			Type: "tool_result", Tool: ToolNotebookEdit, Status: "error",
			Error: fmt.Sprintf("failed to write notebook: %s", err),
		}
	}

	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolNotebookEdit,
		Status: "success",
		Data: map[string]interface{}{
			"path":        path,
			"edit_mode":   editMode,
			"cells_count": len(nb.Cells),
		},
		Display: fmt.Sprintf("Notebook %s: %s (%d cells)", editMode, filepath.Base(path), len(nb.Cells)),
	}
}
