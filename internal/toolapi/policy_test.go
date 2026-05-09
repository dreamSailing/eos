package toolapi

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import "testing"

var testAllModes = []string{"auto", "plan"}

func TestFilterVisibleTools_PlanModeOnlyKeepsLowRisk(t *testing.T) {
	defs := []ToolDefinition{
		{Name: "read", RiskLevel: RiskLow, ReadOnly: true, VisibleIn: testAllModes, Invocable: true},
		{Name: "bash", RiskLevel: RiskHigh, VisibleIn: []string{"auto"}, Invocable: true},
	}

	got := FilterVisibleTools(defs, ExecSession{
		AllowedTools:  map[string]bool{"read": true, "bash": true},
		ExecutionMode: "plan",
	})

	if len(got) != 1 || got[0].Name != "read" {
		t.Fatalf("visible tools=%v, want only read", got)
	}
}

func TestEvaluateToolAccess_AutoModeAllowsMediumRiskWithoutApprovalByDefault(t *testing.T) {
	access := EvaluateToolAccess(ToolDefinition{
		Name:      "edit",
		Category:  "filesystem",
		RiskLevel: RiskMedium,
		VisibleIn: testAllModes,
		Invocable: true,
	}, ExecSession{
		AllowedTools:  map[string]bool{"edit": true},
		ExecutionMode: "auto",
	})

	if access.Mode != "auto" {
		t.Fatalf("mode=%q, want auto", access.Mode)
	}
	if !access.Visible || !access.Executable {
		t.Fatalf("auto mode should allow edit, got %+v", access)
	}
	if access.NeedsApproval {
		t.Fatalf("auto mode should not require approval here, got %+v", access)
	}
}

func TestEvaluateToolAccess_AutoModeRequiresDigestForMediumRiskWhenEnabled(t *testing.T) {
	def := ToolDefinition{Name: "edit", Category: "filesystem", RiskLevel: RiskMedium, VisibleIn: testAllModes, Invocable: true}

	access := EvaluateToolAccess(def, ExecSession{
		AllowedTools:          map[string]bool{"edit": true},
		ExecutionMode:         "auto",
		RequireApprovalDigest: true,
	})
	if !access.NeedsApproval {
		t.Fatalf("auto mode with digest approval should require confirmation, got %+v", access)
	}
}

func TestEvaluateToolAccess_AutoModeOnlyPromptsHighRiskByDefault(t *testing.T) {
	medium := ToolDefinition{Name: "edit", RiskLevel: RiskMedium, Category: "filesystem", VisibleIn: testAllModes, Invocable: true}
	high := ToolDefinition{Name: "bash", RiskLevel: RiskHigh, VisibleIn: testAllModes, Invocable: true}

	mediumAccess := EvaluateToolAccess(medium, ExecSession{
		AllowedTools:  map[string]bool{"edit": true},
		ExecutionMode: "auto",
	})
	if mediumAccess.NeedsApproval {
		t.Fatalf("auto mode should auto-allow medium risk by default, got %+v", mediumAccess)
	}

	highAccess := EvaluateToolAccess(high, ExecSession{
		AllowedTools:  map[string]bool{"bash": true},
		ExecutionMode: "auto",
	})
	if !highAccess.NeedsApproval {
		t.Fatalf("auto mode should still prompt for high risk, got %+v", highAccess)
	}
}

func TestEvaluateToolAccess_PlanRejectsMutatingTools(t *testing.T) {
	def := ToolDefinition{Name: "bash", RiskLevel: RiskHigh, VisibleIn: []string{"auto"}, Invocable: true}

	access := EvaluateToolAccess(def, ExecSession{
		AllowedTools:  map[string]bool{"bash": true},
		ExecutionMode: "计划优先",
	})

	if access.Mode != "plan" {
		t.Fatalf("mode=%q, want plan", access.Mode)
	}
	if access.Visible || access.Executable {
		t.Fatalf("plan should hide mutating tools, got %+v", access)
	}
	if access.Reason != "execution_mode" {
		t.Fatalf("reason=%q, want execution_mode", access.Reason)
	}
}

func TestEvaluateToolAccess_PlanAllowsReadOnlyTools(t *testing.T) {
	def := ToolDefinition{Name: "read", RiskLevel: RiskLow, ReadOnly: true, VisibleIn: testAllModes, Invocable: true}

	access := EvaluateToolAccess(def, ExecSession{
		AllowedTools:  map[string]bool{"read": true},
		ExecutionMode: "plan",
	})

	if !access.Visible || !access.Executable {
		t.Fatalf("plan should allow readonly tools, got %+v", access)
	}
	if access.NeedsApproval {
		t.Fatalf("plan readonly tools should not need approval, got %+v", access)
	}
}

func TestNormalizeExecutionModeAcceptsCurrentModesOnly(t *testing.T) {
	tests := map[string]string{
		"auto":     "auto",
		"自动":       "auto",
		"plan":     "plan",
		"计划优先":     "plan",
		"先出计划":     "plan",
		"unknown":  "auto",
	}

	for input, want := range tests {
		if got := NormalizeExecutionMode(input); got != want {
			t.Fatalf("NormalizeExecutionMode(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestExecutionModeDescriptorForReturnsAliases(t *testing.T) {
	desc := ExecutionModeDescriptorFor("auto")
	if desc.Name != "auto" {
		t.Fatalf("Name=%q, want auto", desc.Name)
	}
	found := false
	for _, alias := range desc.Aliases {
		if alias == "auto-mode" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Aliases=%v, want auto-mode alias", desc.Aliases)
	}
}

func TestEvaluateToolAccess_CapabilityOnlyIsVisibleButNotExecutable(t *testing.T) {
	def := ToolDefinition{
		Name:      "skill:review",
		RiskLevel: RiskLow,
		ReadOnly:  true,
		VisibleIn: testAllModes,
	}

	access := EvaluateToolAccess(def, ExecSession{
		AllowedTools:  map[string]bool{"read": true},
		ExecutionMode: "auto",
	})

	if !access.Visible {
		t.Fatalf("capability-only entry should remain visible, got %+v", access)
	}
	if access.Executable {
		t.Fatalf("capability-only entry should not be executable, got %+v", access)
	}
}
