package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/dreamSailing/eos/internal/i18n"
	"github.com/dreamSailing/eos/internal/pkg/utils"
)

func (m *Manager) fsMove(ctx context.Context, params map[string]any) ToolResult {
	return m.fileOpStructured(ctx, "fs", m.fileOps.MoveFile, params)
}

func (m *Manager) fsCopy(ctx context.Context, params map[string]any) ToolResult {
	lang := LanguageFromContext(ctx)
	src, okS := params["source"].(string)
	dst, okD := params["destination"].(string)
	if !okS || !okD {
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: i18n.T("tool.error.source_destination_required", lang)}
	}

	src = normalizePathPlaceholder(src)
	dst = normalizePathPlaceholder(dst)

	resSrc := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), src)
	resDst := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), dst)
	if !resSrc.IsValid || !resDst.IsValid {
		slog.Error("fs.copy.out_of_root", "component", utils.ComponentTool, "src", src, "dst", dst, "src_error", resSrc.ErrMsg, "dst_error", resDst.ErrMsg)
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: i18n.T("tool.error.outside_root", lang)}
	}
	srcAp := resSrc.AbsPath
	dstAp := resDst.AbsPath
	relSrc := resSrc.RelPath
	relDst := resDst.RelPath

	if err := sandboxWriteError(ctx, dstAp); err != nil {
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: err.Error(), Display: "错误：" + err.Error()}
	}
	fileType, _ := params["type"].(string)
	if fileType == "" {
		if exists, isDir, _ := m.fileOps.PathExists(srcAp); exists && isDir {
			fileType = "dir"
		} else {
			fileType = "file"
		}
	}

	var err error
	if fileType == "dir" {
		err = m.fileOps.CopyDirectory(srcAp, dstAp)
	} else {
		err = m.fileOps.CopyFile(srcAp, dstAp)
	}

	if err != nil {
		slog.Error("fs.copy.error", "component", utils.ComponentTool, "src", srcAp, "dst", dstAp, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "fs", Status: "error", Error: i18n.T("tool.error.create_error", lang, err)}
	}
	return ToolResult{Type: "tool_result", Tool: "fs", Status: "success", Data: map[string]any{"mode": "copy", "type": fileType, "source": filepath.ToSlash(relSrc), "destination": filepath.ToSlash(relDst)}, Display: fmt.Sprintf("Successfully copied %s from %s to %s", fileType, filepath.ToSlash(relSrc), filepath.ToSlash(relDst))}
}

func (m *Manager) fileOpStructured(ctx context.Context, toolName string, op func(string, string) error, params map[string]any) ToolResult {
	lang := LanguageFromContext(ctx)
	src, dst, relSrc, relDst, errMsg := resolveSourceDestinationPaths(ctx, params)
	if errMsg != "" {
		if strings.Contains(errMsg, "source and destination") {
			return ToolResult{Type: "tool_result", Tool: toolName, Status: "error", Error: i18n.T("tool.error.source_destination_required", lang)}
		}
		slog.Error(toolName+".out_of_root", "component", utils.ComponentTool, "src", src, "dst", dst)
		return ToolResult{Type: "tool_result", Tool: toolName, Status: "error", Error: i18n.T("tool.error.outside_root", lang)}
	}
	if err := sandboxWriteError(ctx, src); err != nil {
		return ToolResult{Type: "tool_result", Tool: toolName, Status: "error", Error: err.Error(), Display: "错误：" + err.Error()}
	}
	if err := sandboxWriteError(ctx, dst); err != nil {
		return ToolResult{Type: "tool_result", Tool: toolName, Status: "error", Error: err.Error(), Display: "错误：" + err.Error()}
	}
	if err := op(src, dst); err != nil {
		slog.Error(toolName+".error", "component", utils.ComponentTool, "src", src, "dst", dst, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: toolName, Status: "error", Error: i18n.T("tool.error.move_error", lang, err)}
	}
	return ToolResult{Type: "tool_result", Tool: toolName, Status: "success", Data: map[string]any{"source": filepath.ToSlash(relSrc), "destination": filepath.ToSlash(relDst)}, Display: i18n.T("tool.success.moved", lang, filepath.ToSlash(relSrc), filepath.ToSlash(relDst))}
}

func resolveSourceDestinationPaths(ctx context.Context, params map[string]any) (string, string, string, string, string) {
	source, okS := params["source"].(string)
	dest, okD := params["destination"].(string)
	if !okS || !okD {
		return "", "", "", "", "source and destination required"
	}
	source = normalizePathPlaceholder(source)
	dest = normalizePathPlaceholder(dest)
	root := WorkspaceRootFromContext(ctx)
	resSrc := utils.ResolvePathUnder(root, source)
	resDst := utils.ResolvePathUnder(root, dest)
	if !resSrc.IsValid || !resDst.IsValid {
		return "", "", "", "", "path outside working directory"
	}
	return resSrc.AbsPath, resDst.AbsPath, resSrc.RelPath, resDst.RelPath, ""
}
