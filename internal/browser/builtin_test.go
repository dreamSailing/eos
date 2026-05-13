package browser

import (
	"context"
	"strings"
	"testing"
)

func TestBuiltinRuntimeStatusReadyByDefault(t *testing.T) {
	rt := NewBuiltinRuntime()
	status := rt.Status()
	if !status.Ready {
		t.Fatalf("builtin runtime should be ready by default: %+v", status)
	}
	if got := strings.Join(status.Capabilities, ","); got != "navigate,snapshot,wait,network" {
		t.Fatalf("capabilities = %q", got)
	}
}

func TestBuiltinRuntimeTraceLifecycle(t *testing.T) {
	rt := NewBuiltinRuntime()
	rt.StartTrace("trace-1")
	if rt.SessionCount() != 1 {
		t.Fatalf("session count = %d, want 1", rt.SessionCount())
	}
	rt.ReleaseTrace("trace-1")
	if rt.SessionCount() != 0 {
		t.Fatalf("session count = %d, want 0", rt.SessionCount())
	}
}

func TestBuiltinSessionUnsupportedAction(t *testing.T) {
	rt := NewBuiltinRuntime()
	sess, err := rt.Session("trace-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sess.Click(context.Background(), ClickRequest{Selector: "#submit"}); err == nil {
		t.Fatal("expected unsupported click error")
	} else if !strings.Contains(err.Error(), "DOM-capable backend") {
		t.Fatalf("unexpected error: %v", err)
	}
}
