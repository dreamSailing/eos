//go:build legacy

package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/dreamSailing/eos/internal/pkg/utils"
	gitops "github.com/dreamSailing/eos/internal/tools/git"
)

// EnterWorktree creates a new git worktree and optionally updates the working root
func (rc *RuntimeCore) EnterWorktree(ctx context.Context, name string) (string, error) {
	if name == "" {
		name = fmt.Sprintf("wt-%d", os.Getpid())
	}

	root := rc.workingRoot()
	if strings.TrimSpace(root) == "" {
		root, _ = os.Getwd()
	}
	ops := gitops.NewOpsWithRoot(root)
	if err := rc.ensureSandboxWrite(ctx, ops.Root); err != nil {
		return "", err
	}
	worktreesDir := filepath.Join(root, ".eos", "worktrees")
	targetPath := filepath.Join(worktreesDir, name)
	if err := rc.ensureSandboxWrite(ctx, targetPath); err != nil {
		return "", err
	}

	policy := rc.sandboxPolicy(ctx)
	result := rc.guardedGitCmd(ctx, policy, ops.Root, "worktree", "add", targetPath)
	if result.Err != nil {
		slog.Error("bridge.worktree.create_failed",
			"component", utils.ComponentSystem,
			"error", result.Err,
			"output", result.Stderr,
		)
		return "", fmt.Errorf("git worktree add failed: %s", result.Stderr)
	}

	absPath, _ := filepath.Abs(targetPath)
	slog.Info("bridge.worktree.created",
		"component", utils.ComponentSystem,
		"path", absPath,
	)

	// Emit worktree event
	rc.eventsCh <- Event{
		Type:    "worktree.created",
		Content: "worktree created",
		Data:    map[string]any{"path": absPath, "name": name},
	}

	return absPath, nil
}

// ExitWorktree removes a git worktree
func (rc *RuntimeCore) ExitWorktree(ctx context.Context, path string, remove bool) error {
	root := rc.workingRoot()
	if strings.TrimSpace(root) == "" {
		root, _ = os.Getwd()
	}
	path = strings.TrimSpace(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}
	ops := gitops.NewOpsWithRoot(root)
	if err := rc.ensureSandboxWrite(ctx, ops.Root); err != nil {
		return err
	}
	if err := rc.ensureSandboxWrite(ctx, path); err != nil {
		return err
	}

	policy := rc.sandboxPolicy(ctx)
	if remove {
		result := rc.guardedGitCmd(ctx, policy, ops.Root, "worktree", "remove", path)
		if result.Err != nil {
			return fmt.Errorf("git worktree remove failed: %s", result.Stderr)
		}
	} else {
		result := rc.guardedGitCmd(ctx, policy, ops.Root, "worktree", "remove", "--force", path)
		if result.Err != nil {
			slog.Warn("bridge.worktree.force_remove",
				"component", utils.ComponentSystem,
				"error", result.Err,
				"output", result.Stderr,
			)
		}
	}

	rc.eventsCh <- Event{
		Type:    "worktree.removed",
		Content: "worktree removed",
		Data:    map[string]any{"path": path},
	}

	return nil
}

func (rc *RuntimeCore) ensureSandboxWrite(ctx context.Context, target string) error {
	policy := rc.sandboxPolicy(ctx)
	ok, err := policy.AllowsWrite(target)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return fmt.Errorf("sandbox policy %s blocks writes outside workspace: %s", policy.Mode, filepath.ToSlash(target))
}
