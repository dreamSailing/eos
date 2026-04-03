package skills

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dreamSailing/vb-coding/internal/config"
)

func ResolveScanDirs(workspaceRoot string, cfg *config.Config) []string {
	dirs := make([]string, 0, 8)
	seen := map[string]struct{}{}

	addDir := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return
		}
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		dir = filepath.Clean(dir)
		key := strings.ToLower(dir)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		dirs = append(dirs, dir)
	}

	if home, err := os.UserHomeDir(); err == nil {
		addDir(filepath.Join(home, ".vb", "skills"))
		addDir(filepath.Join(home, ".claude", "skills"))
		addDir(filepath.Join(home, ".trae", "skills"))
	}

	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot != "" {
		addDir(filepath.Join(workspaceRoot, ".vb", "skills"))
		addDir(filepath.Join(workspaceRoot, ".claude", "skills"))
		addDir(filepath.Join(workspaceRoot, ".trae", "skills"))
	}

	if cfg != nil {
		for _, dir := range config.GetEnabledSkillsDirs(cfg) {
			addDir(dir)
		}
	}

	return dirs
}
