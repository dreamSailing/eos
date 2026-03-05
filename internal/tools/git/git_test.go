package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestStatusBranchList(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	root := t.TempDir()
	o := &Ops{Root: root}
	if _, err := o.Init(); err != nil {
		t.Fatalf("init error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file error: %v", err)
	}
	if _, err := o.Add([]string{"a.txt"}); err != nil {
		t.Fatalf("add error: %v", err)
	}
	if _, err := o.Commit("init", "", ""); err != nil {
		t.Fatalf("commit error: %v", err)
	}

	_, err := o.Status()
	if err != nil {
		t.Fatalf("status error: %v", err)
	}
	branches, current, err := o.BranchList()
	if err != nil {
		t.Fatalf("branches error: %v", err)
	}
	if len(branches) == 0 {
		t.Fatalf("expected branches, got empty")
	}
	if current == "" {
		t.Fatalf("expected current branch, got empty")
	}
}
