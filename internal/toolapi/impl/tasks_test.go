package impl

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"testing"

	"github.com/dreamSailing/eos/internal/runtime"
	"github.com/dreamSailing/eos/internal/tools"
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

func TestTasksResumeAndCloseDelegateToAgentRegistryController(t *testing.T) {
	mgr := runtime.NewSubAgentManager()
	subCtx := mgr.CreateContext(runtime.SubAgentTypeReviewer, context.Background(), nil)

	resumed := false
	closed := false
	registryID := runtime.DefaultAgentRegistry().RegisterController(
		mgr,
		func(id string, task string) error {
			if id != subCtx.ID() {
				t.Fatalf("resume id=%q, want %q", id, subCtx.ID())
			}
			resumed = true
			return nil
		},
		func(id string) error {
			if id != subCtx.ID() {
				t.Fatalf("close id=%q, want %q", id, subCtx.ID())
			}
			closed = true
			return nil
		},
	)
	t.Cleanup(func() {
		runtime.DefaultAgentRegistry().UnregisterManager(registryID)
	})

	taskSvc := newTasks()
	if err := taskSvc.Resume(context.Background(), subCtx.ID()); err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if !resumed {
		t.Fatalf("expected registry resume callback to be used")
	}
	if err := taskSvc.Close(context.Background(), subCtx.ID()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !closed {
		t.Fatalf("expected registry close callback to be used")
	}
}
