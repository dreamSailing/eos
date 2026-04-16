package bridge

import (
	"context"

	"github.com/dreamSailing/vb-coding/internal/hooks"
	einoruntime "github.com/dreamSailing/vb-coding/internal/runtime"
	"github.com/dreamSailing/vb-coding/internal/tools"
)

// hookRunnerAdapter bridges runtime.SafetyGate to tools.HookRunner interface
type hookRunnerAdapter struct {
	hooks einoruntime.SafetyGate
}

// PreToolUse implements tools.HookRunner.PreToolUse
func (a *hookRunnerAdapter) PreToolUse(ctx context.Context, toolName string, input map[string]any) (bool, map[string]any, error) {
	if a.hooks.Prompt == nil {
		return true, nil, nil
	}

	// Check if tool needs approval via safety gate classification
	if a.hooks.Classify != nil {
		call := tools.ToolCall{Tool: toolName, Parameters: input}
		category, _, summary, dangerous := a.hooks.Classify(call)
		if dangerous {
			if a.hooks.SessionAllowed != nil && a.hooks.SessionAllowed(category) {
				return true, nil, nil
			}
			dec := a.hooks.Prompt(ctx, category, summary)
			if dec == "deny" {
				return false, nil, nil
			}
			if dec == "session" && a.hooks.AllowSession != nil {
				a.hooks.AllowSession(category)
			}
		}
	}

	return true, nil, nil
}

// PostToolUse implements tools.HookRunner.PostToolUse
func (a *hookRunnerAdapter) PostToolUse(ctx context.Context, toolName string, input map[string]any, result map[string]any) error {
	// Safety gate doesn't have a post-tool-use hook; no-op
	return nil
}

// hookRunnerFromHookManager creates a HookRunner from a HookManager instance.
// This variant delegates to the full HookManager.PreToolUse/PostToolUse which
// runs the complete hooks pipeline (command, prompt, agent, HTTP handlers).
type hookManagerAdapter struct {
	hm *einoruntime.HookManager
}

// PreToolUse delegates to HookManager.PreToolUse and translates the decision
func (a *hookManagerAdapter) PreToolUse(ctx context.Context, toolName string, input map[string]any) (bool, map[string]any, error) {
	dec, err := a.hm.PreToolUse(ctx, toolName, input)
	if err != nil {
		return true, nil, nil // allow on error to avoid blocking
	}
	switch dec.Decision {
	case "deny", "block":
		return false, dec.UpdatedInput, nil
	default:
		return true, dec.UpdatedInput, nil
	}
}

// PostToolUse delegates to HookManager.PostToolUse
func (a *hookManagerAdapter) PostToolUse(ctx context.Context, toolName string, input map[string]any, result map[string]any) error {
	dec, err := a.hm.PostToolUse(ctx, toolName, input, result)
	if err != nil {
		return err
	}
	// Post hooks can block/deny but we only log; don't change the tool result
	_ = dec
	return nil
}

// adaptSafetyGateAsHookRunner creates a HookRunner from a SafetyGate (simple permission check)
func adaptSafetyGateAsHookRunner(sg einoruntime.SafetyGate) tools.HookRunner {
	return &hookRunnerAdapter{hooks: sg}
}

// adaptHookManagerAsHookRunner creates a HookRunner from a HookManager (full hooks pipeline)
func adaptHookManagerAsHookRunner(hm *einoruntime.HookManager) tools.HookRunner {
	return &hookManagerAdapter{hm: hm}
}

// Ensure adapters satisfy the interface
var _ tools.HookRunner = (*hookRunnerAdapter)(nil)
var _ tools.HookRunner = (*hookManagerAdapter)(nil)

// Compile-time check for hooks package dependency
var _ hooks.Decision
