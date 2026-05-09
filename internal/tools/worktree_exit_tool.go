package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"github.com/dreamSailing/eos/internal/pkg/utils"
	"context"
	"fmt"
	"os"
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
			Display: "错误：exit_worktree 需要指定路径",
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
			Display: fmt.Sprintf("错误：路径 %s 不存在", path),
		}
	}

	if remove {
		// Remove git worktree
		cmd := utils.Command("git", "worktree", "remove", "--force", path)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return ToolResult{
				Type:    "tool_result",
				Tool:    ToolExitWorktree,
				Status:  "error",
				Error:   fmt.Sprintf("git worktree remove failed: %s (%s)", err, string(output)),
				Display: fmt.Sprintf("错误：移除 worktree 失败：%s", string(output)),
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
			Display: fmt.Sprintf("已移除 worktree：%s", path),
		}
	}

	// Just detach (prune) without removing
	cmd := utils.Command("git", "worktree", "prune")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolExitWorktree,
			Status:  "error",
			Error:   fmt.Sprintf("git worktree prune failed: %s (%s)", err, string(output)),
			Display: fmt.Sprintf("错误：清理 worktree 失败：%s", string(output)),
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
		Display: fmt.Sprintf("已清理 %s 的 worktree 引用", path),
	}
}
