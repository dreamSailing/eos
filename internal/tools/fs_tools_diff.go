package tools

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"github.com/dreamSailing/eos/internal/i18n"
	"github.com/dreamSailing/eos/internal/pkg/utils"

	"github.com/pmezard/go-difflib/difflib"
)

func (m *Manager) generateDiffStructured(ctx context.Context, params map[string]any) ToolResult {
	lang := LanguageFromContext(ctx)
	path, okP := params["path"].(string)
	proposed, okC := params["proposed_content"].(string)
	if !okP || !okC {
		return ToolResult{Type: "tool_result", Tool: "generate_diff", Status: "error", Error: i18n.T("tool.error.content_required", lang)}
	}
	path = normalizePathPlaceholder(path)
	res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), path)
	if !res.IsValid {
		return ToolResult{Type: "tool_result", Tool: "generate_diff", Status: "error", Error: i18n.T("tool.error.outside_root", lang)}
	}
	ap := res.AbsPath
	rel := res.RelPath
	old := ""
	if m.fileOps.IsTextFile(ap) {
		if content, err := m.fileOps.ReadFile(ap); err == nil {
			old = content
		}
	}

	oldLines := difflib.SplitLines(old)
	newLines := difflib.SplitLines(proposed)
	addedLines := 0
	removedLines := 0

	a := difflib.UnifiedDiff{A: oldLines, B: newLines, FromFile: "a/" + filepath.ToSlash(rel), ToFile: "b/" + filepath.ToSlash(rel), Context: 3}
	text, err := difflib.GetUnifiedDiffString(a)
	if err != nil {
		slog.Error("generate_diff.error", "component", utils.ComponentTool, "path", rel, "proposed_len", len(proposed), "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "generate_diff", Status: "error", Error: i18n.T("tool.error.diff_error", lang, err)}
	}

	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "++") {
			addedLines++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			removedLines++
		}
	}

	changes := strings.TrimSpace(text) != ""
	if !changes && strings.TrimSpace(proposed) == "" {
		return ToolResult{Type: "tool_result", Tool: "generate_diff", Status: "success", Data: map[string]any{
			"path":          filepath.ToSlash(rel),
			"changes":       false,
			"text":          "New file (empty content)",
			"continue":      true,
			"added_lines":   0,
			"removed_lines": 0,
		}}
	}

	return ToolResult{Type: "tool_result", Tool: "generate_diff", Status: "success", Data: map[string]any{
		"path":          filepath.ToSlash(rel),
		"changes":       changes,
		"text":          text,
		"continue":      true,
		"added_lines":   addedLines,
		"removed_lines": removedLines,
	}}
}

func (m *Manager) fsDiff(ctx context.Context, params map[string]any) ToolResult {
	lang := LanguageFromContext(ctx)
	path, okP := params["path"].(string)
	proposed, okC := params["content"].(string)
	if !okP || !okC {
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: i18n.T("tool.error.content_required", lang)}
	}
	path = normalizePathPlaceholder(path)
	res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), path)
	if !res.IsValid {
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: i18n.T("tool.error.outside_root", lang)}
	}
	ap := res.AbsPath
	rel := res.RelPath
	old := ""
	if m.fileOps.IsTextFile(ap) {
		if content, err := m.fileOps.ReadFile(ap); err == nil {
			old = content
		}
	}

	oldLines := difflib.SplitLines(old)
	newLines := difflib.SplitLines(proposed)
	addedLines := 0
	removedLines := 0

	a := difflib.UnifiedDiff{A: oldLines, B: newLines, FromFile: "a/" + filepath.ToSlash(rel), ToFile: "b/" + filepath.ToSlash(rel), Context: 3}
	text, err := difflib.GetUnifiedDiffString(a)
	if err != nil {
		slog.Error("fs.diff.error", "component", utils.ComponentTool, "path", rel, "proposed_len", len(proposed), "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: err.Error()}
	}

	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "++") {
			addedLines++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			removedLines++
		}
	}

	return ToolResult{Type: "tool_result", Tool: "fs", Status: "success", Data: map[string]any{
		"mode":          "diff",
		"path":          filepath.ToSlash(rel),
		"changes":       strings.TrimSpace(text) != "",
		"text":          text,
		"continue":      true,
		"added_lines":   addedLines,
		"removed_lines": removedLines,
	}}
}
