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
