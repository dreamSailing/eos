//go:build legacy

package toolhost

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ToolRunner is the minimal interface for executing a single tool invocation.
// Implementations bridge to the actual Go tool execution layer (e.g., tools.Manager).
// The Go toolhost must not perform approval or risk gating — that responsibility
// belongs to the Rust core's policy layer. By the time a request reaches ToolRunner,
// it has already been approved.
type ToolRunner interface {
	ExecuteTool(ctx context.Context, name string, args json.RawMessage) (output json.RawMessage, display string, err error)
}

// ToolCatalogRunner optionally exposes a real tool catalog to the Rust policy
// layer. It must only return metadata; safety decisions still belong to Rust.
type ToolCatalogRunner interface {
	ListTools(ctx context.Context, req CatalogRequest) ([]ToolDefinition, error)
}

// LegacyHost bridges ToolHost to the real Go tool execution layer via ToolRunner.
// It converts between the wire types (ExecuteRequest/ExecuteResponse) and the
// runner's simpler interface, and handles per-request timeout derived from
// ExecuteRequest.TimeoutMs.
type LegacyHost struct {
	Runner  ToolRunner
	Catalog ToolCatalogRunner
}

func (h *LegacyHost) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResponse, error) {
	if h.Runner == nil {
		return ExecuteResponse{}, fmt.Errorf("legacy host: runner not initialized")
	}

	start := time.Now()

	if req.TimeoutMs != nil && *req.TimeoutMs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*req.TimeoutMs)*time.Millisecond)
		defer cancel()
	}

	if req.WorkspaceRoot != "" {
		ctx = context.WithValue(ctx, workspaceRootKey, req.WorkspaceRoot)
	}
	if req.RequestID != "" {
		ctx = context.WithValue(ctx, requestIDKey, req.RequestID)
	}

	output, display, err := h.Runner.ExecuteTool(ctx, req.Name, req.Args)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return ExecuteResponse{
			Name:       req.Name,
			RequestID:  req.RequestID,
			Status:     "error",
			Error:      err.Error(),
			DurationMs: &elapsed,
		}, nil
	}

	return ExecuteResponse{
		Name:       req.Name,
		RequestID:  req.RequestID,
		Status:     "ok",
		Display:    display,
		Output:     output,
		DurationMs: &elapsed,
	}, nil
}

func (h *LegacyHost) ListCatalog(ctx context.Context, req CatalogRequest) ([]ToolDefinition, error) {
	runner := h.Catalog
	if runner == nil {
		if catalogRunner, ok := h.Runner.(ToolCatalogRunner); ok {
			runner = catalogRunner
		}
	}
	if runner == nil {
		return nil, fmt.Errorf("legacy host: catalog runner not initialized")
	}
	return runner.ListTools(ctx, req)
}

type ctxKey string

const (
	workspaceRootKey ctxKey = "toolhost.workspace_root"
	requestIDKey     ctxKey = "toolhost.request_id"
)

// WorkspaceRootFromCtx extracts the workspace root injected by LegacyHost.
func WorkspaceRootFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(workspaceRootKey).(string); ok {
		return v
	}
	return ""
}

// RequestIDFromCtx extracts the request ID injected by LegacyHost.
func RequestIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// ToolRunnerFunc adapts a plain function to the ToolRunner interface.
type ToolRunnerFunc func(ctx context.Context, name string, args json.RawMessage) (output json.RawMessage, display string, err error)

func (f ToolRunnerFunc) ExecuteTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, string, error) {
	return f(ctx, name, args)
}

type ToolCatalogRunnerFunc func(ctx context.Context, req CatalogRequest) ([]ToolDefinition, error)

func (f ToolCatalogRunnerFunc) ListTools(ctx context.Context, req CatalogRequest) ([]ToolDefinition, error) {
	return f(ctx, req)
}
