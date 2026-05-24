package impl

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。
//
// legacy_bridge.go 是 toolapi 对 internal/tools、internal/runtime、
// internal/tools/bg 等遗留包的唯一依赖入口。executor.go 和 tasks.go
// 通过 bridge.go 中定义的 ToolExecutorBridge / TaskSourceBridge 接口
// 与此文件交互，不再直接 import 上述遗留包。
//
// 当未来迁移到 coreapi.Engine 或其他后端时，只需替换此文件中的实现。

import (
	"context"
	"fmt"
	"time"

	"github.com/dreamSailing/eos/internal/browser"
	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/mcp"
	pluginpkg "github.com/dreamSailing/eos/internal/pkg/plugins"
	"github.com/dreamSailing/eos/internal/runtime"
	"github.com/dreamSailing/eos/internal/skills"
	"github.com/dreamSailing/eos/internal/toolapi"
	"github.com/dreamSailing/eos/internal/tools"
	"github.com/dreamSailing/eos/internal/tools/bg"
)

// ---------------------------------------------------------------------------
// ToolExecutorBridge — legacy implementation backed by tools.Manager
// ---------------------------------------------------------------------------

type legacyToolExecutor struct {
	mgr *tools.Manager
}

func newLegacyToolExecutor(workspaceRoot string) ToolExecutorBridge {
	m := tools.NewManager()
	m.SetWorkspaceRoot(workspaceRoot)
	configureManagerExtensions(context.Background(), m, workspaceRoot)
	return &legacyToolExecutor{mgr: m}
}

func (e *legacyToolExecutor) Execute(ctx context.Context, sess toolapi.ExecSession, calls []toolapi.ToolCall) ([]toolapi.ToolResult, error) {
	if e == nil || e.mgr == nil {
		return nil, fmt.Errorf("executor not initialized")
	}
	ctx = tools.WithAllowedTools(ctx, sess.AllowedTools)
	ctx = tools.WithTraceID(ctx, sess.TraceID)
	ctx = tools.WithWorkspaceRoot(ctx, sess.WorkspaceRoot)
	ctx = tools.WithAccessMode(ctx, toolapi.ResolveAccessMode(sess))
	ctx = tools.WithApprovalMode(ctx, toolapi.ResolveApprovalMode(sess))
	in := make([]tools.ToolCall, 0, len(calls))
	for _, c := range calls {
		in = append(in, tools.ToolCall{
			ID:         c.ID,
			Tool:       c.Name,
			Parameters: c.Params,
		})
	}
	out := e.mgr.ExecuteStructured(ctx, in)
	res := make([]toolapi.ToolResult, 0, len(out))
	for _, r := range out {
		res = append(res, toolapi.ToolResult{
			ID:      r.ID,
			Type:    r.Type,
			Tool:    r.Tool,
			Status:  r.Status,
			Data:    r.Data,
			Error:   r.Error,
			Display: r.Display,
			Ts:      r.Ts,
		})
	}
	return res, nil
}

// configureManagerExtensions wires skills, MCP, and browser runtime into a
// tools.Manager. Extracted from the former extensions.go.
func configureManagerExtensions(ctx context.Context, mgr *tools.Manager, workspaceRoot string) {
	if mgr == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	cfg, _ := config.Load()

	loader := skills.NewLoader()
	skillDirs := skills.ResolveScanDirs(workspaceRoot, &cfg)
	if len(skillDirs) > 0 {
		loader.SetSkillsDirs(skillDirs)
		_ = loader.Scan()
	}
	mgr.SetSkillManager(tools.NewSkillManager(loader, mgr))

	mcpMgr := mcp.NewManager()
	browserRT := browser.NewBuiltinRuntime()
	loadCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	mcpCfg := cfg
	mcpCfg.MCP = pluginpkg.MergeMCPEntries(&cfg, workspaceRoot)
	_ = mcpMgr.LoadFromConfig(loadCtx, &mcpCfg)
	mgr.SetMCPManager(mcpMgr)
	mgr.SetBrowserRuntime(browserRT)
}

// ---------------------------------------------------------------------------
// TaskSourceBridge — legacy implementation backed by bg, TodoStore, AgentRegistry
// ---------------------------------------------------------------------------

type legacyTaskSource struct{}

func newLegacyTaskSource() TaskSourceBridge {
	return &legacyTaskSource{}
}

func (s *legacyTaskSource) List(_ context.Context) ([]toolapi.TaskInfo, error) {
	shellTasks := bg.Default().List()
	todoItems := tools.DefaultTodoStore().List()
	agentTasks := runtime.DefaultAgentRegistry().ListSnapshots()

	out := make([]toolapi.TaskInfo, 0, len(shellTasks)+len(todoItems)+len(agentTasks))
	for _, it := range shellTasks {
		out = append(out, toolapi.TaskInfo{
			ID:        it.ID,
			Kind:      "shell_task",
			Status:    string(it.Status),
			StartedAt: it.StartedAt,
			UpdatedAt: maxTaskTime(it.StartedAt, it.ExitedAt),
			EndedAt:   it.ExitedAt,
			Label:     it.Command,
			Summary:   it.Error,
			CanKill:   it.Status == bg.StatusRunning,
		})
	}
	for idx, it := range todoItems {
		id := it.ID
		if id == "" {
			id = fmt.Sprintf("todo_%d", idx+1)
		}
		out = append(out, toolapi.TaskInfo{
			ID:        id,
			Kind:      "todo_item",
			Status:    it.Status,
			StartedAt: it.UpdatedAt,
			UpdatedAt: it.UpdatedAt,
			Label:     it.Content,
			Metadata: map[string]any{
				"priority": it.Priority,
			},
		})
	}
	for _, it := range agentTasks {
		out = append(out, toolapi.TaskInfo{
			ID:        it.ID,
			Kind:      "agent_task",
			Status:    string(it.Status),
			StartedAt: it.StartedAt,
			UpdatedAt: it.UpdatedAt,
			EndedAt:   it.CompletedAt,
			Label:     it.Task,
			Summary:   it.Result,
			CanKill:   it.Status == runtime.AgentStatusRunning,
			CanResume: it.CanResume,
			CanClose:  it.CanClose,
			Metadata: map[string]any{
				"agent_name":       it.Name,
				"context_strategy": it.Strategy,
				"messages":         it.Messages,
				"error":            it.Error,
				"allowed_tools":    append([]string(nil), it.AllowedTools...),
			},
		})
	}
	return out, nil
}

func (s *legacyTaskSource) Kill(_ context.Context, id string) error {
	if _, err := bg.Default().Kill(id); err == nil {
		return nil
	}
	if runtime.DefaultAgentRegistry().RequestCancel(id) {
		return nil
	}
	return fmt.Errorf("task not found: %s", id)
}

func (s *legacyTaskSource) Resume(_ context.Context, id string) error {
	if runtime.DefaultAgentRegistry().Resume(id, "") {
		return nil
	}
	return fmt.Errorf("task not resumable: %s", id)
}

func (s *legacyTaskSource) Close(_ context.Context, id string) error {
	if runtime.DefaultAgentRegistry().Close(id) {
		return nil
	}
	return fmt.Errorf("task not closeable: %s", id)
}

func maxTaskTime(times ...time.Time) time.Time {
	var out time.Time
	for _, ts := range times {
		if ts.After(out) {
			out = ts
		}
	}
	return out
}
