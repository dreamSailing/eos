//go:build legacy

package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/dreamSailing/eos/internal/tools/fileops"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (rc *RuntimeCore) versionFilesDirForPath(absPath string) (string, string, error) {
	rel, err := rc.relWithinRoot(absPath)
	if err != nil {
		return "", "", filepath.SkipDir
	}
	root := rc.workingRoot()
	currentDir := filepath.Join(fileops.VersionFilesRoot(root), filepath.FromSlash(rel))
	legacyDir := filepath.Join(fileops.LegacyVersionFilesRoot(root), filepath.FromSlash(rel))
	return chooseRuntimeVersionDir(currentDir, legacyDir), rel, nil
}

func chooseRuntimeVersionDir(currentDir, legacyDir string) string {
	if info, err := os.Stat(currentDir); err == nil && info.IsDir() {
		return currentDir
	}
	if info, err := os.Stat(legacyDir); err == nil && info.IsDir() {
		return legacyDir
	}
	return currentDir
}

// ListVersionFiles 列出所有有版本的文件
func (rc *RuntimeCore) ListVersionFiles() ([]fileops.VersionFileEntry, error) {
	versionsDir := rc.versionsRoot()
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
		if d.Name() == "meta.json" || d.Name() == "_index.jsonl" {
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

	files := make([]fileops.VersionFileEntry, 0, len(byDir))
	for relDir, a := range byDir {
		files = append(files, fileops.VersionFileEntry{
			PathRel:      filepath.ToSlash(relDir),
			VersionCount: a.cnt,
			LastModified: a.last,
			TotalSize:    a.total,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].LastModified.After(files[j].LastModified)
	})

	return files, nil
}

// ListVersionsForPath 列出指定文件的所有版本
func (rc *RuntimeCore) ListVersionsForPath(absPath string) ([]fileops.VersionMeta, error) {
	versionsDir, rel, err := rc.versionFilesDirForPath(absPath)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".content") {
			ids = append(ids, strings.TrimSuffix(e.Name(), ".content"))
		}
	}
	sort.Strings(ids)

	var out []fileops.VersionMeta
	for _, id := range ids {
		p := filepath.Join(versionsDir, id+".content")
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		info, _ := os.Stat(p)
		sum := sha256.Sum256(b)
		vm := fileops.VersionMeta{
			ID:      id,
			PathRel: rel,
			Size:    len(b),
			SHA256:  hex.EncodeToString(sum[:]),
		}
		if info != nil {
			vm.Timestamp = info.ModTime()
		}
		out = append(out, vm)
	}
	return out, nil
}

// RollbackFile 回滚文件到指定版本
func (rc *RuntimeCore) RollbackFile(path string, versionID string) string {
	if rc.tm == nil {
		return "Tools manager not initialized"
	}
	return rc.tm.RollbackFile(map[string]any{
		"path":       path,
		"version_id": versionID,
	})
}

// GetVersionDiff 获取版本差异
func (rc *RuntimeCore) GetVersionDiff(path string, versionID string) (string, error) {
	if rc.tm == nil {
		return "", fmt.Errorf("tools manager not initialized")
	}
	return "", fmt.Errorf("not implemented")
}

// DeleteVersion 删除指定文件的单个版本
func (rc *RuntimeCore) DeleteVersion(path, versionID string) string {
	if rc.tm == nil {
		return "Tools manager not initialized"
	}

	absPath := rc.resolveWithinRoot(path)

	vs, err := rc.ListVersionsForPath(absPath)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	if len(vs) == 0 {
		return "Error: no versions found for this file"
	}

	if len(vs) == 1 {
		return "Error: cannot delete the last version. Use 'Delete all versions' instead."
	}

	return rc.tm.DeleteVersion(map[string]any{
		"path":       path,
		"version_id": versionID,
	})
}

// DeleteAllVersions 删除指定文件的所有版本
func (rc *RuntimeCore) DeleteAllVersions(path string) string {
	if rc.tm == nil {
		return "Tools manager not initialized"
	}

	return rc.tm.DeleteAllVersions(map[string]any{
		"path": path,
	})
}

// DeleteAllFileVersions 批量删除所有文件的所有版本
func (rc *RuntimeCore) DeleteAllFileVersions() string {
	if rc.tm == nil {
		return "Tools manager not initialized"
	}

	return rc.tm.DeleteAllFileVersions(map[string]any{})
}
