package roles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltinRolesLoadAndResolve(t *testing.T) {
	registry := NewDefaultRegistry()
	got := registry.List()
	if len(got) != 8 {
		t.Fatalf("List() len=%d, want 8", len(got))
	}

	for _, roleID := range []string{
		"planner",
		"senior-dev",
		"tester",
		"verification",
		"reviewer",
		"explore",
		"security",
		"architect",
	} {
		role, ok := registry.Resolve(roleID)
		if !ok {
			t.Fatalf("Resolve(%q) ok=false", roleID)
		}
		if role.ID != roleID {
			t.Fatalf("Resolve(%q).ID=%q", roleID, role.ID)
		}
		if strings.TrimSpace(role.SystemPrompt) == "" {
			t.Fatalf("Resolve(%q).SystemPrompt is empty", roleID)
		}
	}

	keywords := map[string][]string{
		"planner":      {"核心目标", "实施计划", "验证方案"},
		"senior-dev":   {"效率优先", "精准定位", "安全优先"},
		"tester":       {"测试结果", "明确结论"},
		"verification": {"VERDICT", "对抗式验收", "验证重点"},
		"reviewer":     {"代码质量", "任务完成度"},
		"explore":      {"探索", "代码库"},
		"security":     {"安全", "OWASP"},
		"architect":    {"执行路径", "调度中心"},
	}
	for roleID, expected := range keywords {
		role, _ := registry.Resolve(roleID)
		for _, kw := range expected {
			if !strings.Contains(role.SystemPrompt, kw) {
				t.Errorf("role %q prompt missing keyword %q", roleID, kw)
			}
		}
	}
}

func TestRegistryResolvesLegacyNames(t *testing.T) {
	registry := NewDefaultRegistry()
	cases := map[string]string{
		"senior_dev": "senior-dev",
		"verify":     "verification",
		"review":     "reviewer",
		"explorer":   "explore",
	}
	for alias, want := range cases {
		role, ok := registry.Resolve(alias)
		if !ok {
			t.Fatalf("Resolve(%q) ok=false", alias)
		}
		if role.ID != want {
			t.Fatalf("Resolve(%q).ID=%q, want %q", alias, role.ID, want)
		}
	}
}

func TestLoadRegistryWithPathsProjectOverridesUser(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user-roles.json")
	projectPath := filepath.Join(dir, "project-roles.json")

	if err := os.WriteFile(userPath, []byte(`{
		"roles": [{
			"id": "reviewer",
			"description": "user reviewer",
			"system_prompt": "user prompt",
			"context_strategy": "hybrid",
			"allowed_tools": ["read", "grep"]
		}]
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile(user) error = %v", err)
	}
	if err := os.WriteFile(projectPath, []byte(`{
		"roles": [{
			"id": "reviewer",
			"description": "project reviewer",
			"system_prompt": "project prompt",
			"context_strategy": "independent",
			"allowed_tools": ["search", "read"],
			"legacy_names": ["review-project"]
		}]
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}

	registry, err := LoadRegistryWithPaths(ConfigPaths{
		UserPath:    userPath,
		ProjectPath: projectPath,
	})
	if err != nil {
		t.Fatalf("LoadRegistryWithPaths() error = %v", err)
	}
	role, ok := registry.Resolve("review-project")
	if !ok {
		t.Fatal("Resolve(review-project) ok=false")
	}
	if role.Description != "project reviewer" {
		t.Fatalf("Description=%q, want project reviewer", role.Description)
	}
	if role.ContextStrategy != ContextIndependent {
		t.Fatalf("ContextStrategy=%q, want %q", role.ContextStrategy, ContextIndependent)
	}
	if got, want := strings.Join(role.AllowedTools, ","), "search,read"; got != want {
		t.Fatalf("AllowedTools=%q, want %q", got, want)
	}
}

func TestApplyJSONRejectsInvalidConfig(t *testing.T) {
	registry := NewDefaultRegistry()
	cases := []struct {
		name string
		data string
		want string
	}{
		{
			name: "duplicate id",
			data: `{"roles":[
				{"id":"dup","system_prompt":"one"},
				{"id":"dup","system_prompt":"two"}
			]}`,
			want: `duplicate role id "dup"`,
		},
		{
			name: "empty prompt",
			data: `{"roles":[{"id":"empty"}]}`,
			want: `role "empty" needs system_prompt or prompt_file`,
		},
		{
			name: "invalid role id",
			data: `{"roles":[{"id":"bad role","system_prompt":"prompt"}]}`,
			want: `role "bad role" has invalid id`,
		},
		{
			name: "invalid allowed tool",
			data: `{"roles":[{"id":"bad-tool","system_prompt":"prompt","allowed_tools":["nope"]}]}`,
			want: `unsupported allowed_tools entry "nope"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := registry.ApplyJSON([]byte(tc.data))
			if err == nil {
				t.Fatal("ApplyJSON() error = nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ApplyJSON() error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestNormalizeAllowedToolsPreservesOrderAndDedupes(t *testing.T) {
	registry, err := NewRegistry([]RoleConfig{{
		ID:           "tool-role",
		SystemPrompt: "prompt",
		AllowedTools: []string{" read ", "READ", "search", "Search", "fs/*", "fs/*", "ProjectStructure", "projectstructure"},
	}})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	role, ok := registry.Resolve("tool-role")
	if !ok {
		t.Fatal("Resolve(tool-role) ok=false")
	}
	if got, want := strings.Join(role.AllowedTools, ","), "read,search,fs/*,ProjectStructure"; got != want {
		t.Fatalf("AllowedTools=%q, want %q", got, want)
	}
}

func TestApplyJSONFileLoadsPromptRelativeToConfig(t *testing.T) {
	dir := t.TempDir()
	promptDir := filepath.Join(dir, "prompts")
	if err := os.MkdirAll(promptDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(promptDir, "custom.md"), []byte("custom file prompt\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(prompt) error = %v", err)
	}
	configPath := filepath.Join(dir, "roles.json")
	if err := os.WriteFile(configPath, []byte(`{
		"roles": [{
			"id": "custom",
			"description": "file prompt role",
			"prompt_file": "prompts/custom.md",
			"context_strategy": "independent"
		}]
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	registry := NewDefaultRegistry()
	if err := registry.ApplyJSONFile(configPath); err != nil {
		t.Fatalf("ApplyJSONFile() error = %v", err)
	}
	role, ok := registry.Resolve("custom")
	if !ok {
		t.Fatal("Resolve(custom) ok=false")
	}
	if role.SystemPrompt != "custom file prompt" {
		t.Fatalf("SystemPrompt=%q, want custom file prompt", role.SystemPrompt)
	}
}
