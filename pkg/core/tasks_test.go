package core

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type fakeTaskBackend struct {
	list  []BackgroundTask
	todos []TodoItem

	tailID      string
	tailFromSeq int64
	tailLimit   int
	tailLines   []string
	tailErr     error

	killID  string
	killErr error

	cleanup int
}

func (f *fakeTaskBackend) ListTasks() []BackgroundTask {
	return append([]BackgroundTask(nil), f.list...)
}

func (f *fakeTaskBackend) ListTodos() []TodoItem {
	return append([]TodoItem(nil), f.todos...)
}

func (f *fakeTaskBackend) TailTask(taskID string, fromSeq int64, limit int) ([]string, error) {
	f.tailID = taskID
	f.tailFromSeq = fromSeq
	f.tailLimit = limit
	return append([]string(nil), f.tailLines...), f.tailErr
}

func (f *fakeTaskBackend) KillTask(taskID string) error {
	f.killID = taskID
	return f.killErr
}

func (f *fakeTaskBackend) CleanupFinishedTasks() int {
	return f.cleanup
}

func TestRuntimeTaskMethodsUseInjectedBackend(t *testing.T) {
	base := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	backend := &fakeTaskBackend{
		list: []BackgroundTask{
			{ID: "older", StartedAt: base.Add(-time.Minute), Label: "older"},
			{ID: "newer-b", StartedAt: base, Label: "newer-b"},
			{ID: "newer-a", StartedAt: base, Label: "newer-a"},
		},
		todos: []TodoItem{
			{ID: "todo-1", Content: "first", Status: "pending", Priority: "high", UpdatedAt: base},
		},
		tailLines: []string{"stdout: ready"},
		cleanup:   2,
	}
	rt := &Runtime{tasks: backend}

	items := rt.ListTasks()
	gotIDs := make([]string, 0, len(items))
	for _, item := range items {
		gotIDs = append(gotIDs, item.ID)
	}
	wantIDs := []string{"newer-a", "newer-b", "older"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("ListTasks IDs = %v, want %v", gotIDs, wantIDs)
	}

	lines, err := rt.TailTask(" task-1 ")
	if err != nil {
		t.Fatalf("TailTask() error = %v", err)
	}
	if backend.tailID != "task-1" || backend.tailFromSeq != 0 || backend.tailLimit != 200 {
		t.Fatalf("TailTask backend call = id=%q from=%d limit=%d, want task-1/0/200", backend.tailID, backend.tailFromSeq, backend.tailLimit)
	}
	if !reflect.DeepEqual(lines, []string{"stdout: ready"}) {
		t.Fatalf("TailTask lines = %v", lines)
	}

	todos := rt.ListTodos()
	if !reflect.DeepEqual(todos, backend.todos) {
		t.Fatalf("ListTodos() = %+v, want %+v", todos, backend.todos)
	}
	todos[0].Content = "mutated"
	if backend.todos[0].Content != "first" {
		t.Fatalf("ListTodos returned backend slice; backend content = %q", backend.todos[0].Content)
	}

	changes, unsubscribe := rt.SubscribeStateChanges(4)
	defer unsubscribe()

	if err := rt.KillTask(" task-1 "); err != nil {
		t.Fatalf("KillTask() error = %v", err)
	}
	if backend.killID != "task-1" {
		t.Fatalf("KillTask backend id = %q, want task-1", backend.killID)
	}
	assertStateChange(t, changes, StateTopicTasks, "task.kill")

	if got := rt.CleanupTasks(); got != 2 {
		t.Fatalf("CleanupTasks() = %d, want 2", got)
	}
	assertStateChange(t, changes, StateTopicTasks, "task.cleanup")
}

func TestRuntimeTaskMethodsValidateTaskID(t *testing.T) {
	backend := &fakeTaskBackend{}
	rt := &Runtime{tasks: backend}

	if _, err := rt.TailTask(" \t "); err == nil {
		t.Fatal("TailTask(empty) error = nil, want error")
	}
	if backend.tailID != "" {
		t.Fatalf("TailTask called backend with id %q", backend.tailID)
	}

	if err := rt.KillTask(""); err == nil {
		t.Fatal("KillTask(empty) error = nil, want error")
	}
	if backend.killID != "" {
		t.Fatalf("KillTask called backend with id %q", backend.killID)
	}
}

func TestRuntimeTaskMethodsDoNotNotifyOnBackendErrorsOrNoCleanup(t *testing.T) {
	backend := &fakeTaskBackend{killErr: errors.New("boom")}
	rt := &Runtime{tasks: backend}
	changes, unsubscribe := rt.SubscribeStateChanges(1)
	defer unsubscribe()

	if err := rt.KillTask("task-1"); err == nil {
		t.Fatal("KillTask() error = nil, want backend error")
	}
	assertNoStateChange(t, changes)

	if got := rt.CleanupTasks(); got != 0 {
		t.Fatalf("CleanupTasks() = %d, want 0", got)
	}
	assertNoStateChange(t, changes)
}

func TestLegacyTaskServiceTodosUsesRuntimeBackend(t *testing.T) {
	now := time.Date(2026, 5, 24, 11, 0, 0, 0, time.UTC)
	backend := &fakeTaskBackend{
		todos: []TodoItem{
			{ID: "todo-1", Content: "ship task backend", Status: "done", Priority: 1, UpdatedAt: now},
		},
	}
	items, err := (legacyTaskService{rt: &Runtime{tasks: backend}}).Todos(context.Background())
	if err != nil {
		t.Fatalf("Todos() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("Todos() len = %d, want 1", len(items))
	}
	if items[0].ID != "todo-1" || items[0].Content != "ship task backend" || items[0].Status != "done" || items[0].UpdatedAt != now {
		t.Fatalf("Todos()[0] = %+v", items[0])
	}
}

func assertStateChange(t *testing.T, ch <-chan StateChangeEvent, topic, source string) {
	t.Helper()
	select {
	case ev := <-ch:
		if ev.Topic != topic || ev.Source != source {
			t.Fatalf("state change = (%q, %q), want (%q, %q)", ev.Topic, ev.Source, topic, source)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for state change %s/%s", topic, source)
	}
}

func assertNoStateChange(t *testing.T, ch <-chan StateChangeEvent) {
	t.Helper()
	select {
	case ev := <-ch:
		t.Fatalf("unexpected state change: %+v", ev)
	case <-time.After(25 * time.Millisecond):
	}
}
