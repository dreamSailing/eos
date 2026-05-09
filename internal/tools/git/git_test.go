package git

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestShow(t *testing.T) {
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

	out, err := o.Show("HEAD", "")
	if err != nil {
		t.Fatalf("show error: %v", err)
	}
	if strings.TrimSpace(out.Revision) != "HEAD" {
		t.Fatalf("unexpected revision: %q", out.Revision)
	}
	if !strings.Contains(out.Text, "init") {
		t.Fatalf("expected show output contains commit message, got: %q", out.Text)
	}

	outPath, err := o.Show("HEAD", "a.txt")
	if err != nil {
		t.Fatalf("show with path error: %v", err)
	}
	if !strings.Contains(outPath.Text, "a.txt") {
		t.Fatalf("expected show output contains file path, got: %q", outPath.Text)
	}
}
