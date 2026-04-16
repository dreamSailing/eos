package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// compile-time check
var _ mcpclient.MCPClient = (*StreamableHTTPAdapter)(nil)

// StreamableHTTPAdapter wraps StreamableHTTPClient to satisfy mcpclient.MCPClient
type StreamableHTTPAdapter struct {
	client *StreamableHTTPClient
}

// NewStreamableHTTPAdapter creates an adapter wrapping a StreamableHTTPClient
func NewStreamableHTTPAdapter(baseURL string, headers map[string]string) *StreamableHTTPAdapter {
	return &StreamableHTTPAdapter{
		client: NewStreamableHTTPClient(baseURL, headers),
	}
}

func (a *StreamableHTTPAdapter) Initialize(ctx context.Context, req mcp.InitializeRequest) (*mcp.InitializeResult, error) {
	result, err := a.client.Initialize(ctx)
	if err != nil {
		return nil, err
	}
	// Convert map[string]interface{} to *mcp.InitializeResult
	raw, _ := json.Marshal(result)
	var initResult mcp.InitializeResult
	if err := json.Unmarshal(raw, &initResult); err != nil {
		return nil, fmt.Errorf("failed to parse initialize result: %w", err)
	}
	return &initResult, nil
}

func (a *StreamableHTTPAdapter) Ping(ctx context.Context) error {
	return nil // Streamable HTTP doesn't have a dedicated ping
}

func (a *StreamableHTTPAdapter) ListResourcesByPage(ctx context.Context, req mcp.ListResourcesRequest) (*mcp.ListResourcesResult, error) {
	return a.ListResources(ctx, req)
}

func (a *StreamableHTTPAdapter) ListResources(ctx context.Context, req mcp.ListResourcesRequest) (*mcp.ListResourcesResult, error) {
	resources, err := a.client.ListResources(ctx)
	if err != nil {
		return nil, err
	}
	var mcpResources []mcp.Resource
	for _, r := range resources {
		raw, _ := json.Marshal(r)
		var res mcp.Resource
		if err := json.Unmarshal(raw, &res); err == nil {
			mcpResources = append(mcpResources, res)
		}
	}
	return &mcp.ListResourcesResult{Resources: mcpResources}, nil
}

func (a *StreamableHTTPAdapter) ListResourceTemplatesByPage(ctx context.Context, req mcp.ListResourceTemplatesRequest) (*mcp.ListResourceTemplatesResult, error) {
	return &mcp.ListResourceTemplatesResult{}, nil
}

func (a *StreamableHTTPAdapter) ListResourceTemplates(ctx context.Context, req mcp.ListResourceTemplatesRequest) (*mcp.ListResourceTemplatesResult, error) {
	return &mcp.ListResourceTemplatesResult{}, nil
}

func (a *StreamableHTTPAdapter) ReadResource(ctx context.Context, req mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	result, err := a.client.ReadResource(ctx, req.Params.URI)
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(result)
	var readResult mcp.ReadResourceResult
	if err := json.Unmarshal(raw, &readResult); err != nil {
		return nil, fmt.Errorf("failed to parse read resource result: %w", err)
	}
	return &readResult, nil
}

func (a *StreamableHTTPAdapter) Subscribe(ctx context.Context, req mcp.SubscribeRequest) error {
	return nil
}

func (a *StreamableHTTPAdapter) Unsubscribe(ctx context.Context, req mcp.UnsubscribeRequest) error {
	return nil
}

func (a *StreamableHTTPAdapter) ListPromptsByPage(ctx context.Context, req mcp.ListPromptsRequest) (*mcp.ListPromptsResult, error) {
	return a.ListPrompts(ctx, req)
}

func (a *StreamableHTTPAdapter) ListPrompts(ctx context.Context, req mcp.ListPromptsRequest) (*mcp.ListPromptsResult, error) {
	prompts, err := a.client.ListPrompts(ctx)
	if err != nil {
		return nil, err
	}
	var mcpPrompts []mcp.Prompt
	for _, p := range prompts {
		raw, _ := json.Marshal(p)
		var prompt mcp.Prompt
		if err := json.Unmarshal(raw, &prompt); err == nil {
			mcpPrompts = append(mcpPrompts, prompt)
		}
	}
	return &mcp.ListPromptsResult{Prompts: mcpPrompts}, nil
}

func (a *StreamableHTTPAdapter) GetPrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := make(map[string]interface{})
	if req.Params.Arguments != nil {
		raw, _ := json.Marshal(req.Params.Arguments)
		_ = json.Unmarshal(raw, &args)
	}
	result, err := a.client.GetPrompt(ctx, req.Params.Name, args)
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(result)
	var promptResult mcp.GetPromptResult
	if err := json.Unmarshal(raw, &promptResult); err != nil {
		return nil, fmt.Errorf("failed to parse get prompt result: %w", err)
	}
	return &promptResult, nil
}

func (a *StreamableHTTPAdapter) ListToolsByPage(ctx context.Context, req mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	return a.ListTools(ctx, req)
}

func (a *StreamableHTTPAdapter) ListTools(ctx context.Context, req mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	tools, err := a.client.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	// Convert []interface{} to []mcp.Tool
	var mcpTools []mcp.Tool
	for _, t := range tools {
		raw, _ := json.Marshal(t)
		var tool mcp.Tool
		if err := json.Unmarshal(raw, &tool); err == nil {
			mcpTools = append(mcpTools, tool)
		}
	}
	return &mcp.ListToolsResult{Tools: mcpTools}, nil
}

func (a *StreamableHTTPAdapter) CallTool(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := make(map[string]interface{})
	if req.Params.Arguments != nil {
		raw, _ := json.Marshal(req.Params.Arguments)
		_ = json.Unmarshal(raw, &args)
	}
	result, err := a.client.CallTool(ctx, req.Params.Name, args)
	if err != nil {
		return nil, err
	}
	// Convert to *mcp.CallToolResult
	raw, _ := json.Marshal(result)
	var callResult mcp.CallToolResult
	if err := json.Unmarshal(raw, &callResult); err != nil {
		return nil, fmt.Errorf("failed to parse call tool result: %w", err)
	}
	return &callResult, nil
}

func (a *StreamableHTTPAdapter) SetLevel(ctx context.Context, req mcp.SetLevelRequest) error {
	return nil
}

func (a *StreamableHTTPAdapter) Complete(ctx context.Context, req mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	return &mcp.CompleteResult{}, nil
}

func (a *StreamableHTTPAdapter) Close() error {
	return a.client.Close()
}

func (a *StreamableHTTPAdapter) OnNotification(handler func(notification mcp.JSONRPCNotification)) {
	// Streamable HTTP doesn't support server-sent notifications
}
