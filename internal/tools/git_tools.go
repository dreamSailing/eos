package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"github.com/dreamSailing/eos/internal/pkg/utils"
	"log/slog"
	"path/filepath"
	"strings"

	gitops "github.com/dreamSailing/eos/internal/tools/git"
)

func (m *Manager) gitStatusStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	ops := gitops.NewOpsWithRoot(WorkspaceRootFromContext(ctx))
	changes, err := ops.Status()
	if err != nil {
		slog.Error("git_status.error", "component", utils.ComponentTool, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "git_status", Status: "error", Error: fmt.Sprintf("%v", err)}
	}
	return ToolResult{Type: "tool_result", Tool: "git_status", Status: "success", Data: map[string]interface{}{"changes": changes}, Display: fmt.Sprintf("%d change(s)", len(changes))}
}

func (m *Manager) gitAddStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	var paths []string
	if v, ok := params["paths"].([]interface{}); ok {
		for _, x := range v {
			if s, ok2 := x.(string); ok2 {
				s = strings.TrimSpace(s)
				if s == "" {
					continue
				}
				s = normalizePathPlaceholder(s)
				if s == "." {
					paths = append(paths, s)
					continue
				}
				res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), s)
				if !res.IsValid {
					return ToolResult{Type: "tool_result", Tool: ToolGitAdd, Status: "error", Error: "path outside working directory"}
				}
				paths = append(paths, res.AbsPath)
			}
		}
	}
	ops := gitops.NewOpsWithRoot(WorkspaceRootFromContext(ctx))
	if r, blocked := gitMutationSandboxResult(ctx, ToolGitAdd, ops.Root); blocked {
		return r
	}
	n, err := ops.Add(paths)
	if err != nil {
		slog.Error("git_add.error", "component", utils.ComponentTool, "paths", len(paths), "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "git_add", Status: "error", Error: fmt.Sprintf("%v", err)}
	}
	return ToolResult{Type: "tool_result", Tool: "git_add", Status: "success", Data: map[string]interface{}{"count": n}, Display: fmt.Sprintf("Staged %d file(s)", n)}
}

func (m *Manager) gitCommitStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	msg, _ := params["message"].(string)
	name, _ := params["author_name"].(string)
	email, _ := params["author_email"].(string)
	ops := gitops.NewOpsWithRoot(WorkspaceRootFromContext(ctx))
	if r, blocked := gitMutationSandboxResult(ctx, ToolGitCommit, ops.Root); blocked {
		return r
	}
	out, err := ops.Commit(msg, name, email)
	if err != nil {
		slog.Error("git_commit.error", "component", utils.ComponentTool, "author_name", name, "author_email", email, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "git_commit", Status: "error", Error: fmt.Sprintf("%v", err)}
	}
	return ToolResult{Type: "tool_result", Tool: "git_commit", Status: "success", Data: map[string]interface{}{"hash": out.Hash, "files": out.Files}, Display: fmt.Sprintf("Commit %s", out.Hash)}
}

func (m *Manager) gitBranchListStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	ops := gitops.NewOpsWithRoot(WorkspaceRootFromContext(ctx))
	bs, cur, err := ops.BranchList()
	if err != nil {
		slog.Error("git_branch_list.error", "component", utils.ComponentTool, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "git_branch_list", Status: "error", Error: fmt.Sprintf("%v", err)}
	}
	return ToolResult{Type: "tool_result", Tool: "git_branch_list", Status: "success", Data: map[string]interface{}{"branches": bs, "current": cur}, Display: fmt.Sprintf("Current %s", cur)}
}

func (m *Manager) gitCheckoutStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	name, _ := params["name"].(string)
	create := false
	if v, ok := params["create"].(bool); ok {
		create = v
	}
	ops := gitops.NewOpsWithRoot(WorkspaceRootFromContext(ctx))
	if r, blocked := gitMutationSandboxResult(ctx, ToolGitCheckout, ops.Root); blocked {
		return r
	}
	br, err := ops.Checkout(name, create)
	if err != nil {
		slog.Error("git_checkout.error", "component", utils.ComponentTool, "name", name, "create", create, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "git_checkout", Status: "error", Error: fmt.Sprintf("%v", err)}
	}
	return ToolResult{Type: "tool_result", Tool: "git_checkout", Status: "success", Data: map[string]interface{}{"branch": br}, Display: "Checked out " + br}
}

func (m *Manager) gitInitStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	ops := gitops.NewOpsWithRoot(WorkspaceRootFromContext(ctx))
	if r, blocked := gitMutationSandboxResult(ctx, ToolGitInit, ops.Root); blocked {
		return r
	}
	p, err := ops.Init()
	if err != nil {
		slog.Error("git_init.error", "component", utils.ComponentTool, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "git_init", Status: "error", Error: fmt.Sprintf("%v", err)}
	}
	return ToolResult{Type: "tool_result", Tool: "git_init", Status: "success", Data: map[string]interface{}{"path": p}, Display: "Initialized repo"}
}

func (m *Manager) gitPullStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	remote, _ := params["remote"].(string)
	branch, _ := params["branch"].(string)
	user, _ := params["username"].(string)
	pass, _ := params["password"].(string)
	ops := gitops.NewOpsWithRoot(WorkspaceRootFromContext(ctx))
	if r, blocked := gitMutationSandboxResult(ctx, ToolGitPull, ops.Root); blocked {
		return r
	}
	s, err := ops.Pull(remote, branch, user, pass)
	if err != nil {
		slog.Error("git_pull.error", "component", utils.ComponentTool, "remote", remote, "branch", branch, "user", user, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "git_pull", Status: "error", Error: fmt.Sprintf("%v", err)}
	}
	return ToolResult{Type: "tool_result", Tool: "git_pull", Status: "success", Data: map[string]interface{}{"status": s}, Display: "Pull " + s}
}

func (m *Manager) gitPushStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	remote, _ := params["remote"].(string)
	branch, _ := params["branch"].(string)
	user, _ := params["username"].(string)
	pass, _ := params["password"].(string)
	ops := gitops.NewOpsWithRoot(WorkspaceRootFromContext(ctx))
	if r, blocked := gitMutationSandboxResult(ctx, ToolGitPush, ops.Root); blocked {
		return r
	}
	s, err := ops.Push(remote, branch, user, pass)
	if err != nil {
		slog.Error("git_push.error", "component", utils.ComponentTool, "remote", remote, "branch", branch, "user", user, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "git_push", Status: "error", Error: fmt.Sprintf("%v", err)}
	}
	return ToolResult{Type: "tool_result", Tool: "git_push", Status: "success", Data: map[string]interface{}{"status": s}, Display: "Push " + s}
}

func (m *Manager) gitDiffStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	p, _ := params["path"].(string)
	p = strings.TrimSpace(p)
	p = normalizePathPlaceholder(p)
	res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), p)
	if !res.IsValid {
		return ToolResult{Type: "tool_result", Tool: ToolGitDiff, Status: "error", Error: "path outside working directory"}
	}
	ops := gitops.NewOpsWithRoot(WorkspaceRootFromContext(ctx))
	txt, err := ops.Diff(res.AbsPath)
	if err != nil {
		slog.Error("git_diff.error", "component", utils.ComponentTool, "path", p, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "git_diff", Status: "error", Error: fmt.Sprintf("%v", err)}
	}
	return ToolResult{Type: "tool_result", Tool: "git_diff", Status: "success", Data: map[string]interface{}{"path": filepath.ToSlash(res.RelPath), "text": txt}, Display: txt}
}
