package impl

import (
	"context"
	"testing"

	"github.com/dreamSailing/vb-coding/internal/runtime"
	"github.com/dreamSailing/vb-coding/internal/tools"
)

func TestTasksListIncludesTodoAndAgent(t *testing.T) {
	store := tools.DefaultTodoStore()
	prev := store.List()
	t.Cleanup(func() {
		store.Replace(prev)
	})
	store.Replace([]tools.TodoItem{
		{ID: "todo_phase2_alignment", Content: "Align runtime task center", Status: "pending"},
	})

	mgr := runtime.NewSubAgentManager()
	registryID := runtime.DefaultAgentRegistry().RegisterManager(mgr)
	t.Cleanup(func() {
		runtime.DefaultAgentRegistry().UnregisterManager(registryID)
	})

	subCtx := mgr.CreateContext(runtime.SubAgentTypePlanner, context.Background(), nil)
	if err := mgr.MarkRunning(subCtx.ID(), "Plan task center", nil); err != nil {
		t.Fatalf("MarkRunning() error = %v", err)
	}
	mgr.Complete(subCtx.ID(), "Plan task center", true, "")

	items, err := newTasks().List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	var foundTodo bool
	var foundAgent bool
	for _, item := range items {
		if item.ID == "todo_phase2_alignment" && item.Kind == "todo_item" && item.Status == "pending" {
			foundTodo = true
		}
		if item.ID == subCtx.ID() && item.Kind == "agent_task" && item.Status == string(runtime.AgentStatusCompleted) {
			foundAgent = true
		}
	}

	if !foundTodo {
		t.Fatalf("expected todo_item in task list, got %#v", items)
	}
	if !foundAgent {
		t.Fatalf("expected agent_task in task list, got %#v", items)
	}
}

func TestTasksKillCancelsAgentTask(t *testing.T) {
	mgr := runtime.NewSubAgentManager()
	registryID := runtime.DefaultAgentRegistry().RegisterManager(mgr)
	t.Cleanup(func() {
		runtime.DefaultAgentRegistry().UnregisterManager(registryID)
	})

	subCtx := mgr.CreateContext(runtime.SubAgentTypeTester, context.Background(), nil)
	called := false
	cancel := func() { called = true }
	if err := mgr.MarkRunning(subCtx.ID(), "Cancel me", cancel); err != nil {
		t.Fatalf("MarkRunning() error = %v", err)
	}

	if err := newTasks().Kill(context.Background(), subCtx.ID()); err != nil {
		t.Fatalf("Kill() error = %v", err)
	}
	if !called {
		t.Fatalf("expected cancel func to be called")
	}
}
