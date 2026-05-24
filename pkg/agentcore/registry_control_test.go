package agentcore

import "testing"

func TestRegistryControlUsesExternalIDAndUpdatesState(t *testing.T) {
	registry := NewRegistry(NewDefaultRoleRegistry())

	agent, err := registry.RegisterRootWithID("subagent_reviewer_1", "review", "inspect")
	if err != nil {
		t.Fatalf("RegisterRootWithID() error = %v", err)
	}
	if agent.ID != "subagent_reviewer_1" || agent.RoleID != "reviewer" || agent.Task != "inspect" {
		t.Fatalf("agent=%+v, want external id, canonical reviewer role, task", agent)
	}

	agent, err = registry.UpdateRoleTaskStatus(agent.ID, "verify", "verify result", AgentRunning)
	if err != nil {
		t.Fatalf("UpdateRoleTaskStatus() error = %v", err)
	}
	if agent.RoleID != "verification" || agent.Task != "verify result" || agent.Status != AgentRunning {
		t.Fatalf("agent=%+v, want verification running state", agent)
	}

	got, ok := registry.Get("subagent_reviewer_1")
	if !ok {
		t.Fatalf("Get() ok=false")
	}
	if got.RoleID != "verification" || got.Status != AgentRunning {
		t.Fatalf("got=%+v, want updated state", got)
	}

	agent, err = registry.UpdateRoleTask(agent.ID, "reviewer", "review again")
	if err != nil {
		t.Fatalf("UpdateRoleTask() error = %v", err)
	}
	if agent.RoleID != "reviewer" || agent.Task != "review again" || agent.Status != AgentRunning {
		t.Fatalf("agent=%+v, want updated role/task with preserved running status", agent)
	}

	if !registry.Remove("subagent_reviewer_1") {
		t.Fatalf("Remove() = false, want true")
	}
	if _, ok := registry.Get("subagent_reviewer_1"); ok {
		t.Fatalf("agent should be removed")
	}
}

func TestRegistryControlRemovesChildLinks(t *testing.T) {
	registry := NewRegistry(NewDefaultRoleRegistry())
	root, err := registry.RegisterRootWithID("root", "planner", "plan")
	if err != nil {
		t.Fatalf("RegisterRootWithID(root) error = %v", err)
	}
	child, err := registry.SpawnWithID(root.ID, "child", "reviewer", "review")
	if err != nil {
		t.Fatalf("SpawnWithID(child) error = %v", err)
	}
	if children := registry.Children(root.ID); len(children) != 1 || children[0].ID != child.ID {
		t.Fatalf("children=%+v, want child", children)
	}

	if !registry.Remove(child.ID) {
		t.Fatalf("Remove(child) = false, want true")
	}
	if children := registry.Children(root.ID); len(children) != 0 {
		t.Fatalf("children after remove=%+v, want none", children)
	}
}
