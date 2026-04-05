package toolapi

import "testing"

var testAllModes = []string{"default", "acceptEdits", "plan", "auto", "dontAsk", "bypassPermissions"}

func TestFilterVisibleTools_PlanModeOnlyKeepsLowRisk(t *testing.T) {
	defs := []ToolDefinition{
		{Name: "read", RiskLevel: RiskLow, ReadOnly: true, VisibleIn: testAllModes, Invocable: true},
		{Name: "bash", RiskLevel: RiskHigh, VisibleIn: []string{"default", "acceptEdits", "auto", "dontAsk", "bypassPermissions"}, Invocable: true},
	}

	got := FilterVisibleTools(defs, ExecSession{
		AllowedTools:  map[string]bool{"read": true, "bash": true},
		ExecutionMode: "plan",
	})

	if len(got) != 1 || got[0].Name != "read" {
		t.Fatalf("visible tools=%v, want only read", got)
	}
}

func TestEvaluateToolAccess_DefaultModeRequiresApproval(t *testing.T) {
	access := EvaluateToolAccess(ToolDefinition{
		Name:      "edit",
		Category:  "filesystem",
		RiskLevel: RiskMedium,
		VisibleIn: testAllModes,
		Invocable: true,
	}, ExecSession{
		AllowedTools:  map[string]bool{"edit": true},
		ExecutionMode: "default",
	})

	if !access.Visible || !access.Executable {
		t.Fatalf("default mode should allow edit, got %+v", access)
	}
	if !access.NeedsApproval {
		t.Fatalf("default mode should require approval for medium risk, got %+v", access)
	}
}

func TestEvaluateToolAccess_AcceptEditsAutoApprovesFilesystemMutations(t *testing.T) {
	def := ToolDefinition{Name: "edit", Category: "filesystem", RiskLevel: RiskMedium, VisibleIn: testAllModes, Invocable: true}

	access := EvaluateToolAccess(def, ExecSession{
		AllowedTools:          map[string]bool{"edit": true},
		ExecutionMode:         "acceptEdits",
		RequireApprovalDigest: true,
	})

	if !access.Visible || !access.Executable {
		t.Fatalf("acceptEdits mode should allow edit, got %+v", access)
	}
	if access.NeedsApproval {
		t.Fatalf("acceptEdits mode should auto-approve filesystem edits, got %+v", access)
	}
}

func TestEvaluateToolAccess_AcceptEditsStillPromptsNonFilesystemMutations(t *testing.T) {
	def := ToolDefinition{Name: "git_commit", Category: "git", RiskLevel: RiskMedium, VisibleIn: testAllModes, Invocable: true}

	access := EvaluateToolAccess(def, ExecSession{
		AllowedTools:  map[string]bool{"git_commit": true},
		ExecutionMode: "acceptEdits",
	})
	if !access.NeedsApproval {
		t.Fatalf("acceptEdits should still prompt non-filesystem mutations, got %+v", access)
	}
}

func TestEvaluateToolAccess_BypassSkipsApproval(t *testing.T) {
	def := ToolDefinition{Name: "bash", RiskLevel: RiskHigh, VisibleIn: testAllModes, Invocable: true}

	access := EvaluateToolAccess(def, ExecSession{
		AllowedTools:          map[string]bool{"bash": true},
		ExecutionMode:         "bypassPermissions",
		RequireApprovalDigest: true,
	})

	if !access.Visible || !access.Executable {
		t.Fatalf("bypassPermissions mode should allow bash, got %+v", access)
	}
	if access.NeedsApproval {
		t.Fatalf("bypassPermissions mode should skip approval, got %+v", access)
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

func TestEvaluateToolAccess_DontAskDeniesUnapprovedTools(t *testing.T) {
	def := ToolDefinition{Name: "bash", RiskLevel: RiskHigh, VisibleIn: testAllModes, Invocable: true}

	access := EvaluateToolAccess(def, ExecSession{
		AllowedTools:  map[string]bool{"read": true},
		ExecutionMode: "dontAsk",
	})

	if !access.Visible {
		t.Fatalf("dontAsk should keep tool visible in catalog, got %+v", access)
	}
	if access.Executable {
		t.Fatalf("dontAsk should deny unapproved tools, got %+v", access)
	}
	if access.Reason != "dont_ask" {
		t.Fatalf("dontAsk denial reason=%q, want dont_ask", access.Reason)
	}
}

func TestEvaluateToolAccess_DontAskAllowsExplicitTools(t *testing.T) {
	def := ToolDefinition{Name: "read", RiskLevel: RiskLow, ReadOnly: true, VisibleIn: testAllModes, Invocable: true}

	access := EvaluateToolAccess(def, ExecSession{
		AllowedTools:  map[string]bool{"read": true},
		ExecutionMode: "dontAsk",
	})

	if !access.Visible || !access.Executable {
		t.Fatalf("dontAsk should allow explicitly approved tools, got %+v", access)
	}
	if access.NeedsApproval {
		t.Fatalf("dontAsk should not prompt approved tools, got %+v", access)
	}
}

func TestNormalizeExecutionModeAcceptsClaudeAliases(t *testing.T) {
	tests := map[string]string{
		"default":            "default",
		"manual":             "default",
		"acceptEdits":        "acceptEdits",
		"accept_edits":       "acceptEdits",
		"dontAsk":            "dontAsk",
		"dont_ask":           "dontAsk",
		"bypassPermissions":  "bypassPermissions",
		"bypass_permissions": "bypassPermissions",
		"bypass":             "bypassPermissions",
		"unknown":            "default",
	}

	for input, want := range tests {
		if got := NormalizeExecutionMode(input); got != want {
			t.Fatalf("NormalizeExecutionMode(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestExecutionModeDescriptorForReturnsAliases(t *testing.T) {
	desc := ExecutionModeDescriptorFor("acceptEdits")
	if desc.Name != "acceptEdits" {
		t.Fatalf("Name=%q, want acceptEdits", desc.Name)
	}
	found := false
	for _, alias := range desc.Aliases {
		if alias == "accept-edits" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Aliases=%v, want accept-edits alias", desc.Aliases)
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
