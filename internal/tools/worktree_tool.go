package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dreamSailing/eos/internal/pkg/utils"
	gitops "github.com/dreamSailing/eos/internal/tools/git"
)

// enterWorktreeStructured handles the enter_worktree tool
func (m *Manager) enterWorktreeStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	name, _ := params["name"].(string)
	if name == "" {
		name = fmt.Sprintf("worktree-%d", os.Getpid())
	}

	root := WorkspaceRootFromContext(ctx)
	if strings.TrimSpace(root) == "" {
		root, _ = os.Getwd()
	}
	ops := gitops.NewOpsWithRoot(root)
	if r, blocked := gitMutationSandboxResult(ctx, ToolEnterWorktree, ops.Root); blocked {
		return r
	}

	// Check if we're in a git repo
	if _, err := os.Stat(filepath.Join(ops.Root, ".git")); err != nil {
		// Check if parent has .git
		if _, err := exec.LookPath("git"); err != nil {
			return ToolResult{
				Type:    "tool_result",
				Tool:    ToolEnterWorktree,
				Status:  "error",
				Error:   "not in a git repository and git not found",
				Display: "错误：enter_worktree 需要 git 仓库",
			}
		}
	}

	// Create worktrees directory
	worktreesDir := filepath.Join(root, ".eos", "worktrees")
	targetPath := filepath.Join(worktreesDir, name)
	if err := sandboxWriteError(ctx, targetPath); err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolEnterWorktree, Status: "error", Error: err.Error(), Display: "错误：" + err.Error()}
	}
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolEnterWorktree,
			Status:  "error",
			Error:   fmt.Sprintf("failed to create worktrees directory: %s", err),
			Display: fmt.Sprintf("错误：创建 worktrees 目录失败：%s", err),
		}
	}

	// Create git worktree
	cmd := utils.CommandContext(ctx, "git", "worktree", "add", targetPath)
	cmd.Dir = ops.Root
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolEnterWorktree,
			Status:  "error",
			Error:   fmt.Sprintf("git worktree add failed: %s (%s)", err, string(output)),
			Display: fmt.Sprintf("错误：创建 worktree 失败：%s", string(output)),
		}
	}

	absPath, _ := filepath.Abs(targetPath)

	// Notify via callback
	if OnWorktreeChange != nil {
		OnWorktreeChange("created", absPath)
	}

	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolEnterWorktree,
		Status: "success",
		Data: map[string]interface{}{
			"path": absPath,
			"name": name,
		},
		Display: fmt.Sprintf("已创建 worktree：%s", absPath),
	}
}

// OnWorktreeChange is called when a worktree is created or removed
var OnWorktreeChange func(event, path string)

// IsGitRepo checks if the current directory is inside a git repository
func IsGitRepo() bool {
	cmd := utils.Command("git", "rev-parse", "--is-inside-work-tree")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}
