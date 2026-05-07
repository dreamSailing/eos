package scheduler

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	toolapiimpl "github.com/dreamSailing/eos/internal/toolapi/impl"
	"github.com/dreamSailing/eos/internal/tools/bg"
)

func TestServiceCreateUpdateDelete(t *testing.T) {
	service := NewService(filepath.Join(t.TempDir(), "schedules.json"), toolapiimpl.NewServices(), t.TempDir())
	created, err := service.Create(Schedule{
		Name:    "shell job",
		Enabled: true,
		Cron:    "*/5 * * * *",
		Kind:    TaskKindShell,
		Payload: map[string]any{"command": "printf hi"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected created ID")
	}
	updated, err := service.Update(created.ID, Schedule{
		Name:    "shell job 2",
		Enabled: false,
		Cron:    "*/10 * * * *",
		Kind:    TaskKindShell,
		Payload: map[string]any{"command": "printf ok"},
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != "shell job 2" || updated.Enabled {
		t.Fatalf("unexpected updated item: %+v", updated)
	}
	if err := service.Delete(created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if items := service.List(); len(items) != 0 {
		t.Fatalf("expected empty list, got %d items", len(items))
	}
}

func TestServiceTriggerNowShell(t *testing.T) {
	workspace := t.TempDir()
	before := taskIDs(bg.Default().List())
	service := NewService(filepath.Join(t.TempDir(), "schedules.json"), toolapiimpl.NewServices(), workspace)
	item, err := service.Create(Schedule{
		Name:    "trigger shell",
		Enabled: true,
		Cron:    "*/5 * * * *",
		Kind:    TaskKindShell,
		Payload: map[string]any{"command": "echo hi"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := service.TriggerNow(context.Background(), item.ID); err != nil {
		t.Fatalf("TriggerNow() error = %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	got, ok := service.Get(item.ID)
	if !ok {
		t.Fatalf("expected schedule to exist")
	}
	if got.ID != item.ID {
		t.Fatalf("unexpected schedule: %+v", got)
	}
	waitForNewBackgroundTaskExit(t, before, workspace)
}

func taskIDs(items []bg.TaskInfo) map[string]struct{} {
	ids := make(map[string]struct{}, len(items))
	for _, item := range items {
		ids[item.ID] = struct{}{}
	}
	return ids
}

func waitForNewBackgroundTaskExit(t *testing.T, before map[string]struct{}, workspace string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	seen := ""
	for time.Now().Before(deadline) {
		for _, item := range bg.Default().List() {
			if _, ok := before[item.ID]; ok {
				continue
			}
			if item.WorkingDir != workspace {
				continue
			}
			seen = item.ID
			if item.Status != bg.StatusRunning {
				bg.Default().CleanupFinished()
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if seen != "" {
		_, _ = bg.Default().Kill(seen)
		bg.Default().CleanupFinished()
	}
	t.Fatalf("background shell task did not exit before cleanup")
}
