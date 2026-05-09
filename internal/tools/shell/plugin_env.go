package shell

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/dreamSailing/eos/internal/config"
	pluginpkg "github.com/dreamSailing/eos/internal/pkg/plugins"
)

func withPluginEnv(ctx context.Context, workingDir string) context.Context {
	bins := enabledPluginBinDirs(workingDir)
	if len(bins) == 0 {
		return ctx
	}
	pathValue := prependPath(os.Getenv("PATH"), bins)
	if strings.TrimSpace(pathValue) == "" {
		return ctx
	}
	return WithEnv(ctx, mergeEnv(os.Environ(), []string{"PATH=" + pathValue}))
}

func enabledPluginBinDirs(workingDir string) []string {
	workingDir = strings.TrimSpace(workingDir)
	if workingDir == "" {
		return nil
	}
	workspaceRoot := filepath.Clean(workingDir)
	cfg, _ := config.Load()
	items, err := pluginpkg.Discover(workspaceRoot)
	if err != nil || len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		if !item.HasBin {
			continue
		}
		enabled := true
		if cfgEnabled, ok := config.PluginEnabled(&cfg, item.Name); ok {
			enabled = cfgEnabled
		}
		if !enabled {
			continue
		}
		dir := filepath.Join(item.RootDir, "bin")
		key := strings.ToLower(filepath.Clean(dir))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, dir)
	}
	return out
}

func prependPath(current string, dirs []string) string {
	if len(dirs) == 0 {
		return strings.TrimSpace(current)
	}
	parts := make([]string, 0, len(dirs)+1)
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir != "" {
			parts = append(parts, dir)
		}
	}
	if strings.TrimSpace(current) != "" {
		parts = append(parts, current)
	}
	return strings.Join(parts, string(os.PathListSeparator))
}
