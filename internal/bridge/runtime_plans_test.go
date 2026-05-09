package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	codectx "github.com/dreamSailing/eos/internal/context"
)

func TestPlanUserBaseDirFallsBackToHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	got := planUserBaseDir()
	want := filepath.Join(home, ".eos", "plans")
	if got != want {
		t.Fatalf("planUserBaseDir() = %q, want %q", got, want)
	}
}

func TestPlanDateDir(t *testing.T) {
	ts := time.Date(2026, time.May, 9, 10, 11, 12, 0, time.UTC)
	got := filepath.ToSlash(planDateDir(ts))
	if got != "2026/05/09" {
		t.Fatalf("planDateDir() = %q, want %q", got, "2026/05/09")
	}
}

func TestPlanWorkspaceNamespaceDiffersByRoot(t *testing.T) {
	a := planWorkspaceNamespace("/tmp/project-a")
	b := planWorkspaceNamespace("/tmp/project-b")
	if a == "" || b == "" {
		t.Fatalf("expected non-empty namespace ids")
	}
	if a == b {
		t.Fatalf("expected distinct namespace ids for different roots")
	}
}

func TestPersistPlanArtifactsWritesWorkspaceAndUserCopies(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	root := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll(root) error = %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	ts := time.Date(2026, time.May, 9, 10, 11, 12, 0, time.UTC)
	plan := "# Alpha Plan\n\n- first"

	paths, changed, err := persistPlanArtifacts(root, ts, plan)
	if err != nil {
		t.Fatalf("persistPlanArtifacts() error = %v", err)
	}
	if !changed {
		t.Fatalf("expected first persist to be treated as changed")
	}
	for _, p := range []string{paths.WorkspaceCurrent, paths.UserLatest, paths.UserSnapshot} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected %q to exist, err=%v", p, err)
		}
	}
	workspaceContent, err := os.ReadFile(paths.WorkspaceCurrent)
	if err != nil {
		t.Fatalf("ReadFile(workspace) error = %v", err)
	}
	if strings.TrimSpace(string(workspaceContent)) != plan {
		t.Fatalf("workspace content mismatch: %q", string(workspaceContent))
	}
	latestContent, err := os.ReadFile(paths.UserLatest)
	if err != nil {
		t.Fatalf("ReadFile(latest) error = %v", err)
	}
	if strings.TrimSpace(string(latestContent)) != plan {
		t.Fatalf("latest content mismatch: %q", string(latestContent))
	}
}

func TestPersistPlanArtifactsUpdatesLatestAndSkipsDuplicateSnapshot(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	root := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll(root) error = %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	ts1 := time.Date(2026, time.May, 9, 10, 11, 12, 0, time.UTC)
	ts2 := ts1.Add(2 * time.Minute)
	ts3 := ts2.Add(2 * time.Minute)
	plan1 := "# Alpha Plan\n\n- first"
	plan2 := "# Alpha Plan\n\n- second"

	paths1, changed1, err := persistPlanArtifacts(root, ts1, plan1)
	if err != nil {
		t.Fatalf("persist 1 error = %v", err)
	}
	if !changed1 {
		t.Fatalf("expected first persist to change")
	}
	paths2, changed2, err := persistPlanArtifacts(root, ts2, plan2)
	if err != nil {
		t.Fatalf("persist 2 error = %v", err)
	}
	if !changed2 {
		t.Fatalf("expected second persist to change")
	}
	if paths1.UserSnapshot == paths2.UserSnapshot {
		t.Fatalf("expected different snapshot paths across timestamps")
	}
	if _, err := os.Stat(paths1.UserSnapshot); err != nil {
		t.Fatalf("first snapshot missing: %v", err)
	}
	if _, err := os.Stat(paths2.UserSnapshot); err != nil {
		t.Fatalf("second snapshot missing: %v", err)
	}

	paths3, changed3, err := persistPlanArtifacts(root, ts3, plan2)
	if err != nil {
		t.Fatalf("persist 3 error = %v", err)
	}
	if changed3 {
		t.Fatalf("expected duplicate content not to be treated as changed")
	}
	if _, err := os.Stat(paths3.UserSnapshot); !os.IsNotExist(err) {
		t.Fatalf("expected no duplicate snapshot, err=%v", err)
	}
	latestContent, err := os.ReadFile(paths3.UserLatest)
	if err != nil {
		t.Fatalf("ReadFile(latest) error = %v", err)
	}
	if strings.TrimSpace(string(latestContent)) != plan2 {
		t.Fatalf("latest content = %q, want %q", string(latestContent), plan2)
	}
}

func TestRuntimeCoreHandlePlanUpdateDeduplicates(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	root := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll(root) error = %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	mgr := codectx.NewMultiEngine()
	mgr.AddRoot(root)
	mgr.SetActive(root)
	rc := &RuntimeCore{workspaceMgr: mgr}

	rc.HandlePlanUpdate("# Plan\n\n- item")
	rc.HandlePlanUpdate("# Plan\n\n- item")

	dateDir := planDateDir(time.Now())
	namespace := planWorkspaceNamespace(root)
	base := filepath.Join(planUserBaseDir(), dateDir, namespace)
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatalf("ReadDir(base) error = %v", err)
	}
	snapshotCount := 0
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == userLatestPlanFile {
			continue
		}
		snapshotCount++
	}
	if snapshotCount != 1 {
		t.Fatalf("snapshot count = %d, want 1", snapshotCount)
	}
}
