package cli

import "github.com/dreamSailing/eos/internal/modes"

type resolvedModeConfig struct {
	AccessMode    string
	ApprovalMode  string
	SandboxMode   string
	SkipAllChecks bool
}

func resolveModeConfig(accessMode string, approvalMode string, sandboxMode string, skipPermissions bool) resolvedModeConfig {
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
		resolvedAccess = modes.NormalizeAccessMode(accessMode)
	}
	resolvedApproval := ""
	if approvalMode != "" {
		resolvedApproval = modes.NormalizeApprovalMode(approvalMode)
	}
	resolvedSandbox := modes.NormalizeSandboxMode(sandboxMode)
	if resolvedSandbox == "" {
		resolvedSandbox = modes.SandboxModeFromAccessMode(modes.ResolveAccessMode(modes.ExecSession{SandboxMode: sandboxMode}))
	}
	if resolvedAccess != "" {
		resolvedSandbox = modes.SandboxModeFromAccessMode(resolvedAccess)
	}

	return resolvedModeConfig{
		AccessMode:   resolvedAccess,
		ApprovalMode: resolvedApproval,
		SandboxMode:  resolvedSandbox,
	}
}
