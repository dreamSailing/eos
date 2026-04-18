package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/hooks"
	"github.com/dreamSailing/eos/internal/tools"
)

func TestLoadHookConfigIncludesEnabledPluginHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	workspace := t.TempDir()
	pluginRoot := filepath.Join(workspace, ".claude", "plugins", "formatter")
	if err := os.MkdirAll(filepath.Join(pluginRoot, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(pluginRoot, "hooks"), 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".claude-plugin", "plugin.json"), []byte(`{"name":"formatter","description":"Format project files"}`), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}
	hooksPath := filepath.Join(pluginRoot, "hooks", "hooks.json")
	raw := `{"description":"formatter hooks","hooks":{"PreToolUse":[{"matcher":"bash","hooks":[{"type":"command","command":"echo hi"}]}]}}`
	if err := os.WriteFile(hooksPath, []byte(raw), 0o644); err != nil {
		t.Fatalf("write hooks.json: %v", err)
	}

	cfg, err := loadHookConfig(workspace)
	if err != nil {
		t.Fatalf("loadHookConfig error: %v", err)
	}
	groups := cfg.Hooks["PreToolUse"]
	if len(groups) != 1 {
		t.Fatalf("len(cfg.Hooks[PreToolUse])=%d, want 1", len(groups))
	}
	if groups[0].Source != "plugin:formatter" {
		t.Fatalf("group source=%q, want plugin:formatter", groups[0].Source)
	}
	if groups[0].BaseDir != pluginRoot {
		t.Fatalf("group baseDir=%q, want %q", groups[0].BaseDir, pluginRoot)
	}
	if groups[0].SourcePath != hooksPath {
		t.Fatalf("group sourcePath=%q, want %q", groups[0].SourcePath, hooksPath)
	}
}

func TestLoadHookConfigSkipsDisabledPluginHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	workspace := t.TempDir()
	pluginRoot := filepath.Join(workspace, ".claude", "plugins", "formatter")
	if err := os.MkdirAll(filepath.Join(pluginRoot, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(pluginRoot, "hooks"), 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".claude-plugin", "plugin.json"), []byte(`{"name":"formatter"}`), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, "hooks", "hooks.json"), []byte(`{"hooks":{"PreToolUse":[{"matcher":"bash","hooks":[{"type":"command","command":"echo hi"}]}]}}`), 0o644); err != nil {
		t.Fatalf("write hooks.json: %v", err)
	}

	appCfg := config.Config{}
	config.SetPluginEnabled(&appCfg, "formatter", false)
	if err := config.Save(appCfg, config.Path()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	cfg, err := loadHookConfig(workspace)
	if err != nil {
		t.Fatalf("loadHookConfig error: %v", err)
	}
	if len(cfg.Hooks["PreToolUse"]) != 0 {
		t.Fatalf("expected disabled plugin hooks to be skipped, got %+v", cfg.Hooks["PreToolUse"])
	}
}

func TestHookManagerManagedHooksOnlyIncludesPlugins(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		ManagedHooksOnly: true,
		Hooks: map[string][]hooks.MatcherGroup{
			"PreToolUse": {
				{Source: "user_settings", Matcher: "bash", Hooks: []hooks.Handler{{Type: "command", Command: systemMessageCommand("user")}}},
				{Source: "plugin:formatter", Matcher: "bash", Hooks: []hooks.Handler{{Type: "command", Command: systemMessageCommand("plugin")}}},
			},
		},
	}

	dec, err := hm.PreToolUse(context.Background(), "bash", map[string]any{"command": "echo hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Decision != "allow" {
		t.Fatalf("expected allow, got %q", dec.Decision)
	}
	if dec.AdditionalContext != "plugin" {
		t.Fatalf("expected plugin hook to run, got %q", dec.AdditionalContext)
	}
}

func TestHookManagerPluginHooksReceiveProjectAndPluginEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	workspace := t.TempDir()
	pluginRoot := filepath.Join(workspace, ".claude", "plugins", "formatter")

	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"PreToolUse": {
				{
					Source:  "plugin:formatter",
					BaseDir: pluginRoot,
					Matcher: "bash",
					Hooks: []hooks.Handler{{
						Type:    "command",
						Command: pluginEnvCommand(workspace, pluginRoot, filepath.Join(home, ".claude", "plugins", "data", "formatter")),
					}},
				},
			},
		},
	}

	ctx := tools.WithWorkspaceRoot(context.Background(), workspace)
	dec, err := hm.PreToolUse(ctx, "bash", map[string]any{"command": "echo hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Decision != "allow" {
		t.Fatalf("expected allow, got %q", dec.Decision)
	}
	if strings.TrimSpace(dec.AdditionalContext) != "env-ok" {
		t.Fatalf("expected env-ok, got %q", dec.AdditionalContext)
	}
}

func pluginEnvCommand(projectRoot string, pluginRoot string, pluginData string) string {
	projectRoot = strings.ReplaceAll(projectRoot, `'`, `''`)
	pluginRoot = strings.ReplaceAll(pluginRoot, `'`, `''`)
	pluginData = strings.ReplaceAll(pluginData, `'`, `''`)
	if goruntime.GOOS == "windows" {
		return "$x=[Console]::In.ReadToEnd(); if ($env:CLAUDE_PROJECT_DIR -eq '" + projectRoot + "' -and $env:CLAUDE_PLUGIN_ROOT -eq '" + pluginRoot + "' -and $env:CLAUDE_PLUGIN_DATA -eq '" + pluginData + "') { '{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"additionalContext\":\"env-ok\"}}' } else { [Console]::Error.Write('missing env'); exit 2 }"
	}
	return "cat >/dev/null; if [ \"$CLAUDE_PROJECT_DIR\" = '" + projectRoot + "' ] && [ \"$CLAUDE_PLUGIN_ROOT\" = '" + pluginRoot + "' ] && [ \"$CLAUDE_PLUGIN_DATA\" = '" + pluginData + "' ]; then echo '{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"additionalContext\":\"env-ok\"}}'; else echo 'missing env' 1>&2; exit 2; fi"
}
