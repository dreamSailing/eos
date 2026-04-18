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

func TestRegexSearchDirTool(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(wd) }()

	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("Key=1\n"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "b.md"), []byte("key=2\n"), 0644)

	m := NewManager()
	r := m.execStructured(context.Background(), ToolCall{Tool: "search", Parameters: map[string]interface{}{"mode": "regex", "pattern": "(?i)key=\\d+", "context": 0}})
	if r.Status != "success" {
		t.Fatalf("exec failed: %+v", r)
	}
}

func TestTextSearchFileTool(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(wd) }()

	p := filepath.Join(dir, "c.txt")
	_ = os.WriteFile(p, []byte("Hello\nworld\n"), 0644)
	m := NewManager()
	r := m.execStructured(context.Background(), ToolCall{Tool: "search", Parameters: map[string]interface{}{"mode": "text", "pattern": "world", "root": dir, "context": 0}})
	if r.Status != "success" {
		t.Fatalf("exec failed: %+v", r)
	}
}
