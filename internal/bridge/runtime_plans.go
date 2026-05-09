package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	workspaceCurrentPlanFile = "current-plan.md"
	userLatestPlanFile       = "latest.md"
)

type planPersistPaths struct {
	WorkspaceCurrent string
	UserLatest       string
	UserSnapshot     string
}

// PlanSnapshot is a compact, GUI-friendly view of the latest plan and the
// files where the CLI persists it.
type PlanSnapshot struct {
	HasPlan          bool
	Content          string
	WorkspaceCurrent string
	UserLatest       string
	UserSnapshot     string
	UpdatedAt        time.Time
}

func planUserBaseDir() string {
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".eos", "plans")
	}
	if wd, err := os.Getwd(); err == nil && strings.TrimSpace(wd) != "" {
		return filepath.Join(filepath.Clean(wd), ".eos", "plans")
	}
	return filepath.Join(".eos", "plans")
}

func planWorkspaceNamespace(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return "default"
	}
	normalized := strings.ToLower(filepath.ToSlash(filepath.Clean(root)))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:8])
}

func planDateDir(ts time.Time) string {
	if ts.IsZero() {
		ts = time.Now()
	}
	return filepath.Join(
		fmt.Sprintf("%04d", ts.Year()),
		fmt.Sprintf("%02d", int(ts.Month())),
		fmt.Sprintf("%02d", ts.Day()),
	)
}

func planSlug(plan string) string {
	lines := strings.Split(strings.ReplaceAll(plan, "\r\n", "\n"), "\n")
	candidate := "plan"
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			line = strings.TrimSpace(strings.TrimLeft(line, "#"))
			if line != "" {
				candidate = line
				break
			}
		}
	}
	candidate = strings.ToLower(candidate)
	var b strings.Builder
	lastDash := false
	for _, r := range candidate {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "plan"
	}
	return out
}

func planPaths(root string, ts time.Time, plan string) planPersistPaths {
	dateDir := planDateDir(ts)
	slug := planSlug(plan)
	namespace := planWorkspaceNamespace(root)
	userDir := filepath.Join(planUserBaseDir(), dateDir, namespace)
	stamp := ts.Format("20060102-150405")
	workspaceCurrent := ""
	root = strings.TrimSpace(root)
	if root != "" {
		workspaceCurrent = filepath.Join(root, ".trae", "documents", workspaceCurrentPlanFile)
	}
	return planPersistPaths{
		WorkspaceCurrent: workspaceCurrent,
		UserLatest:       filepath.Join(userDir, userLatestPlanFile),
		UserSnapshot:     filepath.Join(userDir, fmt.Sprintf("%s-%s.md", stamp, slug)),
	}
}

func ensureParentDir(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

func writeTextFile(path string, data string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := ensureParentDir(path); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(data), 0o644)
}

func readTextFileIfExists(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

func persistPlanArtifacts(root string, ts time.Time, plan string) (planPersistPaths, bool, error) {
	plan = strings.TrimSpace(plan)
	paths := planPaths(root, ts, plan)
	if plan == "" {
		return paths, false, nil
	}
	latestContent, err := readTextFileIfExists(paths.UserLatest)
	if err != nil {
		return paths, false, err
	}
	changed := strings.TrimSpace(latestContent) != plan
	if paths.WorkspaceCurrent != "" {
		if err := writeTextFile(paths.WorkspaceCurrent, plan+"\n"); err != nil {
			return paths, false, err
		}
	}
	if err := writeTextFile(paths.UserLatest, plan+"\n"); err != nil {
		return paths, false, err
	}
	if changed {
		if err := writeTextFile(paths.UserSnapshot, plan+"\n"); err != nil {
			return paths, false, err
		}
	}
	return paths, changed, nil
}

func (rc *RuntimeCore) PersistPlan(plan string) error {
	if rc == nil {
		return nil
	}
	_, _, err := rc.persistPlanAt(time.Now(), plan)
	return err
}

func (rc *RuntimeCore) persistPlanAt(ts time.Time, plan string) (planPersistPaths, bool, error) {
	root := ""
	if rc != nil {
		root = rc.workingRoot()
	}
	return persistPlanArtifacts(root, ts, plan)
}

func (rc *RuntimeCore) PlanSnapshot() PlanSnapshot {
	if rc == nil {
		return PlanSnapshot{}
	}
	plan := strings.TrimSpace(rc.cm.LastPlan())
	if plan == "" {
		plan = strings.TrimSpace(rc.lastPlanStored)
	}
	now := time.Now()
	result := PlanSnapshot{
		HasPlan:   plan != "",
		Content:   plan,
		UpdatedAt: now,
	}
	if plan == "" {
		return result
	}
	paths := planPaths(rc.workingRoot(), now, plan)
	result.WorkspaceCurrent = paths.WorkspaceCurrent
	result.UserLatest = paths.UserLatest
	result.UserSnapshot = paths.UserSnapshot
	if info, err := os.Stat(paths.WorkspaceCurrent); err == nil {
		result.UpdatedAt = info.ModTime()
	}
	return result
}
