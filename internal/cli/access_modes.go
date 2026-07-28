package cli

import "github.com/dreamSailing/eos/internal/modes"

type resolvedModeConfig struct {
	AccessMode    string
	ApprovalMode  string
	SandboxMode   string
	SkipAllChecks bool
}

// resolveModeConfig 把用户传入的访问/审批/沙箱模式解析成启动期 env 值。
//
// skipPermissions 分支不再在壳层合成 danger-full-access + never + full_access
// 三件套——双轴协同（ApprovalMode=Never + SandboxMode=DangerFullAccess）现在
// 由内核 bin 侧读 EOS_SKIP_PERMISSIONS 后用 permission_enter_full_access 单一
// 真相源派生（AGENTS.md §3：壳层不做业务裁决）。壳层只透传 skip 标志。
func resolveModeConfig(accessMode string, approvalMode string, sandboxMode string, skipPermissions bool) resolvedModeConfig {
	if skipPermissions {
		// 只透传 skip 标志，不合成 mode 值。execOptionEnv 会把 EOS_SKIP_PERMISSIONS=1
		// 传给内核，内核启动期原子地设双轴；显式 mode 与 skip 共存时内核会 fail-fast。
		return resolvedModeConfig{
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
