package runtime

import (
	"context"
	"sync/atomic"
	"testing"
	"strings"

	"github.com/dreamSailing/vb-coding/internal/hooks"
	ai "github.com/dreamSailing/vb-coding/internal/ai"
)

type fakeHookModel struct {
	out   string
	err   error
	calls int32
}

func (m *fakeHookModel) Chat(ctx context.Context, messages []ai.Message) (string, error) {
	atomic.AddInt32(&m.calls, 1)
	return m.out, m.err
}

func (m *fakeHookModel) ChatStream(ctx context.Context, messages []ai.Message, onDelta func(string), onReasoning func(string)) (string, error) {
	atomic.AddInt32(&m.calls, 1)
	return m.out, m.err
}

func TestHookManagerPromptHookDeniesPreToolUse(t *testing.T) {
	hm := NewHookManager(nil)
	hm.SetModel(&fakeHookModel{out: `{"ok": false, "reason": "no"}`})
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"PreToolUse": {
				{Matcher: "bash", Hooks: []hooks.Handler{{Type: "prompt", Prompt: "Check: $ARGUMENTS"}}},
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

func TestHookManagerAgentHookBlocksStop(t *testing.T) {
	hm := NewHookManager(nil)
	hm.SetAgentEvaluator(func(ctx context.Context, prompt string) (string, error) {
		return `{"ok": false, "reason": "continue"}`, nil
	})
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"Stop": {
				{Hooks: []hooks.Handler{{Type: "agent", Prompt: "Verify: $ARGUMENTS"}}},
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
	if dec.Reason == "" {
		t.Fatalf("expected reason")
	}
}

func TestHookManagerPromptHooksDeduplicateHandlers(t *testing.T) {
	hm := NewHookManager(nil)
	fm := &fakeHookModel{out: `{"ok": true}`}
	hm.SetModel(fm)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"PreToolUse": {
				{Matcher: "bash", Hooks: []hooks.Handler{
					{Type: "prompt", Prompt: "Check: $ARGUMENTS"},
					{Type: "prompt", Prompt: "Check: $ARGUMENTS"},
				}},
			},
		},
	}
	_, _ = hm.PreToolUse(context.Background(), "bash", map[string]any{"command": "echo hi"})
	if got := atomic.LoadInt32(&fm.calls); got != 1 {
		t.Fatalf("expected 1 model call, got %d", got)
	}
}

func TestHookManagerPromptHookSystemMessage(t *testing.T) {
	hm := NewHookManager(nil)
	fm := &fakeHookModel{out: `{"ok": true, "systemMessage": "hi"}`}
	hm.SetModel(fm)
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"PostToolUse": {
				{Matcher: "bash", Hooks: []hooks.Handler{{Type: "prompt", Prompt: "Check: $ARGUMENTS"}}},
			},
		},
	}
	dec, err := hm.PostToolUse(context.Background(), "bash", map[string]any{"command": "echo hi"}, map[string]any{"status": "success"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Decision != "allow" {
		t.Fatalf("expected allow, got %q", dec.Decision)
	}
	if dec.AdditionalContext != "hi" {
		t.Fatalf("expected system message, got %q", dec.AdditionalContext)
	}
}

func TestHookManagerAgentHookSuppressOutput(t *testing.T) {
	hm := NewHookManager(nil)
	hm.SetAgentEvaluator(func(ctx context.Context, prompt string) (string, error) {
		return `{"ok": true, "systemMessage": "hi", "suppressOutput": true}`, nil
	})
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"Stop": {
				{Hooks: []hooks.Handler{{Type: "agent", Prompt: "Verify: $ARGUMENTS"}}},
			},
		},
	}
	dec, err := hm.Stop(context.Background(), "done", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Decision != "allow" {
		t.Fatalf("expected allow, got %q", dec.Decision)
	}
	if dec.AdditionalContext != "" {
		t.Fatalf("expected empty additionalContext, got %q", dec.AdditionalContext)
	}
}

func TestHookManagerStatusMessageEmitsPhaseNote(t *testing.T) {
	hm := NewHookManager(nil)
	fm := &fakeHookModel{out: `{"ok": true}`}
	hm.SetModel(fm)
	var got []string
	hm.SetOnMeta(func(line string) {
		got = append(got, line)
	})
	hm.base = hooks.Config{
		Hooks: map[string][]hooks.MatcherGroup{
			"PreToolUse": {
				{Matcher: "bash", Hooks: []hooks.Handler{{Type: "prompt", Prompt: "Check: $ARGUMENTS", StatusMessage: "Running hook"}}},
			},
		},
	}
	_, _ = hm.PreToolUse(context.Background(), "bash", map[string]any{"command": "echo hi"})
	found := false
	for _, s := range got {
		if strings.Contains(s, "phase.note:HOOK_STATUS:Running hook") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected status phase.note, got %v", got)
	}
}
