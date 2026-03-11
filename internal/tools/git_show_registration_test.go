package tools

import "testing"

func TestGitShow_ToolDefinitionRegistered(t *testing.T) {
	def, ok := GetToolDefinition(ToolGitShow)
	if !ok {
		t.Fatalf("tool definition not found: %s", ToolGitShow)
	}
	if def.Name != ToolGitShow {
		t.Fatalf("unexpected tool name: %s", def.Name)
	}
	if def.RiskLevel != RiskLevelLow {
		t.Fatalf("unexpected risk level: %v", def.RiskLevel)
	}
}

func TestGitShow_ManagerStructuredRegistered(t *testing.T) {
	m := NewManager()
	if _, ok := m.structured[ToolGitShow]; !ok {
		t.Fatalf("tool handler not registered: %s", ToolGitShow)
	}
}
