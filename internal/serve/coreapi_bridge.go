package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dreamSailing/eos/internal/toolapi"
	"github.com/dreamSailing/eos/internal/tools"
	"github.com/dreamSailing/eos/pkg/coreapi"
)

func newCoreAPIBridge(base toolapi.Services, engine coreapi.Engine) toolapi.Services {
	return &coreapiBridge{base: base, engine: engine}
}

type coreapiBridge struct {
	base   toolapi.Services
	engine coreapi.Engine
}

func (b *coreapiBridge) NewExecutor(workspaceRoot string) toolapi.Executor {
	return &coreapiExecutorAdapter{
		executor:  b.engine.Tools(),
		workspace: workspaceRoot,
	}
}

func (b *coreapiBridge) Catalog() toolapi.Catalog {
	if tc := b.engine.ToolCatalog(); tc != nil {
		return &coreapiCatalogAdapter{catalog: tc, base: b.base.Catalog()}
	}
	return b.base.Catalog()
}

func (b *coreapiBridge) Tasks() toolapi.Tasks {
	return &coreapiTaskAdapter{tasks: b.engine.Tasks()}
}

type coreapiCatalogAdapter struct {
	catalog coreapi.ToolCatalogService
	base    toolapi.Catalog
}

func (a *coreapiCatalogAdapter) List(ctx context.Context) ([]toolapi.ToolDefinition, error) {
	req := coreapi.ListToolCatalogRequest{}
	if root := tools.WorkspaceRootFromContext(ctx); root != "" {
		req.WorkspaceRoot = root
	}
	coreDefs, err := a.catalog.List(ctx, req)
	if err != nil {
		return nil, err
	}
	return coreapiDefsToToolAPIDefs(coreDefs), nil
}

func (a *coreapiCatalogAdapter) RiskLevel(toolName string) toolapi.RiskLevel {
	return a.base.RiskLevel(toolName)
}

type coreapiExecutorAdapter struct {
	executor  coreapi.ToolExecutor
	workspace string
}

func (a *coreapiExecutorAdapter) Execute(ctx context.Context, sess toolapi.ExecSession, calls []toolapi.ToolCall) ([]toolapi.ToolResult, error) {
	if a.executor == nil {
		return nil, fmt.Errorf("coreapi tool executor not available")
	}
	results := make([]toolapi.ToolResult, 0, len(calls))
	for _, call := range calls {
		argsJSON, err := json.Marshal(call.Params)
		if err != nil {
			return nil, fmt.Errorf("marshal tool args: %w", err)
		}
		result, err := a.executor.Execute(ctx, coreapi.ToolRequest{
			SessionID: sess.WorkspaceRoot,
			RequestID: call.ID,
			Name:      call.Name,
			Args:      argsJSON,
		})
		if err != nil {
			return nil, err
		}
		results = append(results, mapCoreAPIToolResult(result))
	}
	return results, nil
}

func mapCoreAPIToolResult(r coreapi.ToolResult) toolapi.ToolResult {
	out := toolapi.ToolResult{
		ID:      r.RequestID,
		Type:    "tool_result",
		Tool:    r.Name,
		Status:  r.Status,
		Error:   r.Error,
		Display: r.Display,
		Ts:      time.Now().Unix(),
	}
	if out.Status == "" {
		out.Status = "success"
	}
	if len(r.Output) > 0 {
		var data map[string]any
		if err := json.Unmarshal(r.Output, &data); err == nil {
			out.Data = data
		} else {
			out.Data = map[string]any{"output": string(r.Output)}
		}
	}
	if out.Data == nil {
		out.Data = map[string]any{}
	}
	return out
}

type coreapiTaskAdapter struct {
	tasks coreapi.TaskService
}

func (a *coreapiTaskAdapter) List(ctx context.Context) ([]toolapi.TaskInfo, error) {
	if a.tasks == nil {
		return nil, fmt.Errorf("coreapi task service not available")
	}
	snapshots, err := a.tasks.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]toolapi.TaskInfo, 0, len(snapshots))
	for _, s := range snapshots {
		out = append(out, toolapi.TaskInfo{
			ID:        s.ID,
			Kind:      s.Kind,
			Status:    s.Status,
			StartedAt: s.StartedAt,
			UpdatedAt: s.UpdatedAt,
			EndedAt:   s.EndedAt,
			Label:     s.Label,
			Summary:   s.Summary,
			CanKill:   s.CanKill,
			CanResume: s.CanResume,
			CanClose:  s.CanClose,
			Metadata:  s.Metadata,
		})
	}
	return out, nil
}

func (a *coreapiTaskAdapter) Kill(ctx context.Context, id string) error {
	if a.tasks == nil {
		return fmt.Errorf("coreapi task service not available")
	}
	return a.tasks.Kill(ctx, coreapi.TaskIDRequest{TaskID: id})
}

func (a *coreapiTaskAdapter) Resume(ctx context.Context, id string) error {
	if a.tasks == nil {
		return fmt.Errorf("coreapi task service not available")
	}
	return fmt.Errorf("task resume: %w", coreapi.ErrUnsupported)
}

func (a *coreapiTaskAdapter) Close(ctx context.Context, id string) error {
	if a.tasks == nil {
		return fmt.Errorf("coreapi task service not available")
	}
	return fmt.Errorf("task close: %w", coreapi.ErrUnsupported)
}
