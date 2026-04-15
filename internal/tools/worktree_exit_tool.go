package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// exitWorktreeStructured handles the exit_worktree tool
func (m *Manager) exitWorktreeStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	path, _ := params["path"].(string)
	if path == "" {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolExitWorktree,
			Status:  "error",
			Error:   "path is required",
			Display: "Error: exit_worktree requires a path",
		}
	}

	remove := true
	if v, ok := params["remove"].(bool); ok {
		remove = v
	}

	// Verify the path exists as a worktree
	if _, err := os.Stat(path); err != nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolExitWorktree,
			Status:  "error",
			Error:   fmt.Sprintf("path does not exist: %s", path),
			Display: fmt.Sprintf("Error: path %s does not exist", path),
		}
	}

	if remove {
		// Remove git worktree
		cmd := exec.Command("git", "worktree", "remove", "--force", path)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return ToolResult{
				Type:    "tool_result",
				Tool:    ToolExitWorktree,
				Status:  "error",
				Error:   fmt.Sprintf("git worktree remove failed: %s (%s)", err, string(output)),
				Display: fmt.Sprintf("Error removing worktree: %s", string(output)),
			}
		}

		if OnWorktreeChange != nil {
			OnWorktreeChange("removed", path)
		}

		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolExitWorktree,
			Status: "success",
			Data: map[string]interface{}{
				"path":   path,
				"removed": true,
			},
			Display: fmt.Sprintf("Removed worktree at %s", path),
		}
	}

	// Just detach (prune) without removing
	cmd := exec.Command("git", "worktree", "prune")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolExitWorktree,
			Status:  "error",
			Error:   fmt.Sprintf("git worktree prune failed: %s (%s)", err, string(output)),
			Display: fmt.Sprintf("Error pruning worktrees: %s", string(output)),
		}
	}

	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolExitWorktree,
		Status: "success",
		Data: map[string]interface{}{
			"path":    path,
			"removed": false,
		},
		Display: fmt.Sprintf("Pruned worktree references for %s", path),
	}
}
