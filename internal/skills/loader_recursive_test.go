package skills

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSkill(t *testing.T, dir, name, desc string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "---\nname: " + name + "\ndescription: " + desc + "\n---\n\nbody"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}

func TestLoaderScanRecursiveLoadsNestedSkills(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "skill-a"), "skill-a", "a")
	writeSkill(t, filepath.Join(root, "nested", "skill-b"), "skill-b", "b")

	l := NewLoader()
	l.SetSkillsDirs([]string{root})
	if err := l.Scan(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	if _, ok := l.Get("skill-a"); !ok {
		t.Fatalf("expected skill-a")
	}
	if _, ok := l.Get("skill-b"); !ok {
		t.Fatalf("expected skill-b")
	}
}

func TestLoaderUserOverridesProjectForSameName(t *testing.T) {
	project := t.TempDir()
	writeSkill(t, filepath.Join(project, "same"), "same", "project")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home: %v", err)
	}

	base := filepath.Join(home, ".eos", "skills")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir base: %v", err)
	}
	userRoot, err := os.MkdirTemp(base, "vbskills_test_")
	if err != nil {
		t.Fatalf("mktemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(userRoot) })
	writeSkill(t, filepath.Join(userRoot, "same"), "same", "user")

	l := NewLoader()
	l.SetSkillsDirs([]string{project, userRoot})
	if err := l.Scan(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	s, ok := l.Get("same")
	if !ok || s == nil {
		t.Fatalf("expected same")
	}
	if s.Description != "user" {
		t.Fatalf("expected user override, got %q", s.Description)
	}
}

func TestLoaderGetStatsIncludesNamesWithoutAliasing(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "skill-a"), "skill-a", "a")
	writeSkill(t, filepath.Join(root, "skill-b"), "skill-b", "b")

	l := NewLoader()
	l.SetSkillsDirs([]string{root})
	if err := l.Scan(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	stats := l.GetStats()
	total, ok := stats["total_skills"].(int)
	if !ok || total < 2 {
		t.Fatalf("total_skills = %v, want at least 2", stats["total_skills"])
	}
	names, ok := stats["names"].([]string)
	if !ok || len(names) < 2 {
		t.Fatalf("names = %#v, want at least 2 names", stats["names"])
	}
	if !containsString(names, "skill-a") || !containsString(names, "skill-b") {
		t.Fatalf("names = %#v, want skill-a and skill-b", names)
	}
	scanDirs, ok := stats["scan_dirs"].([]string)
	if !ok || len(scanDirs) != 1 || scanDirs[0] != root {
		t.Fatalf("scan_dirs = %#v, want [%q]", stats["scan_dirs"], root)
	}

	scanDirs[0] = "mutated"
	gotDirs := l.GetSkillsDirs()
	if len(gotDirs) != 1 || gotDirs[0] != root {
		t.Fatalf("GetSkillsDirs() = %#v, want [%q]", gotDirs, root)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
