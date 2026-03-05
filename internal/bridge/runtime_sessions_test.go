package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dreamSailing/vb-coding/internal/session"
)

func TestRuntimeCore_SaveAndResumeSession(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(old) })

	cm := session.NewContextManager()
	cm.AddUser("hello")
	cm.AddAssistant("world")

	rc := &RuntimeCore{cm: cm}
	rc.tokenHistory = []TokenRecord{{Total: 123}}

	id, err := rc.SaveSession(context.Background(), "test_session")
	if err != nil {
		t.Fatalf("save error: %v", err)
	}
	if id != "test_session" {
		t.Fatalf("unexpected id: %s", id)
	}
	if _, err := os.Stat(filepath.Join(dir, ".vb", "sessions", "test_session.json")); err != nil {
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
