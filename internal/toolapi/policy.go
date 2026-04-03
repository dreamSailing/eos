package toolapi

import "strings"

type ToolAccess struct {
	Name          string
	Mode          string
	RiskLevel     RiskLevel
	Visible       bool
	Executable    bool
	NeedsApproval bool
	Reason        string
}

func NormalizeExecutionMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "manual", "plan", "auto", "bypass":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return "auto"
	}
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
	if def.Invocable && !IsToolAllowed(sess.AllowedTools, def.Name) {
		access.Reason = "allowed_tools"
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
	access.Executable = true

	switch mode {
	case "plan":
		if def.RiskLevel != RiskLow {
			access.Visible = false
			access.Executable = false
			access.Reason = "execution_mode"
			return access
		}
	case "manual":
		access.NeedsApproval = def.RiskLevel != RiskLow
	case "auto":
		access.NeedsApproval = sess.RequireApprovalDigest && def.RiskLevel != RiskLow
	case "bypass":
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
