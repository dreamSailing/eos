package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGetDependenciesOnEngineGo(t *testing.T) {
	mgr := NewManager()
	wd, _ := os.Getwd()
	target := filepath.Join(wd, "manager.go")
	if _, err := os.Stat(target); err != nil {
		t.Skip("manager.go not found")
	}
	r := mgr.execStructured(context.Background(), ToolCall{Tool: "search", Parameters: map[string]interface{}{"mode": "deps", "pattern": target, "depth": 1, "limit": 100}})
	if r.Status != "success" {
		t.Fatalf("get_dependencies failed: %v", r.Error)
	}
	if r.Data["degree"] == nil {
		t.Fatalf("missing degree")
	}
}

func TestAnalyzeImportGraphRoot(t *testing.T) {
	mgr := NewManager()
	wd, _ := os.Getwd()
	r := mgr.execStructured(context.Background(), ToolCall{Tool: "search", Parameters: map[string]interface{}{"mode": "graph", "pattern": "graph", "root": wd, "limit": 10}})
	if r.Status != "success" {
		t.Fatalf("analyze_import_graph failed: %v", r.Error)
	}
	if r.Data["files_total"] == nil {
		t.Fatalf("unexpected data payload")
	}
}
