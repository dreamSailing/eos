package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"github.com/dreamSailing/eos/internal/hooks"
	"github.com/dreamSailing/eos/internal/tools"
	"github.com/dreamSailing/eos/internal/pkg/utils"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type ToolConfig struct {
	Manager      *tools.Manager
	Name         string
	Desc         string
	Params       *schema.ParamsOneOf
	LoopDetector LoopDetector
	Dispatch     *DispatchTools
}

type ToolImpl struct {
	config *ToolConfig
}

func (impl *ToolImpl) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: impl.config.Name, Desc: impl.config.Desc, ParamsOneOf: impl.config.Params}, nil
}

func (impl *ToolImpl) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	if impl.config != nil && impl.config.Dispatch != nil {
		if allowed := impl.config.Dispatch.GetAllowedToolsOverride(); allowed != nil {
			name := strings.ToLower(strings.TrimSpace(impl.config.Name))
			if name != "" && name != tools.ToolSkill && !allowed[name] {
				return "Error: permission denied: tool not allowed", nil
			}
		}
	}

	args := map[string]any{}
	if strings.TrimSpace(argumentsInJSON) != "" {
		if err := utils.UnmarshalWithEscapeFix(argumentsInJSON, &args); err != nil {
			slog.Error("runtime.tools_node.json_parse.error", "component", utils.ComponentTool, "error", err, "arguments_preview", argumentsInJSON[:min(200, len(argumentsInJSON))])
			return "", err
		}
	}
	if impl.config.Manager == nil || strings.TrimSpace(impl.config.Name) == "" {
		return "", nil
	}

	preHookCtx := ""
	if impl.config.Dispatch != nil && impl.config.Dispatch.hookMgr != nil {
		dec, _ := impl.config.Dispatch.hookMgr.PreToolUse(ctx, impl.config.Name, args)
		if dec.UpdatedInput != nil {
			for k, v := range dec.UpdatedInput {
				args[k] = v
			}
		}
		if strings.EqualFold(dec.Decision, "ask") {
			dec.Decision = "deny"
			if strings.TrimSpace(dec.Reason) == "" {
				dec.Reason = "hook requested confirmation"
			}
		}
		if strings.EqualFold(dec.Decision, "deny") || strings.EqualFold(dec.Decision, "block") {
			msg := "Error: denied by hook"
			if strings.TrimSpace(dec.Reason) != "" {
				msg += ": " + strings.TrimSpace(dec.Reason)
			}
			if strings.TrimSpace(dec.AdditionalContext) != "" {
				msg += "\n\n" + strings.TrimSpace(dec.AdditionalContext)
			}
			return msg, nil
		}
		preHookCtx = strings.TrimSpace(dec.AdditionalContext)
	}

	if impl.config.LoopDetector != nil {
		if err := impl.config.LoopDetector.CheckLoop(impl.config.Name, args); err != nil {
			return "Error: " + err.Error(), nil
		}
	}

	if tools.SafetyGatePrompt != nil {
		call := tools.ToolCall{Tool: impl.config.Name, Parameters: args}
		if tools.SafetyGateClassify != nil {
			category, _, summary, dangerous := tools.SafetyGateClassify(call)
			if dangerous && (tools.SafetyGateSessionAllowed == nil || !tools.SafetyGateSessionAllowed(category)) {
				setSafetyPreview(ctx, impl.config.Manager, call)

				if impl.config.Dispatch != nil && impl.config.Dispatch.hookMgr != nil {
					dec, _ := impl.config.Dispatch.hookMgr.PermissionRequest(ctx, impl.config.Name, args, []map[string]any{
						{"type": "toolAlwaysAllow", "tool": impl.config.Name},
					})
					if dec.UpdatedInput != nil {
						for k, v := range dec.UpdatedInput {
							args[k] = v
						}
					}
					if strings.EqualFold(dec.Decision, "deny") || strings.EqualFold(dec.Decision, "block") {
						errMsg := fmt.Sprintf("operation denied by hook: %s", summary)
						if strings.TrimSpace(dec.Reason) != "" {
							errMsg = strings.TrimSpace(dec.Reason)
						}
						return "Error: " + errMsg, nil
					}
					if strings.EqualFold(dec.Decision, "allow") {
						if dec.AllowSession && tools.SafetyGateAllowSession != nil {
							tools.SafetyGateAllowSession(category)
						}
						goto safetyAllowed
					}
				}

				dec := tools.SafetyGatePrompt(ctx, category, summary)
				if dec == "deny" {
					errMsg := fmt.Sprintf("operation denied by user: %s", summary)
					return "Error: " + errMsg, nil
				}
				if dec == "session" && tools.SafetyGateAllowSession != nil {
					tools.SafetyGateAllowSession(category)
				}
			}
		}
	}
	safetyAllowed:

	calls := []tools.ToolCall{{Tool: impl.config.Name, Parameters: args}}
	srs := impl.config.Manager.ExecuteStructured(ctx, calls)
	if len(srs) == 0 {
		return "", nil
	}

	if impl.config.Name == tools.ToolSkill && srs[0].Status == "success" {
		if impl.config.Dispatch != nil {
			if raw, ok := srs[0].Data["allowed_tools"].([]string); ok {
				impl.config.Dispatch.SetAllowedToolsOverride(buildAllowedToolsMap(raw))
			} else {
				impl.config.Dispatch.SetAllowedToolsOverride(nil)
			}
		}
		if fork, ok := srs[0].Data["fork"].(bool); ok && fork {
			if impl.config.Dispatch != nil {
				out, err := runSkillForkFromToolResult(ctx, impl.config.Dispatch, srs[0])
				if err != nil {
					return "Error: " + err.Error(), nil
				}
				return out, nil
			}
		}
	}

	postHookCtx := ""
	if impl.config.Dispatch != nil && impl.config.Dispatch.hookMgr != nil {
		tr := map[string]any{
			"status":  srs[0].Status,
			"display": srs[0].Display,
			"error":   srs[0].Error,
		}
		var dec hooks.Decision
		if srs[0].Status == "success" {
			dec, _ = impl.config.Dispatch.hookMgr.PostToolUse(ctx, impl.config.Name, args, tr)
		} else {
			dec, _ = impl.config.Dispatch.hookMgr.PostToolUseFailure(ctx, impl.config.Name, args, tr)
		}
		if strings.EqualFold(dec.Decision, "block") && strings.TrimSpace(dec.Reason) != "" {
			postHookCtx = "Error: " + strings.TrimSpace(dec.Reason)
			if strings.TrimSpace(dec.AdditionalContext) != "" {
				postHookCtx += "\n\n" + strings.TrimSpace(dec.AdditionalContext)
			}
		} else {
		postHookCtx = strings.TrimSpace(dec.AdditionalContext)
		}
	}

	out := srs[0].Display
	if strings.TrimSpace(preHookCtx) != "" {
		out = strings.TrimSpace(out) + "\n\n" + preHookCtx
	}
	if strings.TrimSpace(postHookCtx) != "" {
		out = strings.TrimSpace(out) + "\n\n" + postHookCtx
	}
	return out, nil
}
