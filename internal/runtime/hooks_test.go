package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dreamSailing/eos/internal/hooks"
	"github.com/dreamSailing/eos/internal/skills"
	"github.com/dreamSailing/eos/internal/tools"
)

func denyHookCommand() string {
	if runtime.GOOS == "windows" {
		return "$x=[Console]::In.ReadToEnd(); '{\"hookSpecificOutput\":{\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"blocked\"}}'"
	}
	return "cat >/dev/null; echo '{\"hookSpecificOutput\":{\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"blocked\"}}'"
}

func denyExit2Command() string {
	if runtime.GOOS == "windows" {
		return "$x=[Console]::In.ReadToEnd(); [Console]::Error.Write('blocked'); exit 2"
	}
	return "cat >/dev/null; echo 'blocked' 1>&2; exit 2"
}

func updatedInputCommand() string {
	if runtime.GOOS == "windows" {
		return "$x=[Console]::In.ReadToEnd(); '{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"allow\",\"updatedInput\":{\"command\":\"echo changed\"}}}'"
	}
	return "cat >/dev/null; echo '{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"allow\",\"updatedInput\":{\"command\":\"echo changed\"}}}'"
}

func decisionBlockCommand(reason string) string {
	reason = strings.ReplaceAll(reason, `"`, `\"`)
	if runtime.GOOS == "windows" {
		return "$x=[Console]::In.ReadToEnd(); '{\"decision\":\"block\",\"reason\":\"" + reason + "\"}'"
	}
	return "cat >/dev/null; echo '{\"decision\":\"block\",\"reason\":\"" + reason + "\"}'"
}

func permissionRequestAllowCommand() string {
	if runtime.GOOS == "windows" {
		return "$x=[Console]::In.ReadToEnd(); '{\"hookSpecificOutput\":{\"hookEventName\":\"PermissionRequest\",\"decision\":{\"behavior\":\"allow\",\"updatedInput\":{\"command\":\"echo safe\"},\"updatedPermissions\":[{\"type\":\"toolAlwaysAllow\",\"tool\":\"bash\"}]}}}'"
	}
	return "cat >/dev/null; echo '{\"hookSpecificOutput\":{\"hookEventName\":\"PermissionRequest\",\"decision\":{\"behavior\":\"allow\",\"updatedInput\":{\"command\":\"echo safe\"},\"updatedPermissions\":[{\"type\":\"toolAlwaysAllow\",\"tool\":\"bash\"}]}}}'"
}

func sessionStartContextCommand() string {
	if runtime.GOOS == "windows" {
		return "$x=[Console]::In.ReadToEnd(); '{\"hookSpecificOutput\":{\"hookEventName\":\"SessionStart\",\"additionalContext\":\"hello\"}}'"
	}
	return "cat >/dev/null; echo '{\"hookSpecificOutput\":{\"hookEventName\":\"SessionStart\",\"additionalContext\":\"hello\"}}'"
}

func notificationContextCommand() string {
	if runtime.GOOS == "windows" {
		return "$x=[Console]::In.ReadToEnd(); '{\"hookSpecificOutput\":{\"hookEventName\":\"Notification\",\"additionalContext\":\"noted\"}}'"
	}
	return "cat >/dev/null; echo '{\"hookSpecificOutput\":{\"hookEventName\":\"Notification\",\"additionalContext\":\"noted\"}}'"
}

func preCompactContextCommand() string {
	if runtime.GOOS == "windows" {
		return "$x=[Console]::In.ReadToEnd(); '{\"hookSpecificOutput\":{\"hookEventName\":\"PreCompact\",\"additionalContext\":\"compacting\"}}'"
	}
	return "cat >/dev/null; echo '{\"hookSpecificOutput\":{\"hookEventName\":\"PreCompact\",\"additionalContext\":\"compacting\"}}'"
}

func TestHookManagerPreToolUseDeny(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"PreToolUse": {
				{Matcher: "bash", Hooks: []hooks.Handler{{Type: "command", Command: denyHookCommand()}}},
			},
		},
	}
	dec, err := hm.PreToolUse(context.Background(), "bash", map[string]any{"command": "echo hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Decision != "deny" {
		t.Fatalf("expected deny, got %q", dec.Decision)
	}
	if dec.Reason == "" {
		t.Fatalf("expected reason")
	}
}

func TestHookManagerPreToolUseExit2Denies(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"PreToolUse": {
				{Matcher: "bash", Hooks: []hooks.Handler{{Type: "command", Command: denyExit2Command()}}},
			},
		},
	}
	dec, err := hm.PreToolUse(context.Background(), "bash", map[string]any{"command": "echo hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Decision != "deny" {
		t.Fatalf("expected deny, got %q", dec.Decision)
	}
}

func TestHookManagerPreToolUseUpdatedInput(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"PreToolUse": {
				{Matcher: "bash", Hooks: []hooks.Handler{{Type: "command", Command: updatedInputCommand()}}},
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
	if dec.UpdatedInput == nil {
		t.Fatalf("expected updatedInput")
	}
	if v, _ := dec.UpdatedInput["command"].(string); v != "echo changed" {
		t.Fatalf("expected command updated, got %v", dec.UpdatedInput["command"])
	}
}

func TestHookManagerUserPromptSubmitExit2Blocks(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"UserPromptSubmit": {
				{Matcher: "ignored", Hooks: []hooks.Handler{{Type: "command", Command: denyExit2Command()}}},
			},
		},
	}
	dec, err := hm.UserPromptSubmit(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Decision != "block" {
		t.Fatalf("expected block, got %q", dec.Decision)
	}
}

func TestHookManagerStopDecisionBlock(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"Stop": {
				{Hooks: []hooks.Handler{{Type: "command", Command: decisionBlockCommand("keep going")}}},
			},
		},
	}
	dec, err := hm.Stop(context.Background(), "done", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Decision != "block" {
		t.Fatalf("expected block, got %q", dec.Decision)
	}
	if strings.TrimSpace(dec.Reason) == "" {
		t.Fatalf("expected reason")
	}
}

func TestHookManagerPermissionRequestAllow(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"PermissionRequest": {
				{Matcher: "bash", Hooks: []hooks.Handler{{Type: "command", Command: permissionRequestAllowCommand()}}},
			},
		},
	}
	dec, err := hm.PermissionRequest(context.Background(), "bash", map[string]any{"command": "rm -rf"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Decision != "allow" {
		t.Fatalf("expected allow, got %q", dec.Decision)
	}
	if dec.UpdatedInput == nil {
		t.Fatalf("expected updatedInput")
	}
	if v, _ := dec.UpdatedInput["command"].(string); v != "echo safe" {
		t.Fatalf("expected updated command, got %v", dec.UpdatedInput["command"])
	}
	if !dec.AllowSession {
		t.Fatalf("expected allow session")
	}
}

func TestHookManagerSessionStartAddsContext(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"SessionStart": {
				{Matcher: "startup", Hooks: []hooks.Handler{{Type: "command", Command: sessionStartContextCommand()}}},
			},
		},
	}
	dec, err := hm.SessionStart(context.Background(), "startup", "m", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(dec.AdditionalContext) != "hello" {
		t.Fatalf("expected additionalContext, got %q", dec.AdditionalContext)
	}
}

func TestHookManagerNotificationAddsContext(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"Notification": {
				{Matcher: "permission_prompt", Hooks: []hooks.Handler{{Type: "command", Command: notificationContextCommand()}}},
			},
		},
	}
	dec, err := hm.Notification(context.Background(), "permission_prompt", "m", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(dec.AdditionalContext) != "noted" {
		t.Fatalf("expected additionalContext, got %q", dec.AdditionalContext)
	}
}

func TestHookManagerPreCompactAddsContext(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"PreCompact": {
				{Matcher: "auto", Hooks: []hooks.Handler{{Type: "command", Command: preCompactContextCommand()}}},
			},
		},
	}
	dec, err := hm.PreCompact(context.Background(), "auto", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(dec.AdditionalContext) != "compacting" {
		t.Fatalf("expected additionalContext, got %q", dec.AdditionalContext)
	}
}

func TestHookManagerConfigChangeExit2Blocks(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"ConfigChange": {
				{Matcher: "project_settings", Hooks: []hooks.Handler{{Type: "command", Command: denyExit2Command()}}},
			},
		},
	}
	dec, err := hm.ConfigChange(context.Background(), "project_settings", "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Decision != "block" {
		t.Fatalf("expected block, got %q", dec.Decision)
	}
}

func TestHookManagerSkillFrontmatterHookAppliesWhenActive(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "hook-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cmd := denyHookCommand()
	md := "---\nname: hook-skill\ndescription: test\nhooks:\n  PreToolUse:\n    - matcher: \"read\"\n      hooks:\n        - type: command\n          command: |\n            " + cmd + "\n---\n\nbody"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := skills.NewLoader()
	loader.SetSkillsDirs([]string{root})
	if err := loader.Scan(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	tm := tools.NewManager()
	sm := tools.NewSkillManager(loader, tm)
	tm.SetSkillManager(sm)

	if _, _, err := sm.InjectSkillWithArguments(context.Background(), "hook-skill", ""); err != nil {
		t.Fatalf("inject: %v", err)
	}

	hm := NewHookManager(tm)
	hm.base = hooks.Config{Hooks: map[string][]hooks.MatcherGroup{}}
	dec, err := hm.PreToolUse(context.Background(), "read", map[string]any{"path": "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Decision != "deny" {
		t.Fatalf("expected deny, got %q", dec.Decision)
	}
}
