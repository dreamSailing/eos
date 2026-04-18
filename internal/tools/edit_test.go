package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


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
