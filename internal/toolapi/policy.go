package toolapi

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import "strings"

type ExecutionModeDescriptor struct {
	Name             string
	Aliases          []string
	Description      string
	ApprovalBehavior string
}

type ToolAccess struct {
	Name          string
	Mode          string
	RiskLevel     RiskLevel
	Visible       bool
	Executable    bool
	NeedsApproval bool
	Reason        string
}

var executionModeDescriptors = []ExecutionModeDescriptor{
	{
		Name:             "plan",
		Aliases:          []string{"计划优先", "先出计划", "plan_first", "plan-first"},
		Description:      "Planning mode: read-only tools stay visible, mutating tools are hidden.",
		ApprovalBehavior: "read_only_only",
	},
	{
		Name: "auto",
		Aliases: []string{
			"default", "manual",
			"acceptEdits", "accept_edits", "accept-edits",
			"dontAsk", "dont_ask", "dont-ask",
			"bypassPermissions", "bypass_permissions", "bypass-permissions", "bypass",
			"默认只读", "手动确认", "接受编辑", "自动无人值守", "拒绝询问", "绕过审批", "内部绕过",
		},
		Description:      "Balanced mode: low/medium risk tools run directly, high risk tools still prompt.",
		ApprovalBehavior: "prompt_high_only",
	},
}

func NormalizeExecutionMode(mode string) string {
	key := normalizeModeKey(mode)
	if key == "" {
		return "auto"
	}
	for _, item := range executionModeDescriptors {
		if key == normalizeModeKey(item.Name) {
			return item.Name
		}
		for _, alias := range item.Aliases {
			if key == normalizeModeKey(alias) {
				return item.Name
			}
		}
	}
	return "auto"
}

func ExecutionModeDescriptorFor(mode string) ExecutionModeDescriptor {
	mode = NormalizeExecutionMode(mode)
	for _, item := range executionModeDescriptors {
		if item.Name == mode {
			out := item
			out.Aliases = append([]string(nil), item.Aliases...)
			return out
		}
	}
	return ExecutionModeDescriptor{Name: "auto"}
}

func SupportedExecutionModes() []ExecutionModeDescriptor {
	out := make([]ExecutionModeDescriptor, 0, len(executionModeDescriptors))
	for _, item := range executionModeDescriptors {
		copied := item
		copied.Aliases = append([]string(nil), item.Aliases...)
		out = append(out, copied)
	}
	return out
}

func IsToolAllowed(allowed map[string]bool, toolName string) bool {
	if allowed == nil {
		return true
	}
	return allowed[strings.ToLower(strings.TrimSpace(toolName))]
}

func EvaluateToolAccess(def ToolDefinition, sess ExecSession) ToolAccess {
	mode := NormalizeExecutionMode(sess.ExecutionMode)
	access := ToolAccess{
		Name:      strings.TrimSpace(def.Name),
		Mode:      mode,
		RiskLevel: def.RiskLevel,
	}

	if strings.TrimSpace(def.Name) == "" {
		access.Reason = "missing_definition"
		return access
	}
	if len(def.VisibleIn) > 0 && !containsMode(def.VisibleIn, mode) {
		access.Reason = "execution_mode"
		return access
	}

	access.Visible = true
	if !def.Invocable {
		access.Reason = "non_invocable"
		return access
	}
	if !IsToolAllowed(sess.AllowedTools, def.Name) {
		access.Reason = "allowed_tools"
		return access
	}
	access.Executable = true

	switch mode {
	case "plan":
		if !def.ReadOnly {
			access.Visible = false
			access.Executable = false
			access.Reason = "execution_mode"
			return access
		}
	case "auto":
		if sess.RequireApprovalDigest {
			access.NeedsApproval = def.RiskLevel != RiskLow
		} else {
			access.NeedsApproval = def.RiskLevel == RiskHigh
		}
	}

	return access
}

func FilterVisibleTools(defs []ToolDefinition, sess ExecSession) []ToolDefinition {
	out := make([]ToolDefinition, 0, len(defs))
	for _, def := range defs {
		access := EvaluateToolAccess(def, sess)
		if access.Visible && access.Executable {
			out = append(out, def)
		}
	}
	return out
}

func FilterVisibleCapabilities(defs []ToolDefinition, sess ExecSession) []ToolDefinition {
	out := make([]ToolDefinition, 0, len(defs))
	for _, def := range defs {
		if EvaluateToolAccess(def, sess).Visible {
			out = append(out, def)
		}
	}
	return out
}

func FindToolDefinition(defs []ToolDefinition, toolName string) (ToolDefinition, bool) {
	target := strings.TrimSpace(toolName)
	for _, def := range defs {
		if strings.EqualFold(strings.TrimSpace(def.Name), target) {
			return def, true
		}
	}
	return ToolDefinition{}, false
}

func containsMode(modes []string, want string) bool {
	for _, mode := range modes {
		if NormalizeExecutionMode(mode) == want {
			return true
		}
	}
	return false
}

func normalizeModeKey(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	mode = strings.ReplaceAll(mode, "_", "")
	mode = strings.ReplaceAll(mode, "-", "")
	mode = strings.ReplaceAll(mode, " ", "")
	return mode
}
