package cli

import "github.com/dreamSailing/eos/internal/toolapi"

type resolvedModeConfig struct {
	AccessMode    string
	ApprovalMode  string
	SandboxMode   string
	SkipAllChecks bool
}

func resolveModeConfig(accessMode string, approvalMode string, sandboxMode string, skipPermissions bool, requireApprovalDigest bool) resolvedModeConfig {
	if skipPermissions {
		return resolvedModeConfig{
			AccessMode:    "danger-full-access",
			ApprovalMode:  "never",
			SandboxMode:   "full_access",
			SkipAllChecks: true,
		}
	}

	resolvedAccess := ""
	if accessMode != "" {
		resolvedAccess = toolapi.NormalizeAccessMode(accessMode)
	}
	resolvedApproval := ""
	if approvalMode != "" {
		resolvedApproval = toolapi.NormalizeApprovalMode(approvalMode)
	}
	resolvedSandbox := toolapi.NormalizeSandboxMode(sandboxMode)
	if resolvedSandbox == "" {
		resolvedSandbox = toolapi.SandboxModeFromAccessMode(toolapi.ResolveAccessMode(toolapi.ExecSession{SandboxMode: sandboxMode}))
	}
	if resolvedAccess != "" {
		resolvedSandbox = toolapi.SandboxModeFromAccessMode(resolvedAccess)
	}

	return resolvedModeConfig{
		AccessMode:   resolvedAccess,
		ApprovalMode: resolvedApproval,
		SandboxMode:  resolvedSandbox,
	}
}
