package modes

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

// modes.go holds the execution/access/approval/sandbox mode normalization
// helpers previously provided by internal/toolapi.policy.go. They moved here
// when the Go gateway layer (internal/toolapi) was removed; the Go TUI and the
// --print headless path are the only remaining consumers.

import "strings"

// ExecSession mirrors the subset of toolapi.ExecSession that the mode helpers
// consult. Kept local so ui has no dependency on the deleted toolapi package.
type ExecSession struct {
	WorkspaceRoot         string
	AllowedTools          map[string]bool
	TraceID               string
	ExecutionMode         string
	AccessMode            string
	ApprovalMode          string
	SandboxMode           string
	RequireApprovalDigest bool
}

type ExecutionModeDescriptor struct {
	Name             string
	Aliases          []string
	Description      string
	ApprovalBehavior string
}

type AccessModeDescriptor struct {
	Name        string
	Aliases     []string
	Description string
}

type ApprovalModeDescriptor struct {
	Name        string
	Aliases     []string
	Description string
}

type SandboxModeDescriptor struct {
	Name        string
	Aliases     []string
	Description string
}

var executionModeDescriptors = []ExecutionModeDescriptor{
	{
		Name:             "plan",
		Aliases:          []string{"计划优先", "先出计划", "plan_first", "plan-first"},
		Description:      "Planning mode: read-only tools stay visible, mutating tools are hidden.",
		ApprovalBehavior: "read_only_only",
	},
	{
		Name:             "auto",
		Aliases:          []string{"自动", "auto_mode", "auto-mode"},
		Description:      "Balanced mode: low/medium risk tools run directly, high risk tools still prompt.",
		ApprovalBehavior: "prompt_high_only",
	},
}

var accessModeDescriptors = []AccessModeDescriptor{
	{
		Name:        "read-only",
		Aliases:     []string{"readonly", "read_only", "只读"},
		Description: "Read-only: blocks mutating tools and side effects.",
	},
	{
		Name:        "workspace-write",
		Aliases:     []string{"workspace_write", "workspace-write", "workspace", "sandbox", "workspace_sandbox", "workspace-sandbox", "工作区写入"},
		Description: "Workspace-write: allows writes inside the workspace boundary.",
	},
	{
		Name:        "danger-full-access",
		Aliases:     []string{"danger_full_access", "danger-full-access", "full_access", "full-access", "fullaccess", "allow_all", "allow-all", "完全访问权限"},
		Description: "Danger-full-access: removes the workspace boundary for privileged tools.",
	},
}

var approvalModeDescriptors = []ApprovalModeDescriptor{
	{
		Name:        "untrusted",
		Aliases:     []string{"cautious", "strict", "不信任"},
		Description: "Always ask before non-read-only or elevated actions.",
	},
	{
		Name:        "on-failure",
		Aliases:     []string{"on_failure", "onfailure", "失败后审批"},
		Description: "Run within the current sandbox first and escalate only after sandbox failures.",
	},
	{
		Name:        "on-request",
		Aliases:     []string{"on_request", "onrequest", "request", "请求时审批"},
		Description: "Allow the agent to request approval when it decides escalation is needed.",
	},
	{
		Name:        "never",
		Aliases:     []string{"no_approval", "no-approval", "skip_approval", "skip-approval", "从不审批"},
		Description: "Never ask for approval.",
	},
}

var sandboxModeDescriptors = []SandboxModeDescriptor{
	{
		Name:        "workspace",
		Aliases:     []string{"sandbox", "workspace_sandbox", "workspace-sandbox", "工作区沙箱"},
		Description: "Workspace sandbox: only tools constrained to the workspace stay visible.",
	},
	{
		Name:        "full_access",
		Aliases:     []string{"full-access", "fullaccess", "allow_all", "allow-all", "完全访问权限"},
		Description: "Full access: tools may cross the workspace boundary subject to approvals.",
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

func SupportedExecutionModes() []ExecutionModeDescriptor {
	out := make([]ExecutionModeDescriptor, 0, len(executionModeDescriptors))
	for _, item := range executionModeDescriptors {
		copied := item
		copied.Aliases = append([]string(nil), item.Aliases...)
		out = append(out, copied)
	}
	return out
}

func NormalizeAccessMode(mode string) string {
	key := normalizeModeKey(mode)
	if key == "" {
		return "workspace-write"
	}
	for _, item := range accessModeDescriptors {
		if key == normalizeModeKey(item.Name) {
			return item.Name
		}
		for _, alias := range item.Aliases {
			if key == normalizeModeKey(alias) {
				return item.Name
			}
		}
	}
	return "workspace-write"
}

func SupportedAccessModes() []AccessModeDescriptor {
	out := make([]AccessModeDescriptor, 0, len(accessModeDescriptors))
	for _, item := range accessModeDescriptors {
		copied := item
		copied.Aliases = append([]string(nil), item.Aliases...)
		out = append(out, copied)
	}
	return out
}

func NormalizeApprovalMode(mode string) string {
	key := normalizeModeKey(mode)
	if key == "" {
		return "on-request"
	}
	for _, item := range approvalModeDescriptors {
		if key == normalizeModeKey(item.Name) {
			return item.Name
		}
		for _, alias := range item.Aliases {
			if key == normalizeModeKey(alias) {
				return item.Name
			}
		}
	}
	return "on-request"
}

func SupportedApprovalModes() []ApprovalModeDescriptor {
	out := make([]ApprovalModeDescriptor, 0, len(approvalModeDescriptors))
	for _, item := range approvalModeDescriptors {
		copied := item
		copied.Aliases = append([]string(nil), item.Aliases...)
		out = append(out, copied)
	}
	return out
}

func NormalizeSandboxMode(mode string) string {
	key := normalizeModeKey(mode)
	if key == "" {
		return "workspace"
	}
	for _, item := range sandboxModeDescriptors {
		if key == normalizeModeKey(item.Name) {
			return item.Name
		}
		for _, alias := range item.Aliases {
			if key == normalizeModeKey(alias) {
				return item.Name
			}
		}
	}
	return "workspace"
}

func SandboxModeFromAccessMode(mode string) string {
	switch NormalizeAccessMode(mode) {
	case "danger-full-access":
		return "full_access"
	default:
		return "workspace"
	}
}

func ResolveAccessMode(sess ExecSession) string {
	if normalized := NormalizeAccessMode(sess.AccessMode); strings.TrimSpace(sess.AccessMode) != "" {
		return normalized
	}
	if NormalizeSandboxMode(sess.SandboxMode) == "full_access" {
		return "danger-full-access"
	}
	return "workspace-write"
}

func normalizeModeKey(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	mode = strings.ReplaceAll(mode, "_", "")
	mode = strings.ReplaceAll(mode, "-", "")
	mode = strings.ReplaceAll(mode, " ", "")
	return mode
}
