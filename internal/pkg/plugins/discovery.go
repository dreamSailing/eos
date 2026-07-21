package plugins

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ManifestAuthor struct {
	Name string `json:"name,omitempty"`
}

type Manifest struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Version     string         `json:"version,omitempty"`
	Author      ManifestAuthor `json:"author,omitempty"`

	RootDir      string `json:"-"`
	ManifestPath string `json:"-"`
	Location     string `json:"-"`

	HasCommands bool `json:"-"`
	HasAgents   bool `json:"-"`
	HasSkills   bool `json:"-"`
	HasHooks    bool `json:"-"`
	HasMCP      bool `json:"-"`
	HasLSP      bool `json:"-"`
	HasSettings bool `json:"-"`
	HasBin      bool `json:"-"`
}

func ResolveScanDirs(workspaceRoot string) []string {
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
		addDir(filepath.Join(home, ".eos", "plugins"))
		addDir(filepath.Join(home, ".claude", "plugins"))
		addDir(filepath.Join(home, ".trae", "plugins"))
	}

	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot != "" {
		addDir(filepath.Join(workspaceRoot, ".eos", "plugins"))
		addDir(filepath.Join(workspaceRoot, ".claude", "plugins"))
		addDir(filepath.Join(workspaceRoot, ".trae", "plugins"))
	}

	return dirs
}

func Discover(workspaceRoot string) ([]Manifest, error) {
	roots := ResolveScanDirs(workspaceRoot)
	found := map[string]Manifest{}
	for _, root := range roots {
		items, err := discoverInRoot(root)
		if err != nil {
			continue
		}
		for _, item := range items {
			key := strings.ToLower(strings.TrimSpace(item.Name))
			if key == "" {
				continue
			}
			existing, ok := found[key]
			if !ok || pluginLocationPriority(item.Location) >= pluginLocationPriority(existing.Location) {
				found[key] = item
			}
		}
	}

	out := make([]Manifest, 0, len(found))
	for _, item := range found {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func FindOwningManifest(path string) (Manifest, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Manifest{}, false
	}
	path = filepath.Clean(path)
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		path = filepath.Dir(path)
	}
	for {
		if item, ok := loadManifest(path); ok {
			return item, true
		}
		parent := filepath.Dir(path)
		if parent == path {
			return Manifest{}, false
		}
		path = parent
	}
}

func (m Manifest) HookConfigPath() string {
	return filepath.Join(m.RootDir, "hooks", "hooks.json")
}

func (m Manifest) MCPConfigPath() string {
	return filepath.Join(m.RootDir, ".mcp.json")
}

func PersistentDataDir(name string) string {
	name = sanitizePluginID(name)
	if name == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "plugins", "data", name)
}

func (m Manifest) Components() []string {
	out := make([]string, 0, 8)
	if m.HasCommands {
		out = append(out, "commands")
	}
	if m.HasAgents {
		out = append(out, "agents")
	}
	if m.HasSkills {
		out = append(out, "skills")
	}
	if m.HasHooks {
		out = append(out, "hooks")
	}
	if m.HasMCP {
		out = append(out, "mcp")
	}
	if m.HasLSP {
		out = append(out, "lsp")
	}
	if m.HasSettings {
		out = append(out, "settings")
	}
	if m.HasBin {
		out = append(out, "bin")
	}
	return out
}

func discoverInRoot(root string) ([]Manifest, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nil
	}
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	if item, ok := loadManifest(root); ok {
		return []Manifest{item}, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	out := make([]Manifest, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if item, ok := loadManifest(filepath.Join(root, entry.Name())); ok {
			out = append(out, item)
		}
	}
	return out, nil
}

func loadManifest(root string) (Manifest, bool) {
	manifestPath := filepath.Join(root, ".claude-plugin", "plugin.json")
	raw, err := os.ReadFile(manifestPath)
	hasManifest := err == nil
	var item Manifest
	if hasManifest {
		if err := json.Unmarshal(raw, &item); err != nil {
			return Manifest{}, false
		}
		item.Name = strings.TrimSpace(item.Name)
	}
	if item.Name == "" {
		if !isPluginRootCandidate(root) {
			return Manifest{}, false
		}
		item.Name = strings.TrimSpace(filepath.Base(root))
		if item.Name == "" {
			return Manifest{}, false
		}
	}
	item.Description = strings.TrimSpace(item.Description)
	item.Version = strings.TrimSpace(item.Version)
	item.RootDir = filepath.Clean(root)
	if hasManifest {
		item.ManifestPath = manifestPath
	}
	item.Location = inferLocation(item.RootDir)
	item.HasCommands = dirExists(filepath.Join(root, "commands"))
	item.HasAgents = dirExists(filepath.Join(root, "agents"))
	item.HasSkills = dirExists(filepath.Join(root, "skills"))
	item.HasHooks = fileExists(filepath.Join(root, "hooks", "hooks.json"))
	item.HasMCP = fileExists(filepath.Join(root, ".mcp.json"))
	item.HasLSP = fileExists(filepath.Join(root, ".lsp.json"))
	item.HasSettings = fileExists(filepath.Join(root, "settings.json"))
	item.HasBin = dirExists(filepath.Join(root, "bin"))
	if !hasManifest && !item.HasCommands && !item.HasAgents && !item.HasSkills && !item.HasHooks && !item.HasMCP && !item.HasLSP && !item.HasSettings && !item.HasBin {
		return Manifest{}, false
	}
	return item, true
}

func isPluginRootCandidate(root string) bool {
	root = strings.TrimSpace(root)
	if root == "" {
		return false
	}
	return strings.EqualFold(filepath.Base(filepath.Dir(filepath.Clean(root))), "plugins")
}

func inferLocation(root string) string {
	root = strings.ToLower(filepath.Clean(root))
	if home, err := os.UserHomeDir(); err == nil {
		home = strings.ToLower(filepath.Clean(home))
		if strings.HasPrefix(root, home) {
			return "user"
		}
	}
	return "project"
}

func pluginLocationPriority(loc string) int {
	switch strings.ToLower(strings.TrimSpace(loc)) {
	case "project":
		return 2
	case "user":
		return 1
	default:
		return 0
	}
}

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func sanitizePluginID(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}
