package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dreamSailing/eos/internal/pkg/utils"
	"log/slog"
)

// EnterWorktree creates a new git worktree and optionally updates the working root
func (rc *RuntimeCore) EnterWorktree(ctx context.Context, name string) (string, error) {
	if name == "" {
		name = fmt.Sprintf("wt-%d", os.Getpid())
	}

	worktreesDir := ".eos/worktrees"
	targetPath := filepath.Join(worktreesDir, name)

	cmd := utils.CommandContext(ctx, "git", "worktree", "add", targetPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("bridge.worktree.create_failed",
			"component", utils.ComponentSystem,
			"error", err,
			"output", string(output),
		)
		return "", fmt.Errorf("git worktree add failed: %s", string(output))
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
	if remove {
		cmd := utils.CommandContext(ctx, "git", "worktree", "remove", path)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git worktree remove failed: %s", string(output))
		}
	} else {
		cmd := utils.CommandContext(ctx, "git", "worktree", "remove", "--force", path)
		if output, err := cmd.CombinedOutput(); err != nil {
			slog.Warn("bridge.worktree.force_remove",
				"component", utils.ComponentSystem,
				"error", err,
				"output", string(output),
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
