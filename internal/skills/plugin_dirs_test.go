package skills

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dreamSailing/eos/internal/config"
)

func TestResolveScanDirsIncludesEnabledPluginSkills(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	pluginRoot := filepath.Join(workspace, ".claude", "plugins", "formatter")
	if err := os.MkdirAll(filepath.Join(pluginRoot, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(pluginRoot, "skills", "review"), 0o755); err != nil {
		t.Fatalf("mkdir skills: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".claude-plugin", "plugin.json"), []byte(`{"name":"formatter","description":"Format files"}`), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}

	dirs := ResolveScanDirs(workspace, &config.Config{})
	want := filepath.Join(pluginRoot, "skills")
	found := false
	for _, dir := range dirs {
		if filepath.Clean(dir) == filepath.Clean(want) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ResolveScanDirs()=%v, want %q", dirs, want)
	}

	wantCommandDir := filepath.Join(home, ".claude", "commands")
	foundCommandDir := false
	for _, dir := range dirs {
		if filepath.Clean(dir) == filepath.Clean(wantCommandDir) {
			foundCommandDir = true
			break
		}
	}
	if !foundCommandDir {
		t.Fatalf("ResolveScanDirs()=%v, want default commands dir %q", dirs, wantCommandDir)
	}
}

func TestResolveScanDirsSkipsDisabledPluginSkills(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	pluginRoot := filepath.Join(workspace, ".claude", "plugins", "formatter")
	if err := os.MkdirAll(filepath.Join(pluginRoot, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(pluginRoot, "skills", "review"), 0o755); err != nil {
		t.Fatalf("mkdir skills: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".claude-plugin", "plugin.json"), []byte(`{"name":"formatter","description":"Format files"}`), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}

	cfg := config.Config{}
	config.SetPluginEnabled(&cfg, "formatter", false)
	dirs := ResolveScanDirs(workspace, &cfg)
	want := filepath.Join(pluginRoot, "skills")
	for _, dir := range dirs {
		if filepath.Clean(dir) == filepath.Clean(want) {
			t.Fatalf("ResolveScanDirs()=%v, did not expect %q", dirs, want)
		}
	}
}

func TestLoaderPluginSkillUsesNamespacedNameAndFolderFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	workspace := t.TempDir()
	skillDir := filepath.Join(workspace, ".claude", "plugins", "formatter", "skills", "review")
	if err := os.MkdirAll(filepath.Join(workspace, ".claude", "plugins", "formatter", ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir manifest: %v", err)
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".claude", "plugins", "formatter", ".claude-plugin", "plugin.json"), []byte(`{"name":"formatter","description":"Format files"}`), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: Review files\n---\n\nbody"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	loader := NewLoader()
	loader.SetSkillsDirs([]string{filepath.Join(workspace, ".claude", "plugins", "formatter", "skills")})
	if err := loader.Scan(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	skill, ok := loader.Get("formatter:review")
	if !ok || skill == nil {
		t.Fatalf("expected formatter:review in %+v", loader.List())
	}
	if skill.PluginName != "formatter" {
		t.Fatalf("PluginName=%q, want formatter", skill.PluginName)
	}
	if skill.Location != "project" {
		t.Fatalf("Location=%q, want project", skill.Location)
	}
}

func TestResolveScanDirsIncludesEnabledPluginCommands(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	pluginRoot := filepath.Join(workspace, ".claude", "plugins", "formatter")
	if err := os.MkdirAll(filepath.Join(pluginRoot, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(pluginRoot, "commands"), 0o755); err != nil {
		t.Fatalf("mkdir commands: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".claude-plugin", "plugin.json"), []byte(`{"name":"formatter","description":"Format files"}`), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}

	dirs := ResolveScanDirs(workspace, &config.Config{})
	want := filepath.Join(pluginRoot, "commands")
	for _, dir := range dirs {
		if filepath.Clean(dir) == filepath.Clean(want) {
			return
		}
	}
	t.Fatalf("ResolveScanDirs()=%v, want %q", dirs, want)
}

func TestLoaderPluginCommandUsesNamespacedName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	workspace := t.TempDir()
	commandDir := filepath.Join(workspace, ".claude", "plugins", "formatter", "commands")
	if err := os.MkdirAll(filepath.Join(workspace, ".claude", "plugins", "formatter", ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir manifest: %v", err)
	}
	if err := os.MkdirAll(commandDir, 0o755); err != nil {
		t.Fatalf("mkdir commands: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".claude", "plugins", "formatter", ".claude-plugin", "plugin.json"), []byte(`{"name":"formatter","description":"Format files"}`), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(commandDir, "review.md"), []byte("Review changed files carefully."), 0o644); err != nil {
		t.Fatalf("write command markdown: %v", err)
	}

	loader := NewLoader()
	loader.SetSkillsDirs([]string{commandDir})
	if err := loader.Scan(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	skill, ok := loader.Get("formatter:review")
	if !ok || skill == nil {
		t.Fatalf("expected formatter:review in %+v", loader.List())
	}
	if skill.Kind != "command" {
		t.Fatalf("Kind=%q, want command", skill.Kind)
	}
	if skill.Description != "Review changed files carefully." {
		t.Fatalf("Description=%q, want derived first paragraph", skill.Description)
	}
}

func TestLoaderUsesFallbackNameAndAllowedToolsList(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	doc := strings.Join([]string{
		"---",
		"description: Review files",
		"allowed-tools:",
		"  - read",
		"  - grep",
		"---",
		"",
		"Review instructions",
	}, "\n")
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(doc), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	loader := NewLoader()
	loader.SetSkillsDirs([]string{root})
	if err := loader.Scan(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	skill, ok := loader.Get("review")
	if !ok || skill == nil {
		t.Fatalf("expected review in %+v", loader.List())
	}
	gotTools := skill.AllowedTools.Values()
	if len(gotTools) != 2 || gotTools[0] != "read" || gotTools[1] != "grep" {
		t.Fatalf("AllowedTools=%v, want [read grep]", gotTools)
	}
}
