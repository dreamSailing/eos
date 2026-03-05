package bridge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"github.com/dreamSailing/vb-coding/internal/tools/fileops"
)

// ListVersionFiles 列出所有有版本的文件
func (rc *RuntimeCore) ListVersionFiles() ([]fileops.VersionFileEntry, error) {
	wd, _ := os.Getwd()
	versionsDir := filepath.Join(wd, ".vb", "versions")
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
	wd, _ := os.Getwd()
	rel, err := filepath.Rel(wd, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil, filepath.SkipDir
	}

	versionsDir := filepath.Join(wd, ".vb", "versions", filepath.FromSlash(rel))
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

	wd, _ := os.Getwd()
	absPath := filepath.Join(wd, filepath.FromSlash(path))

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
