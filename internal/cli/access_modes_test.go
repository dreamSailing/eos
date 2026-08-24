package cli

// access_modes_test.go 守护 resolveModeConfig 的 skipPermissions 分支：
// 壳层不再合成 danger-full-access + never + full_access 三件套——双轴协同
// 现在由内核 bin 侧读 EOS_SKIP_PERMISSIONS 后用 permission_enter_full_access
// 单一真相源派生（AGENTS.md §3：壳层不做业务裁决）。skip=true 时壳层只透传
// 标志，不产出任何 mode 值。

import "testing"

func TestResolveModeConfigSkipPermissionsDoesNotSynthesizeModes(t *testing.T) {
	cfg := resolveModeConfig("", "", "", true)
	if !cfg.SkipAllChecks {
		t.Fatalf("SkipAllChecks must be true when skipPermissions is set: %+v", cfg)
	}
	// 关键：壳层不再合成 mode 值。任一字段非空就意味着壳层又在做业务裁决。
	if cfg.AccessMode != "" {
		t.Errorf("AccessMode must be empty (kernel derives it): got %q", cfg.AccessMode)
	}
	if cfg.ApprovalMode != "" {
		t.Errorf("ApprovalMode must be empty (kernel derives it): got %q", cfg.ApprovalMode)
	}
	if cfg.SandboxMode != "" {
		t.Errorf("SandboxMode must be empty (kernel derives it): got %q", cfg.SandboxMode)
	}
}

func TestResolveModeConfigNormalizesExplicitModes(t *testing.T) {
	// 非 skip 路径仍按用户显式输入归一化（壳层的合法职责：flag 解析）。
	cfg := resolveModeConfig("workspace-write", "never", "workspace", false)
	if cfg.AccessMode != "workspace-write" {
		t.Errorf("AccessMode: got %q want workspace-write", cfg.AccessMode)
	}
	if cfg.ApprovalMode != "never" {
		t.Errorf("ApprovalMode: got %q want never", cfg.ApprovalMode)
	}
	if cfg.SkipAllChecks {
		t.Errorf("SkipAllChecks must be false without skipPermissions")
	}
}

// TestResolveModeConfigSandboxAliasesFoldToKernelVocabulary 守护沙箱别名表：
// --sandbox-mode danger-full-access / full_access 不再被静默降级为 workspace
// （旧 NormalizeSandboxMode 只认 GUI 双值），并折叠到内核 kebab-case 规范值。
func TestResolveModeConfigSandboxAliasesFoldToKernelVocabulary(t *testing.T) {
	cases := []struct {
		name        string
		accessMode  string
		sandboxMode string
		want        string
	}{
		{name: "canonical danger via sandbox alias", sandboxMode: "danger-full-access", want: "danger-full-access"},
		{name: "legacy full_access folds to danger", sandboxMode: "full_access", want: "danger-full-access"},
		{name: "legacy workspace folds to workspace-write", sandboxMode: "workspace", want: "workspace-write"},
		{name: "read-only via access flag", accessMode: "read-only", want: "read-only"},
		{name: "explicit access flag wins over sandbox alias", accessMode: "read-only", sandboxMode: "full_access", want: "read-only"},
		{name: "default falls back to workspace-write", want: "workspace-write"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := resolveModeConfig(tc.accessMode, "", tc.sandboxMode, false)
			if cfg.SandboxMode != tc.want {
				t.Fatalf("SandboxMode: got %q want %q", cfg.SandboxMode, tc.want)
			}
			if cfg.AccessMode != tc.want {
				t.Fatalf("AccessMode: got %q want %q (axes share the kernel vocabulary)", cfg.AccessMode, tc.want)
			}
		})
	}
}

// TestResolveModeConfigFoldsOnFailureApprovalAlias 内核 ApprovalMode 只有
// untrusted/on-request/never 三值；on-failure 是解析侧历史别名，CLI 侧折叠。
func TestResolveModeConfigFoldsOnFailureApprovalAlias(t *testing.T) {
	cfg := resolveModeConfig("", "on-failure", "", false)
	if cfg.ApprovalMode != "on-request" {
		t.Fatalf("ApprovalMode: got %q want on-request (on-failure folds to on-request)", cfg.ApprovalMode)
	}
}
