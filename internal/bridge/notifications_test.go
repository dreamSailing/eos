package bridge

import (
	"context"
	"testing"
	"time"

	"github.com/dreamSailing/vb-coding/internal/notify"
	"github.com/dreamSailing/vb-coding/internal/pkg/settings"
	"github.com/dreamSailing/vb-coding/internal/session"
)

func TestWaitPrompt_SendsDesktopNotification(t *testing.T) {
	ch := make(chan struct{}, 2)
	notify.SetSender(func(title, message string) error {
		ch <- struct{}{}
		return nil
	})
	defer notify.ResetSender()

	rc := &RuntimeCore{
		securityMgr: NewSecurityManager(),
		eventsCh:    make(chan Event, 2),
	}
	rc.securityMgr.SetExecutionMode("plan")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		_, _ = rc.waitPrompt(ctx, PromptRequest{
			Kind:     PromptKindPermission,
			Title:    "Permission required",
			Question: "fs write test",
			Options:  []string{"allow_once", "deny"},
			Category: "file_write",
			Summary:  "fs write test",
		})
		close(done)
	}()

	ev := <-rc.eventsCh
	if ev.Type != "approval.required" || ev.RID == "" {
		t.Fatalf("unexpected event: %#v", ev)
	}
	if got, _ := ev.Data["approval_id"].(string); got != ev.RID {
		t.Fatalf("approval_id=%q, want %q", got, ev.RID)
	}

	select {
	case <-ch:
	case <-time.After(1 * time.Second):
		t.Fatalf("expected desktop notification")
	}

	ok := rc.SubmitPromptResponse(ev.RID, PromptResponse{Decision: "allow_once"})
	if !ok {
		t.Fatalf("failed to submit prompt response")
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatalf("prompt did not return")
	}
}

func TestFinalizeTask_SendsDesktopNotificationInAuto(t *testing.T) {
	ch := make(chan struct{}, 2)
	notify.SetSender(func(title, message string) error {
		ch <- struct{}{}
		return nil
	})
	defer notify.ResetSender()

	rc := &RuntimeCore{
		cm:          session.NewContextManager(),
		securityMgr: NewSecurityManager(),
	}
	rc.securityMgr.SetExecutionMode("auto")

	rc.finalizeTask(nil, "t1", "hello", "ok", true, "")

	select {
	case <-ch:
	case <-time.After(1 * time.Second):
		t.Fatalf("expected desktop notification")
	}
}

func TestFinalizeTask_NoDesktopNotificationWhenDisabled(t *testing.T) {
	ch := make(chan struct{}, 2)
	notify.SetSender(func(title, message string) error {
		ch <- struct{}{}
		return nil
	})
	defer notify.ResetSender()

	f := false
	rc := &RuntimeCore{
		cm:          session.NewContextManager(),
		securityMgr: NewSecurityManager(),
		settings:    settings.Settings{DesktopNotifications: &f},
	}
	rc.securityMgr.SetExecutionMode("auto")

	rc.finalizeTask(nil, "t1", "hello", "ok", true, "")

	select {
	case <-ch:
		t.Fatalf("did not expect desktop notification")
	case <-time.After(300 * time.Millisecond):
	}
}
