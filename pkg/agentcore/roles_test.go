package agentcore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultRolesResolveLegacyAliases(t *testing.T) {
	registry := NewDefaultRoleRegistry()
	role, ok := registry.Resolve("senior_dev")
	if !ok {
		t.Fatal("expected senior_dev alias to resolve")
	}
	if role.ID != "senior-dev" {
		t.Fatalf("role.ID=%q, want senior-dev", role.ID)
	}

	role, ok = registry.Resolve("explorer")
	if !ok {
		t.Fatal("expected explorer alias to resolve")
	}
	if role.ContextStrategy != ContextIndependent {
		t.Fatalf("strategy=%q, want %q", role.ContextStrategy, ContextIndependent)
	}
}

func TestApplyJSONOverridesRole(t *testing.T) {
	registry := NewDefaultRoleRegistry()
	err := registry.ApplyJSON([]byte(`{
		"roles": [{
			"id": "reviewer",
			"description": "custom reviewer",
			"system_prompt": "custom prompt",
			"context_strategy": "hybrid",
			"allowed_tools": ["read"]
		}]
	}`))
	if err != nil {
		t.Fatalf("ApplyJSON() error = %v", err)
	}
	role, ok := registry.Resolve("reviewer")
	if !ok {
		t.Fatal("reviewer not found")
	}
	if role.Description != "custom reviewer" {
		t.Fatalf("description=%q, want custom reviewer", role.Description)
	}
	if len(role.AllowedTools) != 1 || role.AllowedTools[0] != "read" {
		t.Fatalf("allowed tools=%v, want [read]", role.AllowedTools)
	}
}

func TestLoadRoleRegistryAppliesFilesInOrder(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user-roles.json")
	projectPath := filepath.Join(dir, "project-roles.json")
	if err := os.WriteFile(userPath, []byte(`{
		"roles": [{
			"id": "reviewer",
			"description": "user reviewer",
			"system_prompt": "user prompt",
			"context_strategy": "hybrid",
			"allowed_tools": ["read"]
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
			"allowed_tools": ["grep"],
			"legacy_aliases": ["review-project"]
		}]
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}
	registry, err := LoadRoleRegistry(userPath, filepath.Join(dir, "missing.json"), projectPath)
	if err != nil {
		t.Fatalf("LoadRoleRegistry() error = %v", err)
	}
	role, ok := registry.Resolve("review-project")
	if !ok {
		t.Fatal("review-project alias did not resolve")
	}
	if role.Description != "project reviewer" {
		t.Fatalf("description=%q, want project reviewer", role.Description)
	}
	if role.ContextStrategy != ContextIndependent {
		t.Fatalf("strategy=%q, want independent", role.ContextStrategy)
	}
	if len(role.AllowedTools) != 1 || role.AllowedTools[0] != "grep" {
		t.Fatalf("allowed tools=%v, want [grep]", role.AllowedTools)
	}
}

func TestApplyJSONFileLoadsPromptFileRelativeToConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "prompts"), 0o755); err != nil {
		t.Fatalf("MkdirAll(prompts) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts", "custom.md"), []byte("custom file prompt\n"), 0o644); err != nil {
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

	registry := NewDefaultRoleRegistry()
	if err := registry.ApplyJSONFile(configPath); err != nil {
		t.Fatalf("ApplyJSONFile() error = %v", err)
	}
	role, ok := registry.Resolve("custom")
	if !ok {
		t.Fatal("custom role did not resolve")
	}
	if role.SystemPrompt != "custom file prompt" {
		t.Fatalf("SystemPrompt=%q, want prompt file contents", role.SystemPrompt)
	}
	if role.PromptFile != "prompts/custom.md" {
		t.Fatalf("PromptFile=%q, want original relative path", role.PromptFile)
	}
}

func TestRegistryTracksParentChildAgents(t *testing.T) {
	registry := NewRegistry(NewDefaultRoleRegistry())
	root, err := registry.RegisterRootWithTask("planner", "plan work")
	if err != nil {
		t.Fatalf("RegisterRoot() error = %v", err)
	}
	if root.Task != "plan work" {
		t.Fatalf("root.Task=%q, want plan work", root.Task)
	}
	child, err := registry.Spawn(root.ID, "review", "inspect changes")
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}
	if child.ParentID != root.ID {
		t.Fatalf("child.ParentID=%q, want %q", child.ParentID, root.ID)
	}
	children := registry.Children(root.ID)
	if len(children) != 1 || children[0].ID != child.ID {
		t.Fatalf("children=%v, want child %s", children, child.ID)
	}
	items := registry.List()
	if len(items) != 2 || items[0].ID != root.ID || items[1].ID != child.ID {
		t.Fatalf("List()=%+v, want root then child", items)
	}
	got, ok := registry.Get(child.ID)
	if !ok {
		t.Fatalf("Get(%q) returned ok=false", child.ID)
	}
	if got.ID != child.ID {
		t.Fatalf("Get(%q).ID=%q", child.ID, got.ID)
	}
}

func TestMailboxDrainsMessages(t *testing.T) {
	box := NewMailbox()
	if err := box.Send(MailboxMessage{FromAgentID: "agent_1", ToAgentID: "agent_2", Body: "hello"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	listed := box.List("agent_2")
	if len(listed) != 1 || listed[0].Body != "hello" {
		t.Fatalf("List()=%+v, want hello message", listed)
	}
	msgs := box.Drain("agent_2")
	if len(msgs) != 1 {
		t.Fatalf("messages=%d, want 1", len(msgs))
	}
	if again := box.Drain("agent_2"); len(again) != 0 {
		t.Fatalf("messages after drain=%d, want 0", len(again))
	}
}

func TestMailboxClearRemovesMessages(t *testing.T) {
	box := NewMailbox()
	if err := box.Send(MailboxMessage{FromAgentID: "user", ToAgentID: "agent_1", Body: "hello"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	box.Clear("agent_1")
	if got := box.List("agent_1"); len(got) != 0 {
		t.Fatalf("List() after Clear = %+v, want none", got)
	}
}
