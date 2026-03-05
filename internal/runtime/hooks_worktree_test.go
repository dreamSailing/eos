package runtime

import (
	"context"
	"testing"

	"github.com/dreamSailing/vb-coding/internal/hooks"
)

func TestHookManagerWorktreeCreateIgnoresMatcher(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"WorktreeCreate": {
				{Matcher: "won't match", Hooks: []hooks.Handler{{Type: "command", Command: systemMessageCommand("created")}}},
			},
		},
	}
	dec, err := hm.WorktreeCreate(context.Background(), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Decision != "allow" {
		t.Fatalf("expected allow, got %q", dec.Decision)
	}
	if dec.AdditionalContext != "created" {
		t.Fatalf("expected context, got %q", dec.AdditionalContext)
	}
}

func TestHookManagerWorktreeRemoveIgnoresMatcher(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"WorktreeRemove": {
				{Matcher: "won't match", Hooks: []hooks.Handler{{Type: "command", Command: systemMessageCommand("removed")}}},
			},
		},
	}
	dec, err := hm.WorktreeRemove(context.Background(), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Decision != "allow" {
		t.Fatalf("expected allow, got %q", dec.Decision)
	}
	if dec.AdditionalContext != "removed" {
		t.Fatalf("expected context, got %q", dec.AdditionalContext)
	}
}

