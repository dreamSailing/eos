package runtime

import (
	"context"
	"testing"

	"github.com/dreamSailing/vb-coding/internal/hooks"
)

func TestHookManagerTaskCompletedIgnoresMatcher(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"TaskCompleted": {
				{Matcher: "won't match", Hooks: []hooks.Handler{{Type: "command", Command: systemMessageCommand("done")}}},
			},
		},
	}
	dec, err := hm.TaskCompleted(context.Background(), "t", true, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Decision != "allow" {
		t.Fatalf("expected allow, got %q", dec.Decision)
	}
	if dec.AdditionalContext != "done" {
		t.Fatalf("expected context, got %q", dec.AdditionalContext)
	}
}

func TestHookManagerTeammateIdleIgnoresMatcher(t *testing.T) {
	hm := NewHookManager(nil)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"TeammateIdle": {
				{Matcher: "won't match", Hooks: []hooks.Handler{{Type: "command", Command: systemMessageCommand("idle")}}},
			},
		},
	}
	dec, err := hm.TeammateIdle(context.Background(), "planner", true, "", 12)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Decision != "allow" {
		t.Fatalf("expected allow, got %q", dec.Decision)
	}
	if dec.AdditionalContext != "idle" {
		t.Fatalf("expected context, got %q", dec.AdditionalContext)
	}
}

