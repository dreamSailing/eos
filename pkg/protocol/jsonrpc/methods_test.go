package jsonrpc

import "testing"

func TestAllCoreMethodsFreezesMigrationSurface(t *testing.T) {
	methods := AllCoreMethods()
	if len(methods) != 137 {
		t.Fatalf("AllCoreMethods() len=%d, want 137", len(methods))
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
		MethodInsightMemorySnapshot,
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
