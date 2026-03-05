package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestManager_ExecuteStructuredPreservesOrder(t *testing.T) {
	dir := t.TempDir()
	path1 := filepath.Join(dir, "a.txt")
	path2 := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(path1, []byte("a"), 0644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(path2, []byte("b"), 0644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	old, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(old) })

	m := NewManager()
	res := m.ExecuteStructured(context.Background(), []ToolCall{
		{Tool: ToolRead, Parameters: map[string]interface{}{"path": "a.txt"}},
		{Tool: ToolRead, Parameters: map[string]interface{}{"path": "b.txt"}},
	})
	if len(res) != 2 {
		t.Fatalf("unexpected result count: %d", len(res))
	}
	if res[0].Status != "success" || res[1].Status != "success" {
		t.Fatalf("unexpected statuses: %#v", res)
	}
	p0, _ := res[0].Data["path"].(string)
	p1, _ := res[1].Data["path"].(string)
	if filepath.ToSlash(p0) != "a.txt" || filepath.ToSlash(p1) != "b.txt" {
		t.Fatalf("unexpected order: %q %q", p0, p1)
	}
}

func TestManager_ExecuteStructuredCacheAddsMarker(t *testing.T) {
	dir := t.TempDir()
	path1 := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path1, []byte("a"), 0644); err != nil {
		t.Fatalf("write a: %v", err)
	}

	old, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(old) })

	m := NewManager()
	res1 := m.ExecuteStructured(context.Background(), []ToolCall{
		{Tool: ToolRead, Parameters: map[string]interface{}{"path": "a.txt"}},
	})
	if len(res1) != 1 || res1[0].Status != "success" {
		t.Fatalf("unexpected first result: %#v", res1)
	}
	res2 := m.ExecuteStructured(context.Background(), []ToolCall{
		{Tool: ToolRead, Parameters: map[string]interface{}{"path": "a.txt"}},
	})
	if len(res2) != 1 || res2[0].Status != "success" {
		t.Fatalf("unexpected second result: %#v", res2)
	}
	if len(res2[0].Display) < 8 || res2[0].Display[:8] != "[cached]" {
		t.Fatalf("expected cached marker, display=%q", res2[0].Display)
	}
}
