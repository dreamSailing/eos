package jsonrpc

import "testing"

func TestAllCoreMethodsFreezesMigrationSurface(t *testing.T) {
	methods := AllCoreMethods()
	// 149 = 139（历史冻结基线）
	//   + 3 新增内核方法（approval/preview、sandbox/derive_policy、permission/enter_full_access）
	//   + 5 补齐既有遗漏（session/set_meta、session/search、model/bundled_mcp、
	//     workspace/changes、workspace/rollback/{build,apply}）
	//   - 1 移除死方法（insight/memory_snapshot，内核系统 B 清理后已无此路由）
	// 与 generated.CoreMethods()（schema.json 单一真相源）保持一致。
	//   + 14 浏览器方法（+upload_provide/preview_start/preview_stop/focus/set_default_profile/navigate）
	//   + 4 内嵌实时视口方法（live_start/live_stop/input/history）
	//   - 2 移除死方法（preview_start/preview_stop——live 实时流取代）
	//   + 1 git/summary（工作区未提交/未推送提示，status -sb 单命令聚合）
	if len(methods) != 170 {
		t.Fatalf("AllCoreMethods() len=%d, want 170", len(methods))
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
