package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGlob_FindFiles(t *testing.T) {
	dir, err := os.MkdirTemp(".", "globtest_*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	// create files
	fs := []string{"a.go", "b_test.go", "c.txt"}
	for _, f := range fs {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	m := NewManager()
	r := m.execStructured(context.Background(), ToolCall{Tool: "search", Parameters: map[string]interface{}{"mode": "glob", "pattern": "*.go", "root": dir}})
	if r.Status != "success" {
		t.Fatalf("glob error: %s", r.Error)
	}
	arr, ok := r.Data["matches"].([]string)
	if !ok || len(arr) == 0 {
		t.Fatalf("expected matches, got none")
	}
}

func TestBash_RunEcho(t *testing.T) {
	m := NewManager()
	ctx := WithAllowedTools(context.Background(), map[string]bool{"bash": true})
	r := m.execStructured(ctx, ToolCall{Tool: "bash", Parameters: map[string]interface{}{"command": "echo test"}})
	if r.Status != "success" {
		t.Fatalf("bash error: %s", r.Error)
	}
	if r.Display == "" {
		t.Fatalf("expected output display")
	}
}

func TestBash_PermissionDeniedWhenToolNotAllowed(t *testing.T) {
	m := NewManager()
	ctx := WithAllowedTools(context.Background(), map[string]bool{"search": true})
	r := m.execStructured(ctx, ToolCall{Tool: "bash", Parameters: map[string]interface{}{"command": "echo test"}})
	if r.Status != "error" {
		t.Fatalf("expected error, got: %s", r.Status)
	}
}

func TestBash_CommandSanitized(t *testing.T) {
	m := NewManager()
	ctx := WithAllowedTools(context.Background(), map[string]bool{"bash": true})
	r := m.execStructured(ctx, ToolCall{Tool: "bash", Parameters: map[string]interface{}{"command": "echo $(Get-Date)"}})
	if r.Status != "error" {
		t.Fatalf("expected error, got: %s", r.Status)
	}
}

func TestToolPermissionFilterByContext(t *testing.T) {
	m := NewManager()
	ctx := WithRole(context.Background(), "senior-dev")
	ctx = WithAllowedTools(ctx, map[string]bool{"search": true})
	res := m.ExecuteStructured(ctx, []ToolCall{{Tool: "bash", Parameters: map[string]interface{}{"command": "echo test"}}})
	if len(res) == 0 {
		t.Fatalf("expected result")
	}
	if res[0].Status != "error" {
		t.Fatalf("expected permission denied, got: %s", res[0].Status)
	}
}
