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

func TestCreateSkillToolCreatesWorkspaceSkillAndReloads(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("EOS_API_KEY", "")
	t.Setenv("EOS_API_BASE", "")
	t.Setenv("EOS_MODEL", "")

	mgr, sm := newCreateSkillTestManager(t, workspace, home)
	ctx := WithWorkspaceRoot(context.Background(), workspace)

	results := mgr.ExecuteStructured(ctx, []ToolCall{
		{
			Tool: ToolCreateSkill,
			Parameters: map[string]any{
				"name":               "repo-review",
				"request":            "创建一个用于当前仓库代码审查的 skill，重点关注兼容性、迁移和测试缺失。",
				"scope":              "workspace",
				"include_scripts":    true,
				"include_references": true,
				"include_assets":     true,
			},
		},
	})
	if len(results) != 1 {
		t.Fatalf("results=%d want=1", len(results))
	}
	if results[0].Status != "success" {
		t.Fatalf("create_skill status=%q error=%q", results[0].Status, results[0].Error)
	}
	if usedAI, _ := results[0].Data["used_ai_generation"].(bool); usedAI {
		t.Fatalf("expected fallback generation in test")
	}

	skillPath := filepath.Join(workspace, ".eos", "skills", "repo-review", "SKILL.md")
	raw, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "name: repo-review") {
		t.Fatalf("skill doc missing normalized name:\n%s", text)
	}
	if !strings.Contains(text, "何时使用") && !strings.Contains(text, "When To Use") {
		t.Fatalf("skill doc missing body:\n%s", text)
	}
	for _, subdir := range []string{"scripts", "references", "assets"} {
		if info, err := os.Stat(filepath.Join(workspace, ".eos", "skills", "repo-review", subdir)); err != nil || !info.IsDir() {
			t.Fatalf("expected subdir %s to exist, err=%v", subdir, err)
		}
	}
	if !sm.IsActive("repo-review") {
		// create_skill should reload, not activate
		if _, ok := sm.Get("repo-review"); !ok {
			t.Fatalf("expected skill manager to see created skill after reload")
		}
	}

	listResults := mgr.ExecuteStructured(ctx, []ToolCall{{Tool: ToolSkillsList, Parameters: map[string]any{}}})
	if len(listResults) != 1 || listResults[0].Status != "success" {
		t.Fatalf("skills_list failed: %#v", listResults)
	}
	names, _ := listResults[0].Data["names"].([]string)
	if len(names) == 0 || names[0] != "repo-review" {
		t.Fatalf("skills_list names=%v", names)
	}
}

func TestCreateSkillToolRequiresExplicitScope(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	mgr, _ := newCreateSkillTestManager(t, workspace, home)

	results := mgr.ExecuteStructured(WithWorkspaceRoot(context.Background(), workspace), []ToolCall{
		{
			Tool: ToolCreateSkill,
			Parameters: map[string]any{
				"request": "创建一个通用的 release notes skill",
			},
		},
	})
	if len(results) != 1 {
		t.Fatalf("results=%d want=1", len(results))
	}
	if results[0].Status != "error" {
		t.Fatalf("status=%q want error", results[0].Status)
	}
	if !strings.Contains(results[0].Error, "scope parameter is required") {
		t.Fatalf("unexpected error: %q", results[0].Error)
	}
}

func TestCreateSkillToolSupportsUserScopeAndOverwrite(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("EOS_API_KEY", "")
	t.Setenv("EOS_API_BASE", "")
	t.Setenv("EOS_MODEL", "")

	mgr, _ := newCreateSkillTestManager(t, workspace, home)
	ctx := WithWorkspaceRoot(context.Background(), workspace)
	params := map[string]any{
		"name":     "release-notes",
		"request":  "创建一个通用的 release notes 生成 skill，可跨项目复用。",
		"scope":    "user",
		"activate": true,
	}

	first := mgr.ExecuteStructured(ctx, []ToolCall{{Tool: ToolCreateSkill, Parameters: params}})
	if len(first) != 1 || first[0].Status != "success" {
		t.Fatalf("first create failed: %#v", first)
	}
	skillPath := filepath.Join(home, ".eos", "skills", "release-notes", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected user scope skill to exist: %v", err)
	}

	second := mgr.ExecuteStructured(ctx, []ToolCall{{Tool: ToolCreateSkill, Parameters: params}})
	if len(second) != 1 || second[0].Status != "error" {
		t.Fatalf("expected overwrite=false create to fail: %#v", second)
	}
	if !strings.Contains(second[0].Error, "already exists") {
		t.Fatalf("unexpected overwrite error: %q", second[0].Error)
	}

	params["overwrite"] = true
	third := mgr.ExecuteStructured(ctx, []ToolCall{{Tool: ToolCreateSkill, Parameters: params}})
	if len(third) != 1 || third[0].Status != "success" {
		t.Fatalf("expected overwrite=true create to succeed: %#v", third)
	}
}

func newCreateSkillTestManager(t *testing.T, workspace, home string) (*Manager, *SkillManager) {
	t.Helper()
	mgr := NewManager()
	loader := skills.NewLoader()
	loader.SetSkillsDirs([]string{
		filepath.Join(workspace, ".eos", "skills"),
		filepath.Join(home, ".eos", "skills"),
	})
	sm := NewSkillManager(loader, mgr)
	mgr.SetSkillManager(sm)
	return mgr, sm
}
