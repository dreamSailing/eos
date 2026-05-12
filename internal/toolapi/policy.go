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

type ToolAccess struct {
	Name           string
	Mode           string
	AccessMode     string
	ApprovalMode   string
	ApprovalSource string
	SandboxMode    string
	RiskLevel      RiskLevel
	Visible        bool
	Executable     bool
	NeedsApproval  bool
	Reason         string
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
			"自动", "auto_mode", "auto-mode",
		},
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

func AccessModeDescriptorFor(mode string) AccessModeDescriptor {
	mode = NormalizeAccessMode(mode)
	for _, item := range accessModeDescriptors {
		if item.Name == mode {
			out := item
			out.Aliases = append([]string(nil), item.Aliases...)
			return out
		}
	}
	return AccessModeDescriptor{Name: "workspace-write"}
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

func ApprovalModeDescriptorFor(mode string) ApprovalModeDescriptor {
	mode = NormalizeApprovalMode(mode)
	for _, item := range approvalModeDescriptors {
		if item.Name == mode {
			out := item
			out.Aliases = append([]string(nil), item.Aliases...)
			return out
		}
	}
	return ApprovalModeDescriptor{Name: "on-request"}
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

func SandboxModeDescriptorFor(mode string) SandboxModeDescriptor {
	mode = NormalizeSandboxMode(mode)
	for _, item := range sandboxModeDescriptors {
		if item.Name == mode {
			out := item
			out.Aliases = append([]string(nil), item.Aliases...)
			return out
		}
	}
	return SandboxModeDescriptor{Name: "workspace"}
}

func SupportedSandboxModes() []SandboxModeDescriptor {
	out := make([]SandboxModeDescriptor, 0, len(sandboxModeDescriptors))
	for _, item := range sandboxModeDescriptors {
		copied := item
		copied.Aliases = append([]string(nil), item.Aliases...)
		out = append(out, copied)
	}
	return out
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

func ResolveAccessMode(sess ExecSession) string {
	if normalized := NormalizeAccessMode(sess.AccessMode); strings.TrimSpace(sess.AccessMode) != "" {
		return normalized
	}
	if NormalizeSandboxMode(sess.SandboxMode) == "full_access" {
		return "danger-full-access"
	}
	return "workspace-write"
}

func ResolveApprovalMode(sess ExecSession) string {
	if normalized := NormalizeApprovalMode(sess.ApprovalMode); strings.TrimSpace(sess.ApprovalMode) != "" {
		return normalized
	}
	if sess.RequireApprovalDigest {
		return "on-request"
	}
	return "on-failure"
}

func SandboxModeFromAccessMode(mode string) string {
	switch NormalizeAccessMode(mode) {
	case "danger-full-access":
		return "full_access"
	default:
		return "workspace"
	}
}

func EvaluateToolAccess(def ToolDefinition, sess ExecSession) ToolAccess {
	mode := NormalizeExecutionMode(sess.ExecutionMode)
	accessMode := ResolveAccessMode(sess)
	approvalMode, approvalSource := ResolveToolApprovalMode(def, sess)
	sandboxMode := SandboxModeFromAccessMode(accessMode)
	access := ToolAccess{
		Name:           strings.TrimSpace(def.Name),
		Mode:           mode,
		AccessMode:     accessMode,
		ApprovalMode:   approvalMode,
		ApprovalSource: approvalSource,
		SandboxMode:    sandboxMode,
		RiskLevel:      def.RiskLevel,
	}

	if strings.TrimSpace(def.Name) == "" {
		access.Reason = "missing_definition"
		return access
	}
	if accessMode == "read-only" && !def.ReadOnly {
		access.Reason = "access_mode"
		return access
	}
	if def.RequiresFullAccess && accessMode != "danger-full-access" {
		if strings.TrimSpace(sess.AccessMode) != "" {
			access.Reason = "access_mode"
		} else {
			access.Reason = "sandbox_mode"
		}
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
		if strings.TrimSpace(sess.ApprovalMode) != "" {
			switch approvalMode {
			case "untrusted":
				access.NeedsApproval = !def.ReadOnly || def.RiskLevel != RiskLow
			case "on-failure":
				access.NeedsApproval = false
			case "on-request":
				access.NeedsApproval = def.RiskLevel == RiskHigh
			case "never":
				access.NeedsApproval = false
			}
		} else {
			if sess.RequireApprovalDigest {
				access.NeedsApproval = def.RiskLevel != RiskLow
			} else {
				access.NeedsApproval = def.RiskLevel == RiskHigh
			}
		}
	}

	return access
}

func ResolveToolApprovalMode(def ToolDefinition, sess ExecSession) (string, string) {
	mode := ResolveApprovalMode(sess)
	source := "session_default"
	if strings.TrimSpace(sess.ApprovalMode) != "" {
		source = "session_explicit"
	}
	if def.Metadata == nil {
		return mode, source
	}
	if toolMode := resolveToolApprovalOverride(def); strings.TrimSpace(toolMode) != "" {
		return NormalizeApprovalMode(toolMode), "tool_override"
	}
	if serviceMode := NormalizeApprovalMode(metadataString(def.Metadata, "approval_mode")); strings.TrimSpace(metadataString(def.Metadata, "approval_mode")) != "" {
		return serviceMode, "service_default"
	}
	return mode, source
}

func resolveToolApprovalOverride(def ToolDefinition) string {
	if def.Metadata == nil {
		return ""
	}
	raw, ok := def.Metadata["tool_approval_override"]
	if !ok {
		return ""
	}
	defName := strings.TrimSpace(def.Name)
	switch v := raw.(type) {
	case map[string]string:
		for name, mode := range v {
			if strings.EqualFold(strings.TrimSpace(name), defName) {
				return strings.TrimSpace(mode)
			}
		}
	case map[string]any:
		for name, value := range v {
			mode, _ := value.(string)
			if strings.EqualFold(strings.TrimSpace(name), defName) {
				return strings.TrimSpace(mode)
			}
		}
	}
	return ""
}

func metadataString(meta map[string]any, key string) string {
	if len(meta) == 0 {
		return ""
	}
	value, _ := meta[key].(string)
	return strings.TrimSpace(value)
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
