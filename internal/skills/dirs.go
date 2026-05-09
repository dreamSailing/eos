package skills

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dreamSailing/eos/internal/config"
	pluginpkg "github.com/dreamSailing/eos/internal/pkg/plugins"
)

func ResolveScanDirs(workspaceRoot string, cfg *config.Config) []string {
	dirs := make([]string, 0, 12)
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
		addDir(filepath.Join(home, ".eos", "skills"))
		addDir(filepath.Join(home, ".claude", "skills"))
		addDir(filepath.Join(home, ".trae", "skills"))
		addDir(filepath.Join(home, ".eos", "commands"))
		addDir(filepath.Join(home, ".claude", "commands"))
		addDir(filepath.Join(home, ".trae", "commands"))
	}

	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot != "" {
		addDir(filepath.Join(workspaceRoot, ".eos", "skills"))
		addDir(filepath.Join(workspaceRoot, ".claude", "skills"))
		addDir(filepath.Join(workspaceRoot, ".trae", "skills"))
		addDir(filepath.Join(workspaceRoot, ".eos", "commands"))
		addDir(filepath.Join(workspaceRoot, ".claude", "commands"))
		addDir(filepath.Join(workspaceRoot, ".trae", "commands"))
	}

	if cfg != nil {
		for _, dir := range config.GetEnabledSkillsDirs(cfg) {
			addDir(dir)
		}
	}

	for _, dir := range pluginSkillDirs(workspaceRoot, cfg) {
		addDir(dir)
	}
	for _, dir := range pluginCommandDirs(workspaceRoot, cfg) {
		addDir(dir)
	}

	return dirs
}

func pluginSkillDirs(workspaceRoot string, cfg *config.Config) []string {
	return pluginComponentDirs(workspaceRoot, cfg, func(item pluginpkg.Manifest) bool { return item.HasSkills }, "skills")
}

func pluginCommandDirs(workspaceRoot string, cfg *config.Config) []string {
	return pluginComponentDirs(workspaceRoot, cfg, func(item pluginpkg.Manifest) bool { return item.HasCommands }, "commands")
}

func pluginComponentDirs(workspaceRoot string, cfg *config.Config, include func(pluginpkg.Manifest) bool, component string) []string {
	items, err := pluginpkg.Discover(workspaceRoot)
	if err != nil || len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if !include(item) {
			continue
		}
		enabled := true
		if cfgEnabled, ok := config.PluginEnabled(cfg, item.Name); ok {
			enabled = cfgEnabled
		}
		if !enabled {
			continue
		}
		out = append(out, filepath.Join(item.RootDir, component))
	}
	return out
}
