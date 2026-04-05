package toolapi

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
		Name:             "default",
		Aliases:          []string{"manual"},
		Description:      "Conservative mode: read-only tools run directly, mutating tools require approval.",
		ApprovalBehavior: "prompt_mutations",
	},
	{
		Name:             "acceptEdits",
		Aliases:          []string{"accept_edits", "accept-edits"},
		Description:      "Auto-accepts filesystem edits and file operations; other side-effectful tools still require approval.",
		ApprovalBehavior: "auto_accept_filesystem",
	},
	{
		Name:             "plan",
		Description:      "Planning mode: read-only tools stay visible, mutating tools are hidden.",
		ApprovalBehavior: "read_only_only",
	},
	{
		Name:             "auto",
		Description:      "Balanced mode: low/medium risk tools run directly, high risk tools still prompt.",
		ApprovalBehavior: "prompt_high_only",
	},
	{
		Name:             "dontAsk",
		Aliases:          []string{"dont_ask", "dont-ask"},
		Description:      "Locked mode: only explicitly allowed tools can execute; everything else is denied without prompting.",
		ApprovalBehavior: "deny_unapproved",
	},
	{
		Name:             "bypassPermissions",
		Aliases:          []string{"bypass", "bypass_permissions", "bypass-permissions"},
		Description:      "Unsafe mode: skips approval prompts and leaves only registry/policy filtering in place.",
		ApprovalBehavior: "no_prompts",
	},
}

func NormalizeExecutionMode(mode string) string {
	key := normalizeModeKey(mode)
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
	return "default"
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
	return ExecutionModeDescriptor{Name: "default"}
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
	if mode == "dontAsk" && !isExplicitToolAllowed(sess.AllowedTools, def.Name) {
		access.Reason = "dont_ask"
		return access
	}
	if mode != "dontAsk" && !IsToolAllowed(sess.AllowedTools, def.Name) {
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
	case "default":
		access.NeedsApproval = !def.ReadOnly
	case "acceptEdits":
		access.NeedsApproval = !def.ReadOnly && !acceptEditsAutoApproved(def)
	case "auto":
		if sess.RequireApprovalDigest {
			access.NeedsApproval = def.RiskLevel != RiskLow
		} else {
			access.NeedsApproval = def.RiskLevel == RiskHigh
		}
	case "dontAsk":
		access.NeedsApproval = false
	case "bypassPermissions":
		access.NeedsApproval = false
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

func isExplicitToolAllowed(allowed map[string]bool, toolName string) bool {
	if allowed == nil {
		return false
	}
	return allowed[strings.ToLower(strings.TrimSpace(toolName))]
}

func acceptEditsAutoApproved(def ToolDefinition) bool {
	return strings.EqualFold(strings.TrimSpace(def.Category), "filesystem") &&
		!def.ReadOnly &&
		def.RiskLevel != RiskHigh
}
