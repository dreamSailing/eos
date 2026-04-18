package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"testing"
	"time"

	"github.com/dreamSailing/eos/internal/tools"
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
		if ev.Type != "approval.required" || ev.RID == "" {
			t.Fatalf("unexpected event: %#v", ev)
		}
		if got, _ := ev.Data["approval_id"].(string); got != ev.RID {
			t.Fatalf("approval_id=%q, want %q", got, ev.RID)
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

func TestPromptPermissionLegacyAcceptEditsNormalizesToAuto(t *testing.T) {
	rc := &RuntimeCore{
		securityMgr: NewSecurityManager(),
		eventsCh:    make(chan Event, 1),
	}
	rc.securityMgr.SetExecutionMode("acceptEdits")

	if got := rc.promptPermission(context.Background(), "file_write", "fs write ./README.md"); got != "allow" {
		t.Fatalf("promptPermission() = %q, want allow", got)
	}

	select {
	case ev := <-rc.eventsCh:
		t.Fatalf("did not expect prompt event, got %#v", ev)
	default:
	}
}

func TestPromptPermissionPlanDeniesInsteadOfPrompting(t *testing.T) {
	rc := &RuntimeCore{
		securityMgr: NewSecurityManager(),
		eventsCh:    make(chan Event, 1),
	}
	rc.securityMgr.SetExecutionMode("plan")

	if got := rc.promptPermission(context.Background(), "git-push", "git push"); got != "deny" {
		t.Fatalf("promptPermission() = %q, want deny", got)
	}

	select {
	case ev := <-rc.eventsCh:
		t.Fatalf("did not expect prompt event, got %#v", ev)
	default:
	}
}

func TestUserConfirmPromptStillAsksUserInAutoMode(t *testing.T) {
	rc := &RuntimeCore{
		securityMgr: NewSecurityManager(),
		eventsCh:    make(chan Event, 2),
	}
	rc.securityMgr.SetExecutionMode("auto")

	done := make(chan tools.UserConfirmResponse, 1)
	go func() {
		res, err := rc.userConfirmPrompt(context.Background(), tools.UserConfirmRequest{
			Question: "请选择执行策略",
			Options:  []string{"方案A", "方案B"},
		})
		if err != nil {
			t.Errorf("userConfirmPrompt() error = %v", err)
			return
		}
		done <- res
	}()

	select {
	case ev := <-rc.eventsCh:
		if ev.Type != "approval.required" && ev.Type != "inquiry.required" && ev.Type != "user_confirm.required" {
			t.Fatalf("unexpected prompt event type: %s", ev.Type)
		}
		if ev.RID == "" {
			t.Fatalf("prompt event missing request id: %#v", ev)
		}
		if ok := rc.SubmitPromptResponse(ev.RID, PromptResponse{Decision: "confirm", Option: "方案B", OptionIndex: 1}); !ok {
			t.Fatalf("failed to submit prompt response")
		}
	case <-time.After(time.Second):
		t.Fatalf("expected user confirm prompt event")
	}

	select {
	case res := <-done:
		if !res.Confirmed || res.Option != "方案B" || res.OptionIndex != 1 {
			t.Fatalf("unexpected response: %+v", res)
		}
	case <-time.After(time.Second):
		t.Fatalf("userConfirmPrompt did not return")
	}
}
