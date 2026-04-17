package skills

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
