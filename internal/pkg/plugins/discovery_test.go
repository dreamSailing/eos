package plugins

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverFindsManifestPluginsAndComponents(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	userPlugin := filepath.Join(home, ".eos", "plugins", "shared-plugin")
	projectPlugin := filepath.Join(workspace, ".eos", "plugins", "review-kit")
	writePluginManifest(t, userPlugin, "shared-plugin", "shared plugin")
	writePluginManifest(t, projectPlugin, "review-kit", "review plugin")
	mustMkdir(t, filepath.Join(projectPlugin, "skills", "review"))
	mustWriteFile(t, filepath.Join(projectPlugin, "hooks", "hooks.json"), `{"hooks":{"PreToolUse":[{"matcher":"bash","hooks":[{"type":"command","command":"echo ok"}]}]}}`)
	mustWriteFile(t, filepath.Join(projectPlugin, ".mcp.json"), `{}`)

	items, err := Discover(workspace)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("plugin count=%d, want 2 (%+v)", len(items), items)
	}

	var review Manifest
	found := false
	for _, item := range items {
		if item.Name == "review-kit" {
			review = item
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing review-kit in %+v", items)
	}
	if review.Location != "project" {
		t.Fatalf("Location=%q, want project", review.Location)
	}
	if !review.HasSkills || !review.HasHooks || !review.HasMCP {
		t.Fatalf("components=%v, want skills/hooks/mcp enabled", review.Components())
	}
}

func TestDiscoverPrefersProjectPluginOverUserPluginWithSameName(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	writePluginManifest(t, filepath.Join(home, ".eos", "plugins", "dup"), "dup", "user plugin")
	writePluginManifest(t, filepath.Join(workspace, ".trae", "plugins", "dup"), "dup", "project plugin")

	items, err := Discover(workspace)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("plugin count=%d, want 1", len(items))
	}
	if items[0].Description != "project plugin" {
		t.Fatalf("Description=%q, want project plugin", items[0].Description)
	}
	if items[0].Location != "project" {
		t.Fatalf("Location=%q, want project", items[0].Location)
	}
}

func TestFindOwningManifestResolvesAncestorPlugin(t *testing.T) {
	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins", "review-kit")
	writePluginManifest(t, pluginRoot, "review-kit", "review plugin")
	skillDir := filepath.Join(pluginRoot, "skills", "review")
	mustMkdir(t, skillDir)

	item, ok := FindOwningManifest(skillDir)
	if !ok {
		t.Fatalf("expected owning manifest")
	}
	if item.Name != "review-kit" {
		t.Fatalf("Name=%q, want review-kit", item.Name)
	}
	if item.RootDir != pluginRoot {
		t.Fatalf("RootDir=%q, want %q", item.RootDir, pluginRoot)
	}
}

func TestDiscoverSupportsManifestlessPlugins(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	pluginRoot := filepath.Join(workspace, ".trae", "plugins", "manifestless")
	mustMkdir(t, filepath.Join(pluginRoot, "skills", "review"))

	items, err := Discover(workspace)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("plugin count=%d, want 1", len(items))
	}
	if items[0].Name != "manifestless" {
		t.Fatalf("Name=%q, want manifestless", items[0].Name)
	}
	if !items[0].HasSkills {
		t.Fatalf("expected manifestless plugin to expose skills: %+v", items[0])
	}
}

func TestFindOwningManifestResolvesManifestlessPlugin(t *testing.T) {
	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins", "review-kit")
	skillDir := filepath.Join(pluginRoot, "skills", "review")
	mustMkdir(t, skillDir)

	item, ok := FindOwningManifest(skillDir)
	if !ok {
		t.Fatalf("expected owning manifest")
	}
	if item.Name != "review-kit" {
		t.Fatalf("Name=%q, want review-kit", item.Name)
	}
	if item.RootDir != pluginRoot {
		t.Fatalf("RootDir=%q, want %q", item.RootDir, pluginRoot)
	}
}

func TestPersistentDataDirSanitizesPluginName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	got := PersistentDataDir("formatter@internal")
	want := filepath.Join(home, ".eos", "plugins", "data", "formatter-internal")
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("PersistentDataDir()=%q, want %q", got, want)
	}
}

func writePluginManifest(t *testing.T, root string, name string, desc string) {
	t.Helper()
	mustMkdir(t, filepath.Join(root, ".claude-plugin"))
	mustWriteFile(t, filepath.Join(root, ".claude-plugin", "plugin.json"), `{"name":"`+name+`","description":"`+desc+`","version":"1.0.0"}`)
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
