package codectx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildIndexAndSuggest(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(wd) }()

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nimport \"fmt\"\nfunc Hello(){fmt.Println(\"hi\")}"), 0644); err != nil {
		t.Fatal(err)
	}
	e := NewEngine(dir)
	if err := e.BuildIndex(); err != nil {
		t.Fatalf("build index: %v", err)
	}
	if len(e.Index) == 0 {
		t.Fatalf("index empty")
	}
	s := e.Suggest("Hello", 5)
	if len(s) == 0 {
		t.Fatalf("suggest empty")
	}
}
