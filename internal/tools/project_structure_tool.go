package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"github.com/dreamSailing/eos/internal/pkg/utils"
)

// projectStructureStructured handles the project_structure tool
func (m *Manager) projectStructureStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	start := "."
	if p, ok := params["path"].(string); ok && strings.TrimSpace(p) != "" {
		start = normalizePathPlaceholder(strings.TrimSpace(p))
	}

	res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), start)
	if !res.IsValid {
		return ToolResult{Type: "tool_result", Tool: ToolProjectStructure, Status: "error", Error: "path outside working directory"}
	}
	startAbs := res.AbsPath
	startRel := filepath.ToSlash(res.RelPath)

	structure, err := generateProjectStructure(startAbs)
	if err != nil {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolProjectStructure,
			Status: "error",
			Error:  fmt.Sprintf("failed to generate structure: %v", err),
		}
	}

	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolProjectStructure,
		Status: "success",
		Data: map[string]interface{}{
			"structure": structure,
			"path":      startRel,
		},
		Display: fmt.Sprintf("Project structure for %s:\n%s", startRel, structure),
	}
}

// generateProjectStructure is similar to utils.GenerateProjectStructureFile but returns string
// and accepts a root path.
func generateProjectStructure(rootPath string) (string, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Project Directory Structure for: %s\n", rootPath))

	// Ignore list (same as in utils/env.go)
	ignoreDirs := map[string]bool{
		".git": true, ".idea": true, ".vscode": true,
		"node_modules": true, "dist": true, "build": true,
		"vendor": true, "__pycache__": true, ".DS_Store": true,
		"bin": true, "obj": true, "target": true,
	}

	var walk func(path string, prefix string, depth int)
	walk = func(path string, prefix string, depth int) {
		if depth > 5 { // Limit depth
			return
		}

		entries, err := os.ReadDir(path)
		if err != nil {
			// Ignore read errors for individual dirs
			return
		}

		filtered := make([]os.DirEntry, 0, len(entries))
		for _, e := range entries {
			if ignoreDirs[e.Name()] {
				continue
			}
			filtered = append(filtered, e)
		}

		count := len(filtered)
		for i, e := range filtered {
			isLast := i == count-1
			connector := "├── "
			if isLast {
				connector = "└── "
			}

			sb.WriteString(prefix + connector + e.Name())
			if e.IsDir() {
				sb.WriteString("/")
				sb.WriteString("\n")

				newPrefix := prefix + "│   "
				if isLast {
					newPrefix = prefix + "    "
				}
				walk(filepath.Join(path, e.Name()), newPrefix, depth+1)
			} else {
				sb.WriteString("\n")
			}
		}
	}

	walk(rootPath, "", 0)
	return sb.String(), nil
}
