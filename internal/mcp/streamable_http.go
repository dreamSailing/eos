package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// StreamableHTTPClient implements MCP transport over Streamable HTTP
type StreamableHTTPClient struct {
	baseURL    string
	headers    map[string]string
	sessionID  string
	httpClient *http.Client
	mu         sync.Mutex
}

// NewStreamableHTTPClient creates a new Streamable HTTP MCP client
func NewStreamableHTTPClient(baseURL string, headers map[string]string) *StreamableHTTPClient {
	return &StreamableHTTPClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		headers: headers,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Initialize sends the MCP initialize request
func (c *StreamableHTTPClient) Initialize(ctx context.Context) (map[string]interface{}, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      uuid.New().String(),
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "vb-coding",
				"version": "1.0.0",
			},
		},
	}

	resp, err := c.sendRequest(ctx, reqBody)
	if err != nil {
		return nil, fmt.Errorf("MCP initialize failed: %w", err)
	}

	// Extract session ID from response headers
	if resp.Header != nil {
		if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
			c.mu.Lock()
			c.sessionID = sid
			c.mu.Unlock()
		}
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse initialize response: %w", err)
	}

	return result, nil
}

// ListTools sends a tools/list request
func (c *StreamableHTTPClient) ListTools(ctx context.Context) ([]interface{}, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      uuid.New().String(),
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	resp, err := c.sendRequest(ctx, reqBody)
	if err != nil {
		return nil, fmt.Errorf("MCP tools/list failed: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools response: %w", err)
	}

	if tools, ok := result["tools"].([]interface{}); ok {
		return tools, nil
	}

	return nil, fmt.Errorf("no tools found in response")
}

// CallTool sends a tools/call request
func (c *StreamableHTTPClient) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (interface{}, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      uuid.New().String(),
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": arguments,
		},
	}

	resp, err := c.sendRequest(ctx, reqBody)
	if err != nil {
		return nil, fmt.Errorf("MCP tools/call failed: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tool call response: %w", err)
	}

	return result, nil
}

type mcpResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

func (c *StreamableHTTPClient) sendRequest(ctx context.Context, body map[string]interface{}) (*mcpResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	c.mu.Lock()
	sid := c.sessionID
	c.mu.Unlock()
	if sid != "" {
		req.Header.Set("Mcp-Session-Id", sid)
	}

	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle SSE response
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") {
		return c.handleSSEResponse(resp)
	}

	// Regular JSON response
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &mcpResponse{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       respBody,
	}, nil
}

func (c *StreamableHTTPClient) handleSSEResponse(resp *http.Response) (*mcpResponse, error) {
	scanner := bufio.NewScanner(resp.Body)
	var lastData []byte

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "" {
				continue
			}
			lastData = []byte(data)

			// Check if this is the final response
			var parsed map[string]interface{}
			if err := json.Unmarshal(lastData, &parsed); err == nil {
				if _, hasResult := parsed["result"]; hasResult {
					break
				}
			}
		}
	}

	if len(lastData) == 0 {
		return nil, fmt.Errorf("no data received from SSE stream")
	}

	return &mcpResponse{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       lastData,
	}, nil
}

// Close terminates the session
func (c *StreamableHTTPClient) Close() error {
	// Send a DELETE request to terminate the session if we have a session ID
	c.mu.Lock()
	sid := c.sessionID
	c.mu.Unlock()

	if sid == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Mcp-Session-Id", sid)

	_, err = c.httpClient.Do(req)
	return err
}

// ListResources sends a resources/list request
func (c *StreamableHTTPClient) ListResources(ctx context.Context) ([]interface{}, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      uuid.New().String(),
		"method":  "resources/list",
		"params":  map[string]interface{}{},
	}

	resp, err := c.sendRequest(ctx, reqBody)
	if err != nil {
		return nil, fmt.Errorf("MCP resources/list failed: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse resources response: %w", err)
	}

	if resources, ok := result["resources"].([]interface{}); ok {
		return resources, nil
	}

	return nil, nil
}

// ReadResource sends a resources/read request
func (c *StreamableHTTPClient) ReadResource(ctx context.Context, uri string) (interface{}, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      uuid.New().String(),
		"method":  "resources/read",
		"params": map[string]interface{}{
			"uri": uri,
		},
	}

	resp, err := c.sendRequest(ctx, reqBody)
	if err != nil {
		return nil, fmt.Errorf("MCP resources/read failed: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse resource read response: %w", err)
	}

	return result, nil
}

// ListPrompts sends a prompts/list request
func (c *StreamableHTTPClient) ListPrompts(ctx context.Context) ([]interface{}, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      uuid.New().String(),
		"method":  "prompts/list",
		"params":  map[string]interface{}{},
	}

	resp, err := c.sendRequest(ctx, reqBody)
	if err != nil {
		return nil, fmt.Errorf("MCP prompts/list failed: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse prompts response: %w", err)
	}

	if prompts, ok := result["prompts"].([]interface{}); ok {
		return prompts, nil
	}

	return nil, nil
}

// GetPrompt sends a prompts/get request
func (c *StreamableHTTPClient) GetPrompt(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	params := map[string]interface{}{
		"name": name,
	}
	if len(args) > 0 {
		params["arguments"] = args
	}

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      uuid.New().String(),
		"method":  "prompts/get",
		"params":  params,
	}

	resp, err := c.sendRequest(ctx, reqBody)
	if err != nil {
		return nil, fmt.Errorf("MCP prompts/get failed: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse prompt get response: %w", err)
	}

	return result, nil
}
