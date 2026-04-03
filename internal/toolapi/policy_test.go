package toolapi

import "testing"

func TestFilterVisibleTools_PlanModeOnlyKeepsLowRisk(t *testing.T) {
	defs := []ToolDefinition{
		{Name: "read", RiskLevel: RiskLow, VisibleIn: []string{"manual", "plan", "auto", "bypass"}, Invocable: true},
		{Name: "bash", RiskLevel: RiskHigh, VisibleIn: []string{"manual", "auto", "bypass"}, Invocable: true},
	}

	got := FilterVisibleTools(defs, ExecSession{
		AllowedTools:  map[string]bool{"read": true, "bash": true},
		ExecutionMode: "plan",
	})

	if len(got) != 1 || got[0].Name != "read" {
		t.Fatalf("visible tools=%v, want only read", got)
	}
}

func TestEvaluateToolAccess_ManualModeRequiresApproval(t *testing.T) {
	def := ToolDefinition{Name: "edit", RiskLevel: RiskMedium, VisibleIn: []string{"manual", "auto", "bypass"}, Invocable: true}

	access := EvaluateToolAccess(def, ExecSession{
		AllowedTools:  map[string]bool{"edit": true},
		ExecutionMode: "manual",
	})

	if !access.Visible || !access.Executable {
		t.Fatalf("manual mode should allow edit, got %+v", access)
	}
	if !access.NeedsApproval {
		t.Fatalf("manual mode should require approval for medium risk, got %+v", access)
	}
}

func TestEvaluateToolAccess_BypassSkipsApproval(t *testing.T) {
	def := ToolDefinition{Name: "bash", RiskLevel: RiskHigh, VisibleIn: []string{"manual", "auto", "bypass"}, Invocable: true}

	access := EvaluateToolAccess(def, ExecSession{
		AllowedTools:          map[string]bool{"bash": true},
		ExecutionMode:         "bypass",
		RequireApprovalDigest: true,
	})

	if !access.Visible || !access.Executable {
		t.Fatalf("bypass mode should allow bash, got %+v", access)
	}
	if access.NeedsApproval {
		t.Fatalf("bypass mode should skip approval, got %+v", access)
	}
}

func TestEvaluateToolAccess_CapabilityOnlyIsVisibleButNotExecutable(t *testing.T) {
	def := ToolDefinition{
		Name:      "skill:review",
		RiskLevel: RiskLow,
		VisibleIn: []string{"manual", "plan", "auto", "bypass"},
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
