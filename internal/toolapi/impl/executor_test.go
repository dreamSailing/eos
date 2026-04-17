package impl

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/toolapi"
)

func TestExecutorInitializesSkillsAndMCPManagers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	cfg := config.Config{
		MCP: []config.MCPEntry{
			{Name: "demo", Type: config.MCPTypeStdio, Command: "does-not-exist", Enabled: true},
		},
	}
	if err := config.Save(cfg, config.Path()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	skillDir := filepath.Join(home, ".eos", "skills", "review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	content := "---\nname: review\ndescription: code review helper\n---\n\nbody"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	workspace := t.TempDir()
	exec := newExecutor(workspace)
	results, err := exec.Execute(context.Background(), toolapi.ExecSession{
		WorkspaceRoot: workspace,
		AllowedTools: map[string]bool{
			"skills_list": true,
			"mcp_status":  true,
		},
		ExecutionMode: "auto",
	}, []toolapi.ToolCall{
		{ID: "c1", Name: "skills_list", Params: map[string]any{}},
		{ID: "c2", Name: "mcp_status", Params: map[string]any{}},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results=%d want=2", len(results))
	}
	if results[0].Status != "success" {
		t.Fatalf("skills_list status=%q", results[0].Status)
	}
	names, _ := results[0].Data["names"].([]string)
	if len(names) == 0 || names[0] != "review" {
		t.Fatalf("skills_list names=%v", names)
	}
	if results[1].Status != "success" {
		t.Fatalf("mcp_status status=%q error=%q", results[1].Status, results[1].Error)
	}
	servers, _ := results[1].Data["servers"].([]map[string]any)
	if len(servers) != 1 {
		t.Fatalf("mcp_status servers=%v", results[1].Data["servers"])
	}
}
