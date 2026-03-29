package bridge

import (
	"os"
	"path/filepath"
	"strings"
)

func (rc *RuntimeCore) workingRoot() string {
	root := strings.TrimSpace(rc.GetActiveRoot())
	if root != "" {
		return filepath.Clean(root)
	}
	if wd, err := os.Getwd(); err == nil && strings.TrimSpace(wd) != "" {
		return filepath.Clean(wd)
	}
	return ""
}

func (rc *RuntimeCore) versionsRoot() string {
	root := rc.workingRoot()
	if root == "" {
		return filepath.Join(".vb", "versions")
	}
	return filepath.Join(root, ".vb", "versions")
}

func (rc *RuntimeCore) resolveWithinRoot(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return rc.workingRoot()
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	root := rc.workingRoot()
	if root == "" {
		return filepath.Clean(filepath.FromSlash(path))
	}
	return filepath.Join(root, filepath.FromSlash(path))
}

func (rc *RuntimeCore) relWithinRoot(path string) (string, error) {
	root := rc.workingRoot()
	if root == "" {
		return "", filepath.SkipDir
	}
	abs := strings.TrimSpace(path)
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, filepath.FromSlash(abs))
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	rel = filepath.Clean(rel)
	if rel == "." || rel == "" || strings.HasPrefix(rel, "..") {
		return "", filepath.SkipDir
	}
	return filepath.ToSlash(rel), nil
}
