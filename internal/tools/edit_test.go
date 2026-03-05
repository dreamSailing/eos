package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEdit_SimpleReplace(t *testing.T) {
	dir, err := os.MkdirTemp(".", "edittest_*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	fp := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(fp, []byte("hello world\nhello WORLD"), 0644); err != nil {
		t.Fatal(err)
	}
	m := NewManager()
	r := m.editStructured(context.Background(), map[string]interface{}{"mode": "single", "file": fp, "find": "hello", "replace": "hi", "limit": 1})
	if r.Status != "success" {
		t.Fatalf("edit error: %s", r.Error)
	}
}

func TestMultiEdit_Mixed(t *testing.T) {
	dir, err := os.MkdirTemp(".", "medittest_*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	fp := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(fp, []byte("line1\nline2\nline3"), 0644); err != nil {
		t.Fatal(err)
	}
	m := NewManager()
	edits := []map[string]interface{}{
		{"find": "line2", "replace": "L2"},
		{"start_line": 3.0, "end_line": 3.0, "replace": "LAST"},
	}
	var arr []interface{}
	for _, e := range edits {
		arr = append(arr, e)
	}
	r := m.editStructured(context.Background(), map[string]interface{}{"mode": "multi", "file": fp, "edits": arr})
	if r.Status != "success" {
		t.Fatalf("edit multi mode error: %s", r.Error)
	}
}
