package jsonrpc

import "testing"

func TestAllCoreMethodsFreezesMigrationSurface(t *testing.T) {
	methods := AllCoreMethods()
	// 146 = 139（历史冻结基线）
	//   + 3 新增内核方法（approval/preview、sandbox/derive_policy、permission/enter_full_access）
	//   + 5 补齐既有遗漏（session/set_meta、session/search、model/bundled_mcp、
	//     workspace/changes、workspace/rollback/{build,apply}）
	//   - 1 移除死方法（insight/memory_snapshot，内核系统 B 清理后已无此路由）
	// 与 generated.CoreMethods()（schema.json 单一真相源）保持一致。
	if len(methods) != 146 {
		t.Fatalf("AllCoreMethods() len=%d, want 146", len(methods))
	}

	seen := make(map[string]bool, len(methods))
	for _, method := range methods {
		if method == "" {
			t.Fatal("AllCoreMethods() contains empty method")
		}
		if seen[method] {
			t.Fatalf("AllCoreMethods() contains duplicate method %q", method)
		}
		seen[method] = true
	}

	required := []string{
		MethodWorkspaceWorktreeCreate,
		MethodSessionDelete,
		MethodMCPList,
		MethodLSPDiagnosticsSummary,
		MethodConfigSettingsSave,
		MethodPermissionApprovalModeSet,
		MethodExtensionsSkillInvoke,
		MethodContextCompact,
		MethodUsageCostItems,
		MethodVersionsRollback,
		MethodTaskTail,
		MethodRuntimeReasoningLevelSet,
		MethodModelActivate,
		MethodModelContext,
		MethodModelWorkspaceSet,
		MethodModelSessionSet,
		MethodRemoteWorkspaceOpen,
		MethodGitShow,
		MethodMemorySnapshot,
		MethodMemoryRecordSearch,
		MethodRoleResolve,
		MethodAgentToolExecute,
		MethodToolStats,
		MethodSandboxSetPolicy,
	}
	for _, method := range required {
		if !seen[method] {
			t.Fatalf("AllCoreMethods() missing %s", method)
		}
	}
}
