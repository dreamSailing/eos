package search

// Copyright (c) 2026 EOSAIOS
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, name, content string) string {
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestDirRegexFlagsAndGroups(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.txt", "Key=123\nkey=456\nX\n")
	write(t, dir, "b.md", "foo\nbar\nKey=789\n")
	opts := Options{Root: dir, Flags: "i", ContextLines: 1}
	rs, trunc, err := DirRegex("(key)=(\\d+)", opts)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if trunc {
		t.Fatalf("unexpected trunc")
	}
	if len(rs) != 3 {
		t.Fatalf("want 3 got %d", len(rs))
	}
	allowed := map[string]bool{"Key=123": true, "key=456": true, "Key=789": true}
	if !allowed[rs[0].Match] {
		t.Fatalf("match not expected: %q", rs[0].Match)
	}
	if len(rs[0].Groups) != 2 {
		t.Fatalf("groups count")
	}
	if len(rs[0].Before) > 1 || len(rs[0].After) > 1 {
		t.Fatalf("context lines")
	}
}

func TestFileRegexMultiline(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "c.go", "package x\nfunc X(){}\n/*A\nB*/\n")
	rs, trunc, err := FileRegex(p, "(?s)/\\*A.*B\\*/", Options{ContextLines: 0})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if trunc {
		t.Fatalf("unexpected trunc")
	}
	if len(rs) != 1 {
		t.Fatalf("want 1 got %d", len(rs))
	}
}

func TestDirTextCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "d.txt", "Hello\nworld\n")
	write(t, dir, "e.txt", "hELLo\n")
	rs, trunc, err := DirText("hello", Options{Root: dir, CaseInsensitive: true})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if trunc {
		t.Fatalf("unexpected trunc")
	}
	if len(rs) != 2 {
		t.Fatalf("want 2 got %d", len(rs))
	}
}

func TestIncludesExcludes(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "src/a.go", "TODO: a\n")
	write(t, dir, "vendor/b.go", "TODO: b\n")
	rs, _, err := DirText("TODO", Options{Root: dir, Includes: []string{".go"}, Excludes: []string{"vendor"}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(rs) != 1 {
		t.Fatalf("want 1 got %d", len(rs))
	}
}
