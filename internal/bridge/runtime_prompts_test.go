package bridge

import (
	"context"
	"testing"
	"time"
)

func TestPromptPermissionAutoAllowsMediumRisk(t *testing.T) {
	rc := &RuntimeCore{
		securityMgr: NewSecurityManager(),
		eventsCh:    make(chan Event, 1),
	}
	rc.securityMgr.SetExecutionMode("auto")

	if got := rc.promptPermission(context.Background(), "file_write", "fs write ./internal/ui/app.go"); got != "allow" {
		t.Fatalf("promptPermission() = %q, want allow", got)
	}

	select {
	case ev := <-rc.eventsCh:
		t.Fatalf("did not expect prompt event, got %#v", ev)
	default:
	}
}

func TestPromptPermissionAutoStillPromptsHighRisk(t *testing.T) {
	rc := &RuntimeCore{
		securityMgr: NewSecurityManager(),
		eventsCh:    make(chan Event, 2),
	}
	rc.securityMgr.SetExecutionMode("auto")

	done := make(chan string, 1)
	go func() {
		done <- rc.promptPermission(context.Background(), "git-reset", "git reset HEAD~1")
	}()

	select {
	case ev := <-rc.eventsCh:
		if ev.Type != "prompt.request" || ev.RID == "" {
			t.Fatalf("unexpected event: %#v", ev)
		}
		if ok := rc.SubmitPromptResponse(ev.RID, PromptResponse{Decision: "allow_once"}); !ok {
			t.Fatalf("failed to submit prompt response")
		}
	case <-time.After(time.Second):
		t.Fatalf("expected high-risk prompt event")
	}

	select {
	case got := <-done:
		if got != "allow" {
			t.Fatalf("promptPermission() = %q, want allow", got)
		}
	case <-time.After(time.Second):
		t.Fatalf("promptPermission did not return")
	}
}
