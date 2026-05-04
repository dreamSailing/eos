package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"github.com/dreamSailing/eos/internal/pkg/utils"
	"github.com/dreamSailing/eos/internal/tools/fileops"

	"github.com/pmezard/go-difflib/difflib"
)

// resolveFilePath 解析文件路径，转换为绝对路径并验证安全性
// 返回 (绝对路径, 相对路径, 错误信息)
func resolveFilePath(params map[string]any, pathKey string) (string, string, string) {
	path, ok := params[pathKey].(string)
	if !ok {
		return "", "", "path required"
	}

	wd, _ := os.Getwd()
	ap := path
	if !filepath.IsAbs(ap) {
		ap = filepath.Join(wd, filepath.FromSlash(path))
	}
	rel, errRel := filepath.Rel(wd, ap)
	if errRel != nil || strings.HasPrefix(rel, "..") {
		slog.Error("path.out_of_root", "component", utils.ComponentTool, "path", ap)
		return "", "", "path outside working directory"
	}

	return ap, rel, ""
}

// DiffVersion 显示当前文件与指定版本的差异
func (m *Manager) DiffVersion(params map[string]any) string {
	ap, rel, errMsg := resolveFilePath(params, "path")
	if errMsg != "" {
		return fmt.Sprintf("Error: %s", errMsg)
	}

	versionID, okV := params["version_id"].(string)
	if !okV {
		return "Error: version_id required"
	}

	current, err := m.fileOps.ReadFile(ap)
	if err != nil {
		return fmt.Sprintf("Error reading current file: %v", err)
	}

	oldContent, err := m.fileOps.ReadVersion(ap, versionID)
	if err != nil {
		return fmt.Sprintf("Error reading version: %v", err)
	}

	// 生成 diff
	oldLines := difflib.SplitLines(oldContent)
	newLines := difflib.SplitLines(current)
	a := difflib.UnifiedDiff{A: oldLines, B: newLines, FromFile: "a/" + rel, ToFile: "b/" + rel, Context: 3}
	text, err := difflib.GetUnifiedDiffString(a)
	if err != nil {
		return fmt.Sprintf("Error generating diff: %v", err)
	}

	return text
}

// RollbackFile 回滚文件到指定版本
func (m *Manager) RollbackFile(params map[string]any) string {
	ap, _, errMsg := resolveFilePath(params, "path")
	if errMsg != "" {
		return fmt.Sprintf("Error: %s", errMsg)
	}

	versionID, okV := params["version_id"].(string)
	if !okV {
		return "Error: version_id required"
	}

	content, err := m.fileOps.ReadVersion(ap, versionID)
	if err != nil {
		return fmt.Sprintf("Error reading version: %v", err)
	}

	if err := m.fileOps.WriteFile(ap, content); err != nil {
		return fmt.Sprintf("Error writing file: %v", err)
	}

	return fmt.Sprintf("Successfully rolled back to version %s", versionID)
}

// DeleteVersion 删除指定文件的单个版本
func (m *Manager) DeleteVersion(params map[string]any) string {
	ap, _, errMsg := resolveFilePath(params, "path")
	if errMsg != "" {
		return fmt.Sprintf("Error: %s", errMsg)
	}

	versionID, okV := params["version_id"].(string)
	if !okV {
		return "Error: version_id required"
	}

	remaining, err := m.fileOps.DeleteVersion(ap, versionID)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	return fmt.Sprintf("Deleted version %s. %d version(s) remaining.", versionID, remaining)
}

// DeleteAllVersions 删除指定文件的所有版本
func (m *Manager) DeleteAllVersions(params map[string]any) string {
	ap, rel, errMsg := resolveFilePath(params, "path")
	if errMsg != "" {
		return fmt.Sprintf("Error: %s", errMsg)
	}

	err := m.fileOps.DeleteAllVersions(ap)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	return fmt.Sprintf("Deleted all versions for file: %s", rel)
}

// DeleteAllFileVersions 批量删除所有文件的所有版本
func (m *Manager) DeleteAllFileVersions(params map[string]any) string {
	wd, _ := os.Getwd()
	versionsDir := fileops.ExistingVersionWorkspaceRoot(wd)
	if err := os.RemoveAll(versionsDir); err != nil {
		if os.IsNotExist(err) {
			return "No file versions found"
		}
		return fmt.Sprintf("Error: %v", err)
	}
	return "Deleted all file versions"
}

// ListHistoryFiles 列出所有有版本历史的文件
func (m *Manager) ListHistoryFiles(params map[string]any) string {
	wd, _ := os.Getwd()
	versionsDir := fileops.ExistingVersionFilesRoot(wd)

	if _, err := os.Stat(versionsDir); err != nil {
		if os.IsNotExist(err) {
			return "No file versions found"
		}
		return fmt.Sprintf("Error: %v", err)
	}

	type agg struct {
		cnt int
	}
	byDir := map[string]*agg{}
	_ = filepath.WalkDir(versionsDir, func(path string, d os.DirEntry, err error) error {
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
		return nil
	})

	if len(byDir) == 0 {
		return "No file versions found"
	}
	keys := make([]string, 0, len(byDir))
	for k := range byDir {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString("Files with version history:\n")
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("  %s (%d version(s))\n", filepath.ToSlash(k), byDir[k].cnt))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// GetVersionInfo 获取指定文件的版本信息
func (m *Manager) GetVersionInfo(params map[string]any) string {
	ap, rel, errMsg := resolveFilePath(params, "path")
	if errMsg != "" {
		return fmt.Sprintf("Error: %s", errMsg)
	}

	versions, err := m.fileOps.ListVersions(ap)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	if len(versions) == 0 {
		return fmt.Sprintf("No versions found for file: %s", rel)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Version history for %s:\n", rel))
	for i, v := range versions {
		timeStr := v.Timestamp.Format("2006-01-02 15:04:05")
		sb.WriteString(fmt.Sprintf("  [%d] %s - %s (%d bytes)\n", i+1, v.ID, timeStr, v.Size))
	}

	return strings.TrimRight(sb.String(), "\n")
}
