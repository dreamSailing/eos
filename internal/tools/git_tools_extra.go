package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"github.com/dreamSailing/eos/internal/pkg/utils"

	gitops "github.com/dreamSailing/eos/internal/tools/git"
)

func (m *Manager) gitLogStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	limit := 20
	if v, ok := params["limit"].(float64); ok {
		if int(v) > 0 {
			limit = int(v)
		}
	} else if v, ok := params["limit"].(int); ok {
		if v > 0 {
			limit = v
		}
	}
	oneline := true
	if v, ok := params["oneline"].(bool); ok {
		oneline = v
	}
	graph := false
	if v, ok := params["graph"].(bool); ok {
		graph = v
	}
	all := false
	if v, ok := params["all"].(bool); ok {
		all = v
	}
	p, _ := params["path"].(string)
	p = strings.TrimSpace(p)
	if p != "" {
		p = normalizePathPlaceholder(p)
		res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), p)
		if !res.IsValid {
			return ToolResult{Type: "tool_result", Tool: ToolGitLog, Status: "error", Error: "path outside working directory"}
		}
		p = res.AbsPath
	}

	ops := gitops.NewOpsWithRoot(WorkspaceRootFromContext(ctx))
	out, err := ops.Log(limit, oneline, graph, all, p)
	if err != nil {
		slog.Error("git_log.error", "component", utils.ComponentTool, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: ToolGitLog, Status: "error", Error: fmt.Sprintf("%v", err)}
	}

	data := map[string]interface{}{
		"branch":  out.Branch,
		"entries": out.Entries,
		"text":    out.Text,
	}
	display := strings.TrimSpace(out.Text)
	if display == "" {
		display = fmt.Sprintf("git_log %d", limit)
	}
	return ToolResult{Type: "tool_result", Tool: ToolGitLog, Status: "success", Data: data, Display: display}
}

func (m *Manager) gitShowStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	revision, _ := params["revision"].(string)
	revision = strings.TrimSpace(revision)
	if revision == "" {
		revision = "HEAD"
	}

	p, _ := params["path"].(string)
	p = strings.TrimSpace(p)
	if p != "" {
		p = normalizePathPlaceholder(p)
		res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), p)
		if !res.IsValid {
			return ToolResult{Type: "tool_result", Tool: ToolGitShow, Status: "error", Error: "path outside working directory"}
		}
		p = res.AbsPath
	}

	ops := gitops.NewOpsWithRoot(WorkspaceRootFromContext(ctx))
	out, err := ops.Show(revision, p)
	if err != nil {
		slog.Error("git_show.error", "component", utils.ComponentTool, "revision", revision, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: ToolGitShow, Status: "error", Error: fmt.Sprintf("%v", err)}
	}

	data := map[string]interface{}{
		"branch":   out.Branch,
		"revision": out.Revision,
		"text":     out.Text,
	}
	display := strings.TrimSpace(out.Text)
	if display == "" {
		display = "git_show " + revision
	}
	return ToolResult{Type: "tool_result", Tool: ToolGitShow, Status: "success", Data: data, Display: display}
}

func (m *Manager) gitStashStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	action, _ := params["action"].(string)
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		return ToolResult{Type: "tool_result", Tool: ToolGitStash, Status: "error", Error: "action required"}
	}
	message, _ := params["message"].(string)
	message = strings.TrimSpace(message)

	index := 0
	if v, ok := params["index"].(float64); ok {
		index = int(v)
	} else if v, ok := params["index"].(int); ok {
		index = v
	}
	includeUntracked := false
	if v, ok := params["include_untracked"].(bool); ok {
		includeUntracked = v
	}

	ops := gitops.NewOpsWithRoot(WorkspaceRootFromContext(ctx))
	out, err := ops.Stash(action, message, index, includeUntracked)
	if err != nil {
		slog.Error("git_stash.error", "component", utils.ComponentTool, "action", action, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: ToolGitStash, Status: "error", Error: fmt.Sprintf("%v", err)}
	}

	data := map[string]interface{}{
		"branch":  out.Branch,
		"action":  action,
		"stashes": out.Stashes,
		"text":    out.Text,
	}
	display := strings.TrimSpace(out.Text)
	if display == "" {
		display = "git_stash " + action
	}
	return ToolResult{Type: "tool_result", Tool: ToolGitStash, Status: "success", Data: data, Display: display}
}

func (m *Manager) gitResetStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	mode, _ := params["mode"].(string)
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "mixed"
	}
	target, _ := params["target"].(string)
	target = strings.TrimSpace(target)
	if target == "" {
		return ToolResult{Type: "tool_result", Tool: ToolGitReset, Status: "error", Error: "target required"}
	}

	ops := gitops.NewOpsWithRoot(WorkspaceRootFromContext(ctx))
	out, err := ops.Reset(mode, target)
	if err != nil {
		slog.Error("git_reset.error", "component", utils.ComponentTool, "mode", mode, "target", target, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: ToolGitReset, Status: "error", Error: fmt.Sprintf("%v", err)}
	}

	data := map[string]interface{}{
		"branch":    out.Branch,
		"mode":      out.Mode,
		"target":    out.Target,
		"head_hash": out.HeadHash,
		"text":      out.Text,
	}
	display := strings.TrimSpace(out.Text)
	if display == "" {
		display = fmt.Sprintf("reset %s %s -> %s", mode, target, out.HeadHash)
	}
	return ToolResult{Type: "tool_result", Tool: ToolGitReset, Status: "success", Data: data, Display: display}
}

func (m *Manager) gitRevertStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	commit, _ := params["commit"].(string)
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return ToolResult{Type: "tool_result", Tool: ToolGitRevert, Status: "error", Error: "commit required"}
	}
	noEdit := true
	if v, ok := params["no_edit"].(bool); ok {
		noEdit = v
	}
	mainline := 0
	if v, ok := params["mainline"].(float64); ok {
		mainline = int(v)
	} else if v, ok := params["mainline"].(int); ok {
		mainline = v
	}

	ops := gitops.NewOpsWithRoot(WorkspaceRootFromContext(ctx))
	out, err := ops.Revert(commit, noEdit, mainline)
	if err != nil {
		slog.Error("git_revert.error", "component", utils.ComponentTool, "commit", commit, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: ToolGitRevert, Status: "error", Error: fmt.Sprintf("%v", err)}
	}

	data := map[string]interface{}{
		"branch":    out.Branch,
		"commit":    out.Commit,
		"head_hash": out.HeadHash,
		"text":      out.Text,
	}
	display := strings.TrimSpace(out.Text)
	if display == "" {
		display = "reverted " + commit
	}
	return ToolResult{Type: "tool_result", Tool: ToolGitRevert, Status: "success", Data: data, Display: display}
}

func (m *Manager) gitMergeStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	branch, _ := params["branch"].(string)
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return ToolResult{Type: "tool_result", Tool: ToolGitMerge, Status: "error", Error: "branch required"}
	}
	noEdit := true
	if v, ok := params["no_edit"].(bool); ok {
		noEdit = v
	}
	noFF := false
	if v, ok := params["no_ff"].(bool); ok {
		noFF = v
	}

	ops := gitops.NewOpsWithRoot(WorkspaceRootFromContext(ctx))
	out, err := ops.Merge(branch, noEdit, noFF)
	if err != nil {
		slog.Error("git_merge.error", "component", utils.ComponentTool, "branch", branch, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: ToolGitMerge, Status: "error", Error: fmt.Sprintf("%v", err)}
	}

	data := map[string]interface{}{
		"branch":        out.Branch,
		"merged_branch": out.MergedBranch,
		"head_hash":     out.HeadHash,
		"text":          out.Text,
	}
	display := strings.TrimSpace(out.Text)
	if display == "" {
		display = "merged " + branch
	}
	return ToolResult{Type: "tool_result", Tool: ToolGitMerge, Status: "success", Data: data, Display: display}
}

func (m *Manager) gitRebaseStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	action, _ := params["action"].(string)
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		action = "start"
	}
	upstream, _ := params["upstream"].(string)
	upstream = strings.TrimSpace(upstream)
	onto, _ := params["onto"].(string)
	onto = strings.TrimSpace(onto)
	branch, _ := params["branch"].(string)
	branch = strings.TrimSpace(branch)

	ops := gitops.NewOpsWithRoot(WorkspaceRootFromContext(ctx))
	out, err := ops.Rebase(action, upstream, onto, branch)
	if err != nil {
		slog.Error("git_rebase.error", "component", utils.ComponentTool, "action", action, "upstream", upstream, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: ToolGitRebase, Status: "error", Error: fmt.Sprintf("%v", err)}
	}

	data := map[string]interface{}{
		"branch":    out.Branch,
		"action":    out.Action,
		"upstream":  out.Upstream,
		"onto":      out.Onto,
		"head_hash": out.HeadHash,
		"text":      out.Text,
	}
	display := strings.TrimSpace(out.Text)
	if display == "" {
		display = "git_rebase " + action
	}
	return ToolResult{Type: "tool_result", Tool: ToolGitRebase, Status: "success", Data: data, Display: display}
}
