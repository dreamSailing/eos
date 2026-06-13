package toolhost

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type CatalogRequest struct {
	WorkspaceRoot string   `json:"workspace_root,omitempty"`
	IncludeTools  []string `json:"include_tools,omitempty"`
	AllowedTools  []string `json:"allowed_tools,omitempty"`
}

type ToolDefinition struct {
	Name               string                       `json:"name"`
	Description        string                       `json:"description,omitempty"`
	RiskLevel          string                       `json:"risk_level,omitempty"`
	Params             map[string]ToolParameterInfo `json:"params,omitempty"`
	Examples           []ToolExample                `json:"examples,omitempty"`
	Source             string                       `json:"source,omitempty"`
	Category           string                       `json:"category,omitempty"`
	VisibleIn          []string                     `json:"visible_in,omitempty"`
	ReadOnly           bool                         `json:"read_only"`
	Invocable          bool                         `json:"invocable"`
	RequiresFullAccess bool                         `json:"requires_full_access"`
	Tags               []string                     `json:"tags,omitempty"`
	ParamsSchema       json.RawMessage              `json:"params_schema,omitempty"`
	Metadata           map[string]any               `json:"metadata,omitempty"`
}

type ToolParameterInfo struct {
	Type     string `json:"type,omitempty"`
	Required bool   `json:"required"`
	Desc     string `json:"desc,omitempty"`
}

type ToolExample struct {
	Description string         `json:"description,omitempty"`
	Input       map[string]any `json:"input,omitempty"`
}

type ExecuteRequest struct {
	SessionID     string          `json:"session_id,omitempty"`
	TurnID        string          `json:"turn_id,omitempty"`
	RequestID     string          `json:"request_id,omitempty"`
	AgentID       string          `json:"agent_id,omitempty"`
	Name          string          `json:"name"`
	Args          json.RawMessage `json:"args,omitempty"`
	TimeoutMs     *int64          `json:"timeout_ms,omitempty"`
	WorkspaceRoot string          `json:"workspace_root,omitempty"`
}

type ExecuteResponse struct {
	Name       string          `json:"name"`
	RequestID  string          `json:"request_id,omitempty"`
	Status     string          `json:"status,omitempty"`
	Display    string          `json:"display,omitempty"`
	Output     json.RawMessage `json:"output,omitempty"`
	Error      string          `json:"error,omitempty"`
	DurationMs *int64          `json:"duration_ms,omitempty"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
}

type ToolHost interface {
	Execute(ctx context.Context, req ExecuteRequest) (ExecuteResponse, error)
}

type ToolCatalogHost interface {
	ListCatalog(ctx context.Context, req CatalogRequest) ([]ToolDefinition, error)
}

type FakeHost struct {
	OnExecute func(ctx context.Context, req ExecuteRequest) (ExecuteResponse, error)
	OnCatalog func(ctx context.Context, req CatalogRequest) ([]ToolDefinition, error)
}

func (h *FakeHost) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResponse, error) {
	if h.OnExecute != nil {
		return h.OnExecute(ctx, req)
	}
	return ExecuteResponse{
		Name:      req.Name,
		RequestID: req.RequestID,
		Status:    "ok",
		Display:   fmt.Sprintf("fake execution of %s", req.Name),
		Output:    json.RawMessage(`{"fake":true}`),
	}, nil
}

func (h *FakeHost) ListCatalog(ctx context.Context, req CatalogRequest) ([]ToolDefinition, error) {
	if h.OnCatalog != nil {
		return h.OnCatalog(ctx, req)
	}
	return filterToolDefinitions([]ToolDefinition{
		{
			Name:        "fake_tool",
			Description: "fake tool host catalog entry",
			RiskLevel:   "low",
			Source:      "fake",
			Category:    "test",
			ReadOnly:    true,
			Invocable:   true,
			ParamsSchema: json.RawMessage(`{
				"type":"object",
				"properties":{"input":{"type":"string","description":"fake input"}}
			}`),
		},
	}, req), nil
}

type errHost struct {
	err error
}

func ErrorHost(err error) ToolHost {
	return &errHost{err: err}
}

func (h *errHost) Execute(_ context.Context, req ExecuteRequest) (ExecuteResponse, error) {
	return ExecuteResponse{}, h.err
}

type timeoutHost struct {
	duration time.Duration
}

func TimeoutHost(d time.Duration) ToolHost {
	return &timeoutHost{duration: d}
}

func (h *timeoutHost) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResponse, error) {
	select {
	case <-ctx.Done():
		return ExecuteResponse{}, ctx.Err()
	case <-time.After(h.duration):
		return ExecuteResponse{
			Name:      req.Name,
			RequestID: req.RequestID,
			Status:    "ok",
			Display:   "delayed execution",
			Output:    json.RawMessage(`{"delayed":true}`),
		}, nil
	}
}

func filterToolDefinitions(defs []ToolDefinition, req CatalogRequest) []ToolDefinition {
	include := stringSet(req.IncludeTools)
	allowed := stringSet(req.AllowedTools)
	out := make([]ToolDefinition, 0, len(defs))
	for _, def := range defs {
		if len(include) > 0 {
			if _, ok := include[def.Name]; !ok {
				continue
			}
		}
		if len(allowed) > 0 {
			if _, ok := allowed[def.Name]; !ok {
				continue
			}
		}
		out = append(out, def)
	}
	return out
}

func stringSet(items []string) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item != "" {
			out[item] = struct{}{}
		}
	}
	return out
}
