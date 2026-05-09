package codectx

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

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

func TestBuildIndexHonorsEOSIgnoreOnly(t *testing.T) {
	dir := t.TempDir()
	legacyIgnoreName := "." + "vbignore"
	if err := os.WriteFile(filepath.Join(dir, ".eosignore"), []byte("ignored.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, legacyIgnoreName), []byte("visible.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.go"), []byte("package main\nfunc Ignored(){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "visible.go"), []byte("package main\nfunc Visible(){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := NewEngine(dir)
	if err := e.BuildIndex(); err != nil {
		t.Fatalf("build index: %v", err)
	}
	if _, ok := e.Index["ignored.go"]; ok {
		t.Fatalf("expected ignored.go to be excluded by .eosignore")
	}
	if _, ok := e.Index["visible.go"]; !ok {
		t.Fatalf("expected visible.go to remain indexed because the legacy ignore file is ignored")
	}
}
