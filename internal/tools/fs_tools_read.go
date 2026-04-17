package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"github.com/dreamSailing/eos/internal/i18n"
	"github.com/dreamSailing/eos/internal/pkg/utils"
)

func (m *Manager) readStructured(ctx context.Context, params map[string]any) ToolResult {
	lang := LanguageFromContext(ctx)
	mode, _ := params["mode"].(string)
	if mode == "" {
		mode = "file"
	}
	path, ok := params["path"].(string)
	if !ok {
		return ToolResult{Type: "tool_result", Tool: ToolRead, Status: "error", Error: i18n.T("tool.error.path_required", lang)}
	}

	// 自动去除可能的 @ 前缀（容错处理）
	if strings.HasPrefix(path, "@") {
		cleanPath := strings.TrimPrefix(path, "@")
		slog.Debug("read.auto_fix.remove_at_prefix", "original", path, "fixed", cleanPath)
		path = cleanPath
	}

	// 使用 utils.ResolvePath 进行统一的路径解析和验证
	// 它处理了 Windows 下的 / 开头路径、相对路径、路径遍历检查等
	res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), path)
	if !res.IsValid {
		slog.Error("read.resolve_path.error", "component", utils.ComponentTool, "path", path, "error", res.ErrMsg)
		return ToolResult{Type: "tool_result", Tool: ToolRead, Status: "error", Error: i18n.T("tool.error.outside_root", lang)}
	}

	ap := res.AbsPath
	rel := res.RelPath

	switch mode {
	case "file", "":
		return m.readFileContent(ctx, ap, rel, params)
	case "directory":
		return m.listDirectoryContent(ctx, ap, rel)
	case "exists":
		return m.checkPathExists(ctx, ap, rel)
	case "resolve":
		return m.resolvePath(ctx, ap, rel)
	default:
		return ToolResult{Type: "tool_result", Tool: ToolRead, Status: "error", Error: i18n.T("tool.error.unknown_mode", lang, mode)}
	}
}

func (m *Manager) readFileContent(ctx context.Context, ap, rel string, params map[string]any) ToolResult {
	lang := LanguageFromContext(ctx)
	exists, isDir, errPE := m.fileOps.PathExists(ap)
	if errPE != nil {
		slog.Error("read_file.error", "component", utils.ComponentTool, "path", ap, "err", errPE.Error())
		return ToolResult{Type: "tool_result", Tool: ToolRead, Status: "error", Error: fmt.Sprintf("%v", errPE)}
	}
	if !exists {
		slog.Error("read_file.not_found", "component", utils.ComponentTool, "path", ap)
		return ToolResult{Type: "tool_result", Tool: ToolRead, Status: "error", Error: fmt.Sprintf("%s: %s — 请用 search {mode: \"glob\"} 或 read {mode: \"directory\"} 确认路径", i18n.T("tool.error.file_not_found", lang), rel)}
	}
	if isDir {
		slog.Error("read_file.is_directory", "component", utils.ComponentTool, "path", ap)
		return ToolResult{Type: "tool_result", Tool: ToolRead, Status: "error", Error: i18n.T("tool.error.is_directory", lang)}
	}

	// Check for special binary file types that we can handle
	ext := strings.ToLower(filepath.Ext(ap))
	switch ext {
	case ".pdf":
		pages, _ := params["pages"].(string)
		content, err := ReadPDF(ap, pages)
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: ToolRead, Status: "error", Error: err.Error(), Display: fmt.Sprintf("错误：读取 PDF 失败：%s", err.Error())}
		}
		return ToolResult{Type: "tool_result", Tool: ToolRead, Status: "success", Data: map[string]interface{}{"path": rel, "content": content, "format": "pdf"}, Display: fmt.Sprintf("已读取 PDF：%s", rel)}
	case ".ipynb":
		content, err := ReadNotebook(ap, 2000)
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: ToolRead, Status: "error", Error: err.Error(), Display: fmt.Sprintf("错误：读取 Notebook 失败：%s", err.Error())}
		}
		return ToolResult{Type: "tool_result", Tool: ToolRead, Status: "success", Data: map[string]interface{}{"path": rel, "content": content, "format": "notebook"}, Display: fmt.Sprintf("已读取 Notebook：%s", rel)}
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		desc, data, err := ReadImage(ap)
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: ToolRead, Status: "error", Error: err.Error(), Display: fmt.Sprintf("错误：读取图片失败：%s", err.Error())}
		}
		result := ToolResult{Type: "tool_result", Tool: ToolRead, Status: "success", Data: data, Display: desc}
		return result
	}

	if !m.fileOps.IsTextFile(ap) {
		slog.Error("read_file.unsupported", "component", utils.ComponentTool, "path", ap)
		return ToolResult{Type: "tool_result", Tool: ToolRead, Status: "error", Error: i18n.T("tool.error.unsupported_file", lang)}
	}
	content, err := m.fileOps.ReadFile(ap)
	if err != nil {
		slog.Error("read_file.error", "component", utils.ComponentTool, "path", ap, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: ToolRead, Status: "error", Error: i18n.T("tool.error.read_error", lang, err)}
	}

	finalContent := content
	wasCompressed := false
	if ShouldCompress(content) {
		finalContent, wasCompressed = CompressToolOutput(content, "file")
		slog.Debug("read_file.compressed", "component", utils.ComponentTool,
			"path", ap,
			"original_size", len(content),
			"compressed_size", len(finalContent),
		)
	}

	data := map[string]any{
		"mode":     "file",
		"path":     filepath.ToSlash(rel),
		"bytes":    len(content),
		"content":  finalContent,
		"continue": true,
	}
	if wasCompressed {
		data["compressed"] = true
		data["original_bytes"] = len(content)
	}
	display := fmt.Sprintf("%d bytes", len(content))
	if wasCompressed {
		display = fmt.Sprintf("%d bytes (compressed)", len(content))
	}
	return ToolResult{Type: "tool_result", Tool: ToolRead, Status: "success", Data: data, Display: display}
}

func (m *Manager) listDirectoryContent(ctx context.Context, ap, rel string) ToolResult {
	lang := LanguageFromContext(ctx)
	entries, err := m.fileOps.ListDirectory(ap)
	if err != nil {
		slog.Error("list_directory.error", "component", utils.ComponentTool, "path", ap, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: ToolRead, Status: "error", Error: i18n.T("tool.error.read_error", lang, err)}
	}
	var sb strings.Builder
	sb.WriteString("Path: ")
	sb.WriteString(filepath.ToSlash(rel))
	sb.WriteString("\n")
	if len(entries) == 0 {
		sb.WriteString("No entries")
	} else {
		const maxDisplay = 5
		if len(entries) <= maxDisplay {
			sb.WriteString(strings.Join(entries, ", "))
		} else {
			sb.WriteString(strings.Join(entries[:maxDisplay], ", "))
			sb.WriteString(fmt.Sprintf(", ... (%d more)", len(entries)-maxDisplay))
		}
	}
	return ToolResult{Type: "tool_result", Tool: ToolRead, Status: "success", Data: map[string]any{"mode": "directory", "path": filepath.ToSlash(rel), "entries": entries}, Display: fmt.Sprintf("%d entries", len(entries))}
}

func (m *Manager) checkPathExists(_ context.Context, ap, rel string) ToolResult {
	_, err := os.Stat(ap)
	exists := err == nil
	return ToolResult{Type: "tool_result", Tool: ToolRead, Status: "success", Data: map[string]any{"mode": "exists", "path": filepath.ToSlash(rel), "exists": exists, "continue": true}, Display: fmt.Sprintf("exists: %v", exists)}
}

func (m *Manager) resolvePath(_ context.Context, ap, rel string) ToolResult {
	info, err := os.Stat(ap)
	status := "exists"
	isDir := false
	exists := err == nil
	if err != nil {
		if os.IsNotExist(err) {
			status = "missing"
		} else {
			status = "missing"
		}
	} else {
		if info.IsDir() {
			status = "directory"
			isDir = true
		}
	}

	cand := filepath.ToSlash(rel)
	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolRead,
		Status: "success",
		Data: map[string]any{
			"mode":      "resolve",
			"path":      cand,
			"status":    status,
			"candidate": cand,
			"exists":    exists,
			"is_dir":    isDir,
		},
	}
}
