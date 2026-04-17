package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// enterWorktreeStructured handles the enter_worktree tool
func (m *Manager) enterWorktreeStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	name, _ := params["name"].(string)
	if name == "" {
		name = fmt.Sprintf("worktree-%d", os.Getpid())
	}

	// Check if we're in a git repo
	if _, err := os.Stat(".git"); err != nil {
		// Check if parent has .git
		if _, err := exec.LookPath("git"); err != nil {
			return ToolResult{
				Type:   "tool_result",
				Tool:   ToolEnterWorktree,
				Status: "error",
				Error:  "not in a git repository and git not found",
				Display: "错误：enter_worktree 需要 git 仓库",
			}
		}
	}

	// Create worktrees directory
	worktreesDir := ".eos/worktrees"
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolEnterWorktree,
			Status: "error",
			Error:  fmt.Sprintf("failed to create worktrees directory: %s", err),
			Display: fmt.Sprintf("错误：创建 worktrees 目录失败：%s", err),
		}
	}

	targetPath := filepath.Join(worktreesDir, name)

	// Create git worktree
	cmd := exec.Command("git", "worktree", "add", targetPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolEnterWorktree,
			Status: "error",
			Error:  fmt.Sprintf("git worktree add failed: %s (%s)", err, string(output)),
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
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}
