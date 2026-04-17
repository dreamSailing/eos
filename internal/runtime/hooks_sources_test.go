package runtime

import (
	"context"
	"testing"

	"github.com/dreamSailing/eos/internal/hooks"
)

func TestHookManagerManagedHooksOnlyFiltersUserSettings(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		ManagedHooksOnly: true,
		Hooks: map[string][]hooks.MatcherGroup{
			"PreToolUse": {
				{Source: "user_settings", Matcher: "bash", Hooks: []hooks.Handler{{Type: "command", Command: denyHookCommand()}}},
				{Source: "project_settings", Matcher: "bash", Hooks: []hooks.Handler{{Type: "command", Command: systemMessageCommand("ok")}}},
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
	if dec.AdditionalContext != "ok" {
		t.Fatalf("expected project hook to run, got %q", dec.AdditionalContext)
	}
}

func TestHookManagerEnabledHookSourcesOverridesManagedDefault(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		ManagedHooksOnly:  true,
		EnabledHookSources: []string{"user_settings"},
		Hooks: map[string][]hooks.MatcherGroup{
			"PreToolUse": {
				{Source: "user_settings", Matcher: "bash", Hooks: []hooks.Handler{{Type: "command", Command: systemMessageCommand("u")}}},
				{Source: "project_settings", Matcher: "bash", Hooks: []hooks.Handler{{Type: "command", Command: systemMessageCommand("p")}}},
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
	if dec.AdditionalContext != "u" {
		t.Fatalf("expected only user hook to run, got %q", dec.AdditionalContext)
	}
}

