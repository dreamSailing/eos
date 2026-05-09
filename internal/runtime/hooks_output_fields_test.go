package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/dreamSailing/eos/internal/hooks"
)

func continueFalseCommand(reason string) string {
	reason = strings.ReplaceAll(reason, `"`, `\"`)
	if runtime.GOOS == "windows" {
		return "$x=[Console]::In.ReadToEnd(); '{\"continue\":false,\"stopReason\":\"" + reason + "\"}'"
	}
	return "cat >/dev/null; echo '{\"continue\":false,\"stopReason\":\"" + reason + "\"}'"
}

func systemMessageCommand(msg string) string {
	msg = strings.ReplaceAll(msg, `"`, `\"`)
	if runtime.GOOS == "windows" {
		return "$x=[Console]::In.ReadToEnd(); '{\"systemMessage\":\"" + msg + "\"}'"
	}
	return "cat >/dev/null; echo '{\"systemMessage\":\"" + msg + "\"}'"
}

func suppressOutputCommand() string {
	if runtime.GOOS == "windows" {
		return "$x=[Console]::In.ReadToEnd(); '{\"hookSpecificOutput\":{\"additionalContext\":\"x\",\"suppressOutput\":true}}'"
	}
	return "cat >/dev/null; echo '{\"hookSpecificOutput\":{\"additionalContext\":\"x\",\"suppressOutput\":true}}'"
}

func TestHookManagerDisableAllHooksSkipsExecution(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		DisableAllHooks: true,
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
	if dec.Decision != "allow" {
		t.Fatalf("expected allow, got %q", dec.Decision)
	}
}

func TestHookCommandContinueFalseDeniesPreToolUse(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"PreToolUse": {
				{Matcher: "bash", Hooks: []hooks.Handler{{Type: "command", Command: continueFalseCommand("no")}}},
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
	if strings.TrimSpace(dec.Reason) == "" {
		t.Fatalf("expected reason")
	}
}

func TestHookCommandSystemMessageBecomesAdditionalContext(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"PostToolUse": {
				{Matcher: "bash", Hooks: []hooks.Handler{{Type: "command", Command: systemMessageCommand("hi")}}},
			},
		},
	}
	dec, err := hm.PostToolUse(context.Background(), "bash", map[string]any{"command": "echo hi"}, map[string]any{"status": "success"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(dec.AdditionalContext) != "hi" {
		t.Fatalf("expected additionalContext, got %q", dec.AdditionalContext)
	}
}

func TestHookCommandSuppressOutputClearsAdditionalContext(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"PostToolUse": {
				{Matcher: "bash", Hooks: []hooks.Handler{{Type: "command", Command: suppressOutputCommand()}}},
			},
		},
	}
	dec, err := hm.PostToolUse(context.Background(), "bash", map[string]any{"command": "echo hi"}, map[string]any{"status": "success"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(dec.AdditionalContext) != "" {
		t.Fatalf("expected empty additionalContext, got %q", dec.AdditionalContext)
	}
}

