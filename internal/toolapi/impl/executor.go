package impl

import (
	"context"
	"fmt"

	"github.com/dreamSailing/vb-coding/internal/toolapi"
	"github.com/dreamSailing/vb-coding/internal/tools"
)

type executor struct {
	mgr *tools.Manager
}

func newExecutor(workspaceRoot string) toolapi.Executor {
	m := tools.NewManager()
	m.SetWorkspaceRoot(workspaceRoot)
	configureManagerExtensions(context.Background(), m, workspaceRoot)
	return &executor{mgr: m}
}

func (e *executor) Execute(ctx context.Context, sess toolapi.ExecSession, calls []toolapi.ToolCall) ([]toolapi.ToolResult, error) {
	if e == nil || e.mgr == nil {
		return nil, fmt.Errorf("executor not initialized")
	}
	ctx = tools.WithAllowedTools(ctx, sess.AllowedTools)
	ctx = tools.WithTraceID(ctx, sess.TraceID)
	ctx = tools.WithWorkspaceRoot(ctx, sess.WorkspaceRoot)
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
