package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"github.com/dreamSailing/eos/internal/i18n"
	"github.com/dreamSailing/eos/internal/pkg/utils"
	"github.com/dreamSailing/eos/internal/tools/fileops"
)

func (m *Manager) fsStructured(ctx context.Context, params map[string]any) ToolResult {
	lang := LanguageFromContext(ctx)
	mode, _ := params["mode"].(string)
	if mode == "" {
		mode = "write"
	}

	switch mode {
	case "write":
		return m.fsWrite(ctx, params)
	case "create":
		return m.fsCreate(ctx, params)
	case "mkdir":
		return m.fsMkdir(ctx, params)
	case "delete":
		return m.fsDelete(ctx, params)
	case "move":
		return m.fsMove(ctx, params)
	case "copy":
		return m.fsCopy(ctx, params)
	case "diff":
		return m.fsDiff(ctx, params)
	case "exists", "file", "directory", "read":
		return ToolResult{
			Type:   "tool_result",
			Tool:   "fs",
			Status: "error",
			Error:  i18n.T("tool.error.fs.mode_unsupported", lang, mode),
		}
	default:
		return ToolResult{
			Type:   "tool_result",
			Tool:   "fs",
			Status: "error",
			Error:  i18n.T("tool.error.unknown_mode", lang, mode),
		}
	}
}

func (m *Manager) fsWrite(ctx context.Context, params map[string]any) ToolResult {
	lang := LanguageFromContext(ctx)
	path, okP := params["path"].(string)
	content, okC := params["content"].(string)
	if !okP || !okC {
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: i18n.T("tool.error.content_required", lang)}
	}
	path = normalizePathPlaceholder(path)

	res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), path)
	if !res.IsValid {
		slog.Error("fs.write.out_of_root", "component", utils.ComponentTool, "path", path, "error", res.ErrMsg)
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: i18n.T("tool.error.outside_root", lang)}
	}
	ap := res.AbsPath
	rel := res.RelPath

	slog.Debug("fs.write.start", "component", utils.ComponentTool, "path", ap, "rel", filepath.ToSlash(rel))
	if m.fileOps.IsTextFile(ap) {
		if old, err := m.fileOps.ReadFile(ap); err == nil {
			_, _ = m.fileOps.SaveVersionWithExtra(ap, old, fileops.VersionExtra{
				TraceID:   TraceIDFromContext(ctx),
				Tool:      ToolFS,
				Operation: "write",
			})
			slog.Debug("fs.write.snapshot_saved", "component", utils.ComponentTool, "path", ap)
		}
	}
	if err := m.fileOps.WriteFile(ap, content); err != nil {
		slog.Error("fs.write.error", "component", utils.ComponentTool, "path", ap, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: i18n.T("tool.error.write_error", lang, err)}
	}
	slog.Debug("fs.write.success", "component", utils.ComponentTool, "path", ap, "bytes_written", len(content))
	disp := fmt.Sprintf("Successfully wrote %d characters to %s", len(content), filepath.ToSlash(rel))
	return ToolResult{Type: "tool_result", Tool: "fs", Status: "success", Data: map[string]any{"mode": "write", "path": filepath.ToSlash(rel), "bytes_written": len(content)}, Display: disp}
}

func (m *Manager) fsCreate(ctx context.Context, params map[string]any) ToolResult {
	lang := LanguageFromContext(ctx)
	path, okP := params["path"].(string)
	if !okP {
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: i18n.T("tool.error.path_required", lang)}
	}
	path = normalizePathPlaceholder(path)
	fileType, _ := params["type"].(string)

	res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), path)
	if !res.IsValid {
		slog.Error("fs.create.out_of_root", "component", utils.ComponentTool, "path", path, "error", res.ErrMsg)
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: i18n.T("tool.error.outside_root", lang)}
	}
	ap := res.AbsPath
	rel := res.RelPath

	if fileType == "dir" {
		if err := m.fileOps.CreateDirectory(ap); err != nil {
			slog.Error("fs.create.error", "component", utils.ComponentTool, "path", ap, "err", err.Error())
			return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: i18n.T("tool.error.create_error", lang, err)}
		}
		disp := fmt.Sprintf("Successfully created directory: %s", filepath.ToSlash(rel))
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "success", Data: map[string]any{"mode": "create", "type": "dir", "path": filepath.ToSlash(rel)}, Display: disp}
	}

	content, _ := params["content"].(string)
	if err := os.MkdirAll(filepath.Dir(ap), 0755); err != nil {
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: i18n.T("tool.error.create_error", lang, err)}
	}
	if err := m.fileOps.WriteFile(ap, content); err != nil {
		slog.Error("fs.create.error", "component", utils.ComponentTool, "path", ap, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: i18n.T("tool.error.write_error", lang, err)}
	}
	disp := fmt.Sprintf("Successfully created file: %s", filepath.ToSlash(rel))
	return ToolResult{Type: "tool_result", Tool: "fs", Status: "success", Data: map[string]any{"mode": "create", "type": "file", "path": filepath.ToSlash(rel)}, Display: disp}
}

func (m *Manager) fsMkdir(ctx context.Context, params map[string]any) ToolResult {
	params["type"] = "dir"
	return m.fsCreate(ctx, params)
}

func (m *Manager) fsDelete(ctx context.Context, params map[string]any) ToolResult {
	lang := LanguageFromContext(ctx)
	path, ok := params["path"].(string)
	if !ok {
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: i18n.T("tool.error.path_required", lang)}
	}
	path = normalizePathPlaceholder(path)

	res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), path)
	if !res.IsValid {
		slog.Error("fs.delete.out_of_root", "component", utils.ComponentTool, "path", path, "error", res.ErrMsg)
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: i18n.T("tool.error.outside_root", lang)}
	}
	ap := res.AbsPath
	rel := res.RelPath

	exists, isDir, _ := m.fileOps.PathExists(ap)
	if !exists {
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: i18n.T("tool.error.file_not_found", lang)}
	}

	var err error
	if isDir {
		err = m.fileOps.DeleteDirectoryRecursive(ap)
	} else {
		err = m.fileOps.DeleteFile(ap)
	}

	if err != nil {
		slog.Error("fs.delete.error", "component", utils.ComponentTool, "path", ap, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: i18n.T("tool.error.delete_error", lang, err)}
	}
	disp := i18n.T("tool.success.deleted", lang, filepath.ToSlash(rel))
	return ToolResult{Type: "tool_result", Tool: "fs", Status: "success", Data: map[string]any{"mode": "delete", "path": filepath.ToSlash(rel)}, Display: disp}
}
