package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dreamSailing/eos/internal/skills"
)

func TestSkillBangCommandPreprocess(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "bang-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	md := "---\nname: bang-skill\ndescription: test\n---\n\nHello !`echo hi`"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := skills.NewLoader()
	loader.SetSkillsDirs([]string{root})
	if err := loader.Scan(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	mgr := NewManager()
	sm := NewSkillManager(loader, mgr)
	msgs, _, err := sm.InjectSkillWithArguments(context.Background(), "bang-skill", "")
	if err != nil {
		t.Fatalf("inject: %v", err)
	}

	found := false
	for _, m := range msgs {
		if m.IsMeta {
			if containsAll(m.Content, []string{"Hello", "hi"}) {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected rendered prompt to include command output")
	}
}

func containsAll(s string, subs []string) bool {
	for _, x := range subs {
		if x == "" {
			continue
		}
		if !strings.Contains(s, x) {
			return false
		}
	}
	return true
}
