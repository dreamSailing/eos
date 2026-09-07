package lsp

// Copyright (c) 2026 EOSAIOS
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// 语言探测按标志文件优先级：go.mod > Python 标志 > package.json/tsconfig。
func TestDetectLanguageByMarkerFiles(t *testing.T) {
	cases := []struct {
		files []string
		want  LanguageType
	}{
		{[]string{"go.mod"}, LanguageGo},
		{[]string{"pyproject.toml"}, LanguagePython},
		{[]string{"requirements.txt"}, LanguagePython},
		{[]string{"package.json", "tsconfig.json"}, LanguageTypeScript},
		{[]string{"package.json"}, LanguageJavaScript},
	}
	for _, tc := range cases {
		dir := t.TempDir()
		for _, f := range tc.files {
			writeFile(t, filepath.Join(dir, f), "{}")
		}
		if got := NewDetector().DetectLanguage(dir); got != tc.want {
			t.Fatalf("DetectLanguage(%v) = %q; want %q", tc.files, got, tc.want)
		}
	}
}

// 无标志文件时回退到源码扩展名统计（隐藏目录与 node_modules 不计）。
func TestDetectLanguageByExtensionCount(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.py"), "x=1")
	writeFile(t, filepath.Join(dir, "util.py"), "x=2")
	writeFile(t, filepath.Join(dir, "only.go"), "package x")
	if err := os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, ".hidden", "more.go"), "package y")

	if got := NewDetector().DetectLanguage(dir); got != LanguagePython {
		t.Fatalf("DetectLanguage() = %q; want python (2 py > 1 go, .hidden skipped)", got)
	}

	empty := t.TempDir()
	if got := NewDetector().DetectLanguage(empty); got != "" {
		t.Fatalf("DetectLanguage(empty) = %q; want empty", got)
	}
}

// SetEmbedded 后 FindServer 走内嵌分支（默认构建未编入 with_gopls 时
// 报「未实现」，错误信息可证分支选择）；DisableServer 则无条件拦截。
func TestFindServerEmbeddedBranchAndDisable(t *testing.T) {
	d := NewDetector()
	d.SetEmbedded(LanguageGo)

	_, err := d.FindServer(LanguageGo)
	if err == nil {
		t.Skip("with_gopls 构建编入了内嵌 server，本测试针对默认构建的分支选择")
	}
	if !strings.Contains(err.Error(), "embedded server") {
		t.Fatalf("SetEmbedded 后应走内嵌分支；got error: %v", err)
	}

	d.DisableServer(LanguageGo)
	if _, err := d.FindServer(LanguageGo); err == nil {
		t.Fatal("FindServer must fail for disabled language")
	}
	if d.IsServerEnabled(LanguageGo) {
		t.Fatal("IsServerEnabled must be false after DisableServer")
	}
	d.EnableServer(LanguageGo)
	if !d.IsServerEnabled(LanguageGo) {
		t.Fatal("IsServerEnabled must be true after EnableServer")
	}
}

func TestFindServerUnsupportedLanguage(t *testing.T) {
	if _, err := NewDetector().FindServer(LanguageType("cobol")); err == nil {
		t.Fatal("unsupported language must error")
	}
}

func TestDetectorLanguageSupportMatrix(t *testing.T) {
	d := NewDetector()
	for _, lang := range []LanguageType{LanguageGo, LanguagePython, LanguageTypeScript, LanguageJavaScript} {
		if !d.IsLanguageSupported(lang) {
			t.Fatalf("IsLanguageSupported(%q) = false; want true", lang)
		}
	}
	if d.IsLanguageSupported(LanguageType("cobol")) {
		t.Fatal("unsupported language must report false")
	}
}

// DiagnosticStore 的版本化存取、清除与错误计数。
func TestDiagnosticStoreLifecycle(t *testing.T) {
	s := NewDiagnosticStore()
	uri := DocumentURI("file:///a.go")

	s.Set(uri, 3, []Diagnostic{
		{Message: "unused variable", Severity: SeverityWarning},
		{Message: "undefined: Foo", Severity: SeverityError},
	})
	got, ok := s.Get(uri)
	if !ok || got.Version != 3 || len(got.Diagnostics) != 2 {
		t.Fatalf("Get() = %+v,%v; want version 3 with 2 diagnostics", got, ok)
	}
	if n := s.GetErrors(uri); n != 1 {
		t.Fatalf("GetErrors() = %d; want 1", n)
	}

	// 新版本整体覆盖旧版本。
	s.Set(uri, 4, []Diagnostic{{Message: "ok now"}})
	got, _ = s.Get(uri)
	if got.Version != 4 || len(got.Diagnostics) != 1 {
		t.Fatalf("overwrite = %+v; want version 4 with 1 diagnostic", got)
	}

	s.Clear(uri)
	if _, ok := s.Get(uri); ok {
		t.Fatal("Clear must remove the entry")
	}

	s.Set(DocumentURI("file:///b.go"), 1, []Diagnostic{{Severity: SeverityError}})
	if all := s.GetAll(); len(all) != 1 {
		t.Fatalf("GetAll() = %d entries; want 1", len(all))
	}
	s.ClearAll()
	if all := s.GetAll(); len(all) != 0 {
		t.Fatalf("ClearAll left %d entries", len(all))
	}
}
