package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"github.com/dreamSailing/eos/internal/hooks"
	"github.com/dreamSailing/eos/internal/tools"
)

func (rt *EinoRuntime) ToolsNode(ctx context.Context, text string) (results []string, executed bool, wantContinue bool) {
	if rt.tools == nil {
		LogError("runtime.tools_node.no_tools")
		return nil, false, false
	}
	calls, strict := tools.ParseToolCallsStrict(text)
	if !strict {
		calls = tools.ParseToolCalls(text)
		if len(calls) > 0 {
			rt.ctxm.AddEphemeral("ParserFallback: prefer pure JSON tool call without extra text")
			LogWarn("tools.parser.fallback",
				"text_len", len(strings.TrimSpace(text)),
				"calls", len(calls))
		}
	}
	if len(calls) == 0 {
		return nil, false, false
	}
	LogDebug("runtime.tools_node.calls_parsed",
		"call_count", len(calls),
		"strict_mode", strict)
	if rt.allowedTools == nil && rt.dispatchTools != nil {
		if ov := rt.dispatchTools.GetAllowedToolsOverride(); ov != nil {
			rt.allowedTools = ov
		}
	}
	if rt.allowedTools != nil {
		var filtered []tools.ToolCall
		for _, c := range calls {
			if rt.allowedTools[strings.ToLower(strings.TrimSpace(c.Tool))] {
				filtered = append(filtered, c)
			} else {
				results = append(results, EventToolBlocked+":"+c.Tool)
			}
		}
		calls = filtered
		if len(calls) == 0 {
			return results, false, false
		}
	}
	for _, c := range calls {
		var keys []string
		for k := range c.Parameters {
			if len(keys) >= 2 {
				break
			}
			lk := strings.ToLower(k)
			if lk == "password" || lk == "token" || lk == "api_key" || lk == "secret" {
				continue
			}
			v := c.Parameters[k]
			sv := ""
			switch x := v.(type) {
			case string:
				if len(x) > 32 {
					sv = x[:32] + "…"
				} else {
					sv = x
				}
			case []interface{}:
				sv = "[" + strconv.Itoa(len(x)) + "]"
			default:
				bs, _ := json.Marshal(x)
				s := string(bs)
				if len(s) > 32 {
					sv = s[:32] + "…"
				} else {
					sv = s
				}
			}
			keys = append(keys, k+"="+sv)
		}
		summary := strings.Join(keys, ", ")
		if summary != "" {
			if c.ID != "" {
				results = append(results, EventToolCall+":"+c.ID+":"+c.Tool+" "+summary)
			} else {
				results = append(results, EventToolCall+":"+c.Tool+" "+summary)
			}
		} else {
			if c.ID != "" {
				results = append(results, EventToolCall+":"+c.ID+":"+c.Tool)
			} else {
				results = append(results, EventToolCall+":"+c.Tool)
			}
		}
	}
	var filtered []tools.ToolCall
	for _, c := range calls {
		if rt.turnWrapUp.Active {
			rt.handleBlockedToolDuringWrapUp(c.Tool)
			results = append(results, EventLoopBlock+":"+c.Tool+" (wrap_up)")
			if rt.ctxm != nil {
				rt.ctxm.AddToolObservation(rt.buildWrapUpToolResult(c.Tool))
			}
			continue
		}
		if rt.loopDetector != nil {
			if detailed, ok := any(rt.loopDetector).(*SlidingWindowLoopDetector); ok {
				if decision := detailed.CheckLoopResult(c.Tool, c.Parameters); decision != nil {
					if errors.Is(decision.Error(), ErrLoopForceBreak) {
						results = append(results, EventLoopBlock+":"+c.Tool+" (force break)")
						rt.activateTurnWrapUp(decision)
						return results, false, false
					}
					results = append(results, EventLoopBlock+":"+c.Tool+" (warning)")
					if rt.ctxm != nil {
						rt.ctxm.AddEphemeral(decision.Error().Error())
					}
					rt.emitLoopBlockEvent(decision)
					continue
				}
			} else if err := rt.loopDetector.CheckLoop(c.Tool, c.Parameters); err != nil {
				if errors.Is(err, ErrLoopForceBreak) {
					// 强制中断：停止处理后续工具调用
					results = append(results, EventLoopBlock+":"+c.Tool+" (force break)")
					if rt.ctxm != nil {
						rt.ctxm.AddEphemeral(err.Error())
					}
					return results, false, false
				}
				// 警告：注入提示但继续执行
				results = append(results, EventLoopBlock+":"+c.Tool+" (warning)")
				if rt.ctxm != nil {
					rt.ctxm.AddEphemeral(err.Error())
				}
				continue
			}
		}
		filtered = append(filtered, c)
	}
	if len(filtered) == 0 {
		return results, len(calls) > 0, false
	}
	var prepared []tools.ToolCall
	for _, c := range filtered {
		changed := false
		for k, v := range c.Parameters {
			ks := strings.ToLower(k)
			if ks == "path" || ks == "file" || ks == "target" {
				if s, ok := v.(string); ok && strings.HasPrefix(s, "@") {
					rr := rt.tools.ExecuteStructured(ctx, []tools.ToolCall{{Tool: "read", Parameters: map[string]interface{}{"mode": "resolve", "path": s}}})
					if len(rr) > 0 {
						r0 := rr[0]
						st := r0.Status
						if st == "success" {
							status, _ := r0.Data["status"].(string)
							cand, _ := r0.Data["candidate"].(string)
							switch status {
							case "exists":
								c.Parameters[k] = cand
								changed = true
							case "directory":
								c.Parameters[k] = cand
								changed = true
							case "missing":
								if c.Tool == "read" || c.Tool == "write_file" || c.Tool == "edit" {
									continue
								}
								c.Parameters[k] = cand
								changed = true
							}
						}
					}
				}
			}
		}
		prepared = append(prepared, c)
		_ = changed
	}
	execCalls := make([]tools.ToolCall, 0, len(prepared))
	execParams := make([]map[string]any, 0, len(prepared))
	allResults := make([]tools.ToolResult, 0, len(prepared))
	hookMgr := (*HookManager)(nil)
	if rt.dispatchTools != nil {
		hookMgr = rt.dispatchTools.hookMgr
	}
	for _, c := range prepared {
		if hookMgr != nil {
			dec, _ := hookMgr.PreToolUse(ctx, c.Tool, c.Parameters)
			if strings.TrimSpace(dec.AdditionalContext) != "" {
				rt.ctxm.AddEphemeral(strings.TrimSpace(dec.AdditionalContext))
			}
			if dec.UpdatedInput != nil {
				for k, v := range dec.UpdatedInput {
					if c.Parameters == nil {
						c.Parameters = map[string]any{}
					}
					c.Parameters[k] = v
				}
			}
			if strings.EqualFold(dec.Decision, "ask") {
				dec.Decision = "deny"
				if strings.TrimSpace(dec.Reason) == "" {
					dec.Reason = "hook requested confirmation"
				}
			}
			if strings.EqualFold(dec.Decision, "deny") || strings.EqualFold(dec.Decision, "block") {
				errMsg := "denied by hook"
				if strings.TrimSpace(dec.Reason) != "" {
					errMsg += ": " + strings.TrimSpace(dec.Reason)
				}
				allResults = append(allResults, tools.ToolResult{
					ID:      c.ID,
					Type:    "tool_result",
					Tool:    c.Tool,
					Status:  "error",
					Error:   errMsg,
					Display: "错误：" + errMsg,
				})
				continue
			}
		}

		if tools.SafetyGatePrompt != nil && tools.SafetyGateClassify != nil {
			category, _, summary, dangerous := tools.SafetyGateClassify(c)
			if dangerous && (tools.SafetyGateSessionAllowed == nil || !tools.SafetyGateSessionAllowed(category)) {
				setSafetyPreview(ctx, rt.tools, c)

				if hookMgr != nil {
					dec, _ := hookMgr.PermissionRequest(ctx, c.Tool, c.Parameters, []map[string]any{
						{"type": "toolAlwaysAllow", "tool": c.Tool},
					})
					if dec.UpdatedInput != nil {
						for k, v := range dec.UpdatedInput {
							if c.Parameters == nil {
								c.Parameters = map[string]any{}
							}
							c.Parameters[k] = v
						}
					}
					if strings.EqualFold(dec.Decision, "deny") || strings.EqualFold(dec.Decision, "block") {
						errMsg := "operation denied by hook"
						if strings.TrimSpace(dec.Reason) != "" {
							errMsg = strings.TrimSpace(dec.Reason)
						}
						allResults = append(allResults, tools.ToolResult{
							ID:      c.ID,
							Type:    "tool_result",
							Tool:    c.Tool,
							Status:  "error",
							Error:   errMsg,
							Display: "错误：" + errMsg,
						})
						continue
					}
					if strings.EqualFold(dec.Decision, "allow") {
						if dec.AllowSession && tools.SafetyGateAllowSession != nil {
							tools.SafetyGateAllowSession(category)
						}
						goto safetyAllowed
					}
				}

				pd := tools.SafetyGatePrompt(ctx, category, summary)
				if pd == "deny" {
					errMsg := "operation denied by user: " + summary
					allResults = append(allResults, tools.ToolResult{
						ID:      c.ID,
						Type:    "tool_result",
						Tool:    c.Tool,
						Status:  "error",
						Error:   errMsg,
						Display: "错误：" + errMsg,
					})
					continue
				}
				if pd == "session" && tools.SafetyGateAllowSession != nil {
					tools.SafetyGateAllowSession(category)
				}
			}
		}

	safetyAllowed:
		execCalls = append(execCalls, c)
		execParams = append(execParams, c.Parameters)
	}

	srs := rt.tools.ExecuteStructured(ctx, execCalls)
	if rt.dispatchTools != nil && rt.dispatchTools.hookMgr != nil {
		for i := range srs {
			r := srs[i]
			tr := map[string]any{"status": r.Status, "display": r.Display, "error": r.Error}
			var in map[string]any
			if i >= 0 && i < len(execParams) {
				in = execParams[i]
			}
			var dec hooks.Decision
			if r.Status == "success" {
				dec, _ = rt.dispatchTools.hookMgr.PostToolUse(ctx, r.Tool, in, tr)
			} else {
				dec, _ = rt.dispatchTools.hookMgr.PostToolUseFailure(ctx, r.Tool, in, tr)
			}
			if strings.TrimSpace(dec.AdditionalContext) != "" {
				rt.ctxm.AddEphemeral(strings.TrimSpace(dec.AdditionalContext))
			}
			if strings.EqualFold(dec.Decision, "block") && strings.TrimSpace(dec.Reason) != "" {
				rt.ctxm.AddEphemeral("HookBlocked: " + strings.TrimSpace(dec.Reason))
				wantContinue = true
			}
		}
	}
	allResults = append(allResults, srs...)
	successCount := 0
	errorCount := 0
	for _, r := range allResults {
		switch r.Status {
		case "success":
			successCount++
		case "error":
			errorCount++
			slog.Error("runtime.tools_node.tool_error",
				"tool", r.Tool,
				"error", r.Error,
				"display", r.Display)
		}
		if r.Tool == "read" && r.Status == "success" {
			if mode, ok := r.Data["mode"].(string); ok && mode == "file" {
				if v, ok := r.Data["content"].(string); ok {
					rt.ctxm.AddToolFull(v)
				}
			}
		}
		rt.ctxm.AddToolObservation(r)
		if r.Tool == tools.ToolSkill && r.Status == "success" {
			if raw, ok := r.Data["allowed_tools"].([]string); ok {
				ov := buildAllowedToolsMap(raw)
				rt.allowedTools = ov
				if rt.dispatchTools != nil {
					rt.dispatchTools.SetAllowedToolsOverride(ov)
				}
			}
			if fork, ok := r.Data["fork"].(bool); ok && fork {
				if rt.dispatchTools == nil {
					rt.ctxm.AddEphemeral("skill fork error: dispatch tools not initialized")
				} else {
					out, err := runSkillForkFromToolResult(ctx, rt.dispatchTools, r)
					if err != nil {
						rt.ctxm.AddEphemeral("skill fork error: " + err.Error())
					} else {
						if strings.TrimSpace(out) != "" {
							name, _ := r.Data["skill_name"].(string)
							agentName, _ := r.Data["agent"].(string)
							rt.ctxm.AddToolObservation(map[string]any{
								"type":   "skill_fork_result",
								"skill":  strings.TrimSpace(name),
								"agent":  strings.TrimSpace(agentName),
								"result": out,
							})
						}
						wantContinue = true
					}
				}
			}
		}
		if r.Status == "success" && !rt.turnWrapUp.Active {
			if c, ok := r.Data["continue"].(bool); ok && c {
				wantContinue = true
			} else {
				switch r.Tool {
				case "read", "search":
					wantContinue = true
				}
			}
		}
	}
	slog.Debug("runtime.tools_node.execution_summary",
		"prepared_calls", len(prepared),
		"success_count", successCount,
		"error_count", errorCount)
	return results, true, wantContinue
}
