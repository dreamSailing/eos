package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	codectx "github.com/dreamSailing/eos/internal/context"
	"github.com/dreamSailing/eos/internal/session"
)

func setRuntimeSessionTestHome(t *testing.T) string {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if volume := filepath.VolumeName(home); volume != "" {
		t.Setenv("HOMEDRIVE", volume)
		t.Setenv("HOMEPATH", strings.TrimPrefix(home, volume))
	}
	return home
}

func TestRuntimeCore_SaveAndResumeSession(t *testing.T) {
	setRuntimeSessionTestHome(t)
	dir := t.TempDir()
	old, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(old) })

	cm := session.NewContextManager()
	cm.AddUser("hello")
	cm.AddAssistant("world")

	rc := &RuntimeCore{cm: cm}
	rc.tokenHistory = []TokenRecord{{Total: intPtr(123)}}

	id, err := rc.SaveSession(context.Background(), "test_session")
	if err != nil {
		t.Fatalf("save error: %v", err)
	}
	if id != "test_session" {
		t.Fatalf("unexpected id: %s", id)
	}
	if _, err := os.Stat(filepath.Join(dir, ".eos", "sessions", "test_session.json")); err != nil {
		t.Fatalf("session file not found: %v", err)
	}

	cm2 := session.NewContextManager()
	rc2 := &RuntimeCore{cm: cm2}
	if err := rc2.ResumeSession(context.Background(), "test_session"); err != nil {
		t.Fatalf("resume error: %v", err)
	}
	msgs := cm2.BuildPreview()
	if len(msgs) == 0 {
		t.Fatalf("expected restored messages")
	}
}

func TestRuntimeCore_SaveAndResumeSession_UsesActiveRoot(t *testing.T) {
	setRuntimeSessionTestHome(t)
	dir := t.TempDir()
	other := t.TempDir()

	mgr := codectx.NewMultiEngine()
	mgr.AddRoot(dir)
	mgr.SetActive(dir)

	cm := session.NewContextManager()
	cm.AddUser("hello")
	cm.AddAssistant("world")

	rc := &RuntimeCore{cm: cm, workspaceMgr: mgr}
	rc.tokenHistory = []TokenRecord{{Total: intPtr(123)}}

	id, err := rc.SaveSession(context.Background(), "test_session")
	if err != nil {
		t.Fatalf("save error: %v", err)
	}
	if id != "test_session" {
		t.Fatalf("unexpected id: %s", id)
	}
	if _, err := os.Stat(filepath.Join(dir, ".eos", "sessions", "test_session.json")); err != nil {
		t.Fatalf("session file not found under active root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(other, ".eos", "sessions", "test_session.json")); !os.IsNotExist(err) {
		t.Fatalf("session file should not be created under unrelated root, err=%v", err)
	}

	cm2 := session.NewContextManager()
	rc2 := &RuntimeCore{cm: cm2, workspaceMgr: mgr}
	if err := rc2.ResumeSession(context.Background(), "test_session"); err != nil {
		t.Fatalf("resume error: %v", err)
	}
	msgs := cm2.BuildPreview()
	if len(msgs) == 0 {
		t.Fatalf("expected restored messages")
	}
}

func TestRuntimeCore_SaveSessionMessagesAndDeleteSession(t *testing.T) {
	setRuntimeSessionTestHome(t)
	dir := t.TempDir()
	old, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(old) })

	cm := session.NewContextManager()
	rc := &RuntimeCore{cm: cm}

	id, err := rc.SaveSessionMessages(context.Background(), "thread-1", []SessionTranscriptMessage{
		{Role: "user", Type: "user", Content: "hello"},
		{Role: "assistant", Type: "assistant", Content: "world"},
		{Role: "system", Type: "tool", Content: "tool output"},
	})
	if err != nil {
		t.Fatalf("save session messages error: %v", err)
	}
	if id != "thread-1" {
		t.Fatalf("unexpected id: %s", id)
	}

	msgs, err := rc.LoadSessionMessages("thread-1")
	if err != nil {
		t.Fatalf("load session messages error: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 transcript messages, got %d", len(msgs))
	}
	metas, err := rc.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("expected 1 session meta, got %d", len(metas))
	}
	if metas[0].Summary != "hello" {
		t.Fatalf("summary=%q, want hello", metas[0].Summary)
	}
	if metas[0].Preview != "world" {
		t.Fatalf("preview=%q, want world", metas[0].Preview)
	}
	if err := rc.UpdateSessionTitle("thread-1", "我的线程"); err != nil {
		t.Fatalf("UpdateSessionTitle() error = %v", err)
	}
	metas, err = rc.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() after rename error = %v", err)
	}
	if metas[0].Title != "我的线程" {
		t.Fatalf("title=%q, want 我的线程", metas[0].Title)
	}

	cm2 := session.NewContextManager()
	rc2 := &RuntimeCore{cm: cm2}
	if err := rc2.ResumeSession(context.Background(), "thread-1"); err != nil {
		t.Fatalf("resume error: %v", err)
	}
	preview := cm2.BuildPreview()
	if len(preview) != 3 {
		t.Fatalf("expected transcript to seed context, got %d messages", len(preview))
	}

	if err := rc.DeleteSession("thread-1"); err != nil {
		t.Fatalf("delete session error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".eos", "sessions", "thread-1.json")); !os.IsNotExist(err) {
		t.Fatalf("session file should be removed, err=%v", err)
	}
}

func TestRuntimeCore_SaveSessionMessagesRebuildsContextFromShorterTranscript(t *testing.T) {
	setRuntimeSessionTestHome(t)
	dir := t.TempDir()
	old, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(old) })

	cm := session.NewContextManager()
	cm.AddUser("keep this turn")
	cm.AddAssistant("keep reply")
	cm.AddUser("stale turn")
	cm.AddAssistant("stale reply")
	rc := &RuntimeCore{cm: cm}

	if _, err := rc.SaveSessionMessages(context.Background(), "thread-rollback", []SessionTranscriptMessage{
		{Role: "user", Type: "user", Content: "keep this turn"},
		{Role: "assistant", Type: "assistant", Content: "keep reply"},
	}); err != nil {
		t.Fatalf("SaveSessionMessages() error = %v", err)
	}

	metas, err := rc.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("len(ListSessions())=%d, want 1", len(metas))
	}
	if metas[0].Summary != "keep this turn" {
		t.Fatalf("summary=%q, want keep this turn", metas[0].Summary)
	}
	if metas[0].Preview != "keep reply" {
		t.Fatalf("preview=%q, want keep reply", metas[0].Preview)
	}
	if metas[0].Rounds != 1 {
		t.Fatalf("rounds=%d, want 1", metas[0].Rounds)
	}

	cm2 := session.NewContextManager()
	rc2 := &RuntimeCore{cm: cm2}
	if err := rc2.ResumeSession(context.Background(), "thread-rollback"); err != nil {
		t.Fatalf("ResumeSession() error = %v", err)
	}
	preview := cm2.BuildPreview()
	if len(preview) != 2 {
		t.Fatalf("len(BuildPreview())=%d, want 2", len(preview))
	}
	if preview[0].Content != "keep this turn" {
		t.Fatalf("preview[0].Content=%q, want keep this turn", preview[0].Content)
	}
	if preview[1].Content != "keep reply" {
		t.Fatalf("preview[1].Content=%q, want keep reply", preview[1].Content)
	}
}

func TestRuntimeCore_CurrentSessionControlsResumeDefault(t *testing.T) {
	setRuntimeSessionTestHome(t)
	dir := t.TempDir()
	old, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(old) })

	rc := &RuntimeCore{cm: session.NewContextManager()}
	if _, err := rc.SaveSessionMessages(context.Background(), "thread-1", []SessionTranscriptMessage{
		{Role: "user", Type: "user", Content: "first"},
		{Role: "assistant", Type: "assistant", Content: "reply one"},
	}); err != nil {
		t.Fatalf("save thread-1 error: %v", err)
	}
	if _, err := rc.SaveSessionMessages(context.Background(), "thread-2", []SessionTranscriptMessage{
		{Role: "user", Type: "user", Content: "second"},
		{Role: "assistant", Type: "assistant", Content: "reply two"},
	}); err != nil {
		t.Fatalf("save thread-2 error: %v", err)
	}
	if err := rc.SetCurrentSession("thread-1"); err != nil {
		t.Fatalf("SetCurrentSession() error = %v", err)
	}

	cm2 := session.NewContextManager()
	rc2 := &RuntimeCore{cm: cm2}
	if err := rc2.ResumeSession(context.Background(), ""); err != nil {
		t.Fatalf("ResumeSession(\"\") error = %v", err)
	}
	preview := cm2.BuildPreview()
	if len(preview) == 0 || preview[0].Content != "first" {
		t.Fatalf("expected current session to resume thread-1, got %#v", preview)
	}

	if err := rc.DeleteSession("thread-1"); err != nil {
		t.Fatalf("DeleteSession(thread-1) error = %v", err)
	}
	currentID, err := rc.CurrentSessionID()
	if err != nil {
		t.Fatalf("CurrentSessionID() error = %v", err)
	}
	if currentID != "thread-2" {
		t.Fatalf("current session after delete = %q, want thread-2", currentID)
	}
}
