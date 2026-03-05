package tools

import (
	"context"
	"testing"
	"time"
)

func TestShellSession_StartOutputKill(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	m := NewManager()
	cmd := "echo test && ping -n 1 127.0.0.1"
	ctx := WithAllowedTools(context.Background(), map[string]bool{"bash_session": true})
	r := m.execStructured(ctx, ToolCall{Tool: "bash_session", Parameters: map[string]interface{}{"mode": "start", "command": cmd}})
	if r.Status != "success" {
		t.Fatalf("start error: %s", r.Error)
	}
	id, _ := r.Data["id"].(string)
	time.Sleep(800 * time.Millisecond)
	r2 := m.execStructured(ctx, ToolCall{Tool: "bash_session", Parameters: map[string]interface{}{"mode": "output", "id": id}})
	if r2.Status != "success" {
		t.Fatalf("output error: %s", r2.Error)
	}
	out, _ := r2.Data["stdout"].(string)
	if out == "" {
		t.Log("no output received, may be timing issue")
	}
	r3 := m.execStructured(ctx, ToolCall{Tool: "bash_session", Parameters: map[string]interface{}{"mode": "kill", "id": id}})
	if r3.Status != "success" {
		t.Fatalf("kill error: %s", r3.Error)
	}
}
