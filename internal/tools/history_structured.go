package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"github.com/dreamSailing/eos/internal/pkg/utils"
	"github.com/dreamSailing/eos/internal/tools/fileops"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (m *Manager) historyStructured(ctx context.Context, params map[string]any) ToolResult {
	mode, _ := params["mode"].(string)
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "list_files"
	}
	switch mode {
	case "list_files":
		files, err := scanVersionFiles(WorkspaceRootFromContext(ctx))
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: "history", Status: "error", Error: err.Error()}
		}
		out := make([]map[string]any, 0, len(files))
		for _, f := range files {
			out = append(out, map[string]any{
				"path":               f.PathRel,
				"version_count":      f.VersionCount,
				"last_modified":      f.LastModified.Format(time.RFC3339),
				"total_size":         f.TotalSize,
				"last_modified_unix": f.LastModified.Unix(),
			})
		}
		return ToolResult{Type: "tool_result", Tool: "history", Status: "success", Data: map[string]any{"mode": mode, "files": out}}
	case "list_versions":
		ap, rel, errMsg := resolvePathParam(WorkspaceRootFromContext(ctx), params, "path")
		if errMsg != "" {
			return ToolResult{Type: "tool_result", Tool: "history", Status: "error", Error: errMsg}
		}
		vs, err := m.fileOps.ListVersions(ap)
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: "history", Status: "error", Error: err.Error()}
		}
		items := make([]map[string]any, 0, len(vs))
		for _, v := range vs {
			items = append(items, map[string]any{
				"id":     v.ID,
				"path":   filepath.ToSlash(rel),
				"size":   v.Size,
				"sha256": v.SHA256,
			})
		}
		return ToolResult{Type: "tool_result", Tool: "history", Status: "success", Data: map[string]any{"mode": mode, "path": filepath.ToSlash(rel), "versions": items}}
	case "read_version":
		ap, rel, errMsg := resolvePathParam(WorkspaceRootFromContext(ctx), params, "path")
		if errMsg != "" {
			return ToolResult{Type: "tool_result", Tool: "history", Status: "error", Error: errMsg}
		}
		id, _ := params["version_id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			return ToolResult{Type: "tool_result", Tool: "history", Status: "error", Error: "version_id required"}
		}
		txt, err := m.fileOps.ReadVersion(ap, id)
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: "history", Status: "error", Error: err.Error()}
		}
		return ToolResult{Type: "tool_result", Tool: "history", Status: "success", Data: map[string]any{"mode": mode, "path": filepath.ToSlash(rel), "version_id": id, "content": txt}}
	case "rollback":
		ap, rel, errMsg := resolvePathParam(WorkspaceRootFromContext(ctx), params, "path")
		if errMsg != "" {
			return ToolResult{Type: "tool_result", Tool: "history", Status: "error", Error: errMsg}
		}
		id, _ := params["version_id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			return ToolResult{Type: "tool_result", Tool: "history", Status: "error", Error: "version_id required"}
		}
		content, err := m.fileOps.ReadVersion(ap, id)
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: "history", Status: "error", Error: err.Error()}
		}
		if err := sandboxWriteError(ctx, ap); err != nil {
			return ToolResult{Type: "tool_result", Tool: "history", Status: "error", Error: err.Error(), Display: "错误：" + err.Error()}
		}
		if m.fileOps.IsTextFile(ap) {
			if cur, err := m.fileOps.ReadFile(ap); err == nil {
				_, _ = m.fileOps.SaveVersionWithExtra(ap, cur, fileops.VersionExtra{
					TraceID:   TraceIDFromContext(ctx),
					Tool:      "history",
					Operation: "rollback",
				})
			}
		}
		if err := m.fileOps.WriteFile(ap, content); err != nil {
			return ToolResult{Type: "tool_result", Tool: "history", Status: "error", Error: err.Error()}
		}
		return ToolResult{Type: "tool_result", Tool: "history", Status: "success", Data: map[string]any{"mode": mode, "path": filepath.ToSlash(rel), "version_id": id}, Display: "Rolled back " + filepath.ToSlash(rel)}
	case "list_checkpoints":
		limit := toInt(params["limit"], 20)
		cps, err := fileops.ListCheckpointsUnder(WorkspaceRootFromContext(ctx), limit)
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: "history", Status: "error", Error: err.Error()}
		}
		items := make([]map[string]any, 0, len(cps))
		for _, c := range cps {
			mm := map[string]any{
				"trace_id":   c.TraceID,
				"created_at": c.CreatedAt,
				"updated_at": c.UpdatedAt,
				"user_text":  c.UserText,
				"file_count": c.FileCount,
			}
			if c.Success != nil {
				mm["success"] = *c.Success
			}
			items = append(items, mm)
		}
		return ToolResult{Type: "tool_result", Tool: "history", Status: "success", Data: map[string]any{"mode": mode, "checkpoints": items}}
	case "restore_checkpoint":
		traceID, _ := params["trace_id"].(string)
		traceID = strings.TrimSpace(traceID)
		if traceID == "" {
			return ToolResult{Type: "tool_result", Tool: "history", Status: "error", Error: "trace_id required"}
		}
		cp, err := fileops.LoadCheckpointUnder(WorkspaceRootFromContext(ctx), traceID)
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: "history", Status: "error", Error: err.Error()}
		}
		if cp == nil || len(cp.Files) == 0 {
			return ToolResult{Type: "tool_result", Tool: "history", Status: "success", Data: map[string]any{"mode": mode, "trace_id": traceID, "restored": 0}, Display: "No files to restore"}
		}
		keys := make([]string, 0, len(cp.Files))
		for k := range cp.Files {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		restored := 0
		var errs []string
		for _, rel := range keys {
			vid := strings.TrimSpace(cp.Files[rel])
			if strings.TrimSpace(rel) == "" || vid == "" {
				continue
			}
			ap, resRel, errMsg := resolvePathString(WorkspaceRootFromContext(ctx), rel)
			if errMsg != "" {
				errs = append(errs, rel+": "+errMsg)
				continue
			}
			content, err := m.fileOps.ReadVersion(ap, vid)
			if err != nil {
				errs = append(errs, rel+": "+err.Error())
				continue
			}
			if err := sandboxWriteError(ctx, ap); err != nil {
				errs = append(errs, rel+": "+err.Error())
				continue
			}
			if m.fileOps.IsTextFile(ap) {
				if cur, err := m.fileOps.ReadFile(ap); err == nil {
					_, _ = m.fileOps.SaveVersionWithExtra(ap, cur, fileops.VersionExtra{
						TraceID:   TraceIDFromContext(ctx),
						Tool:      "history",
						Operation: "restore_checkpoint",
					})
				}
			}
			if err := m.fileOps.WriteFile(ap, content); err != nil {
				errs = append(errs, rel+": "+err.Error())
				continue
			}
			restored++
			slog.Info("history.restore_checkpoint.file", "component", utils.ComponentTool, "path", resRel, "version_id", vid, "trace_id", traceID)
		}
		data := map[string]any{"mode": mode, "trace_id": traceID, "restored": restored}
		if len(errs) > 0 {
			data["errors"] = errs
		}
		return ToolResult{Type: "tool_result", Tool: "history", Status: "success", Data: data, Display: fmt.Sprintf("Restored %d file(s)", restored)}
	default:
		return ToolResult{Type: "tool_result", Tool: "history", Status: "error", Error: "unknown mode: " + mode}
	}
}

func resolvePathParam(root string, params map[string]any, key string) (abs string, rel string, errMsg string) {
	p, _ := params[key].(string)
	return resolvePathString(root, p)
}

func resolvePathString(root string, p string) (abs string, rel string, errMsg string) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", "", "path required"
	}
	p = normalizePathPlaceholder(p)
	res := utils.ResolvePathUnder(root, p)
	if !res.IsValid {
		return "", "", "path outside working directory"
	}
	return res.AbsPath, res.RelPath, ""
}

func scanVersionFiles(root string) ([]fileops.VersionFileEntry, error) {
	wd := strings.TrimSpace(root)
	if wd == "" {
		wd, _ = os.Getwd()
	}
	versionsDir := filepath.Join(wd, ".eos", "versions")
	if _, err := os.Stat(versionsDir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	type agg struct {
		cnt   int
		last  time.Time
		total int
	}
	byDir := map[string]*agg{}
	err := filepath.WalkDir(versionsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path == versionsDir {
				return nil
			}
			rel, e := filepath.Rel(versionsDir, path)
			if e == nil && strings.HasPrefix(rel, "_") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".content") {
			return nil
		}
		dir := filepath.Dir(path)
		relDir, e := filepath.Rel(versionsDir, dir)
		if e != nil || relDir == "." || strings.HasPrefix(relDir, "_") {
			return nil
		}
		a := byDir[relDir]
		if a == nil {
			a = &agg{}
			byDir[relDir] = a
		}
		a.cnt++
		if info, e2 := d.Info(); e2 == nil {
			if info.ModTime().After(a.last) {
				a.last = info.ModTime()
			}
			a.total += int(info.Size())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]fileops.VersionFileEntry, 0, len(byDir))
	for relDir, a := range byDir {
		out = append(out, fileops.VersionFileEntry{
			PathRel:      filepath.ToSlash(relDir),
			VersionCount: a.cnt,
			LastModified: a.last,
			TotalSize:    a.total,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastModified.After(out[j].LastModified) })
	return out, nil
}
