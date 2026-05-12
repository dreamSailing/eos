package mcp_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	eosmcp "github.com/dreamSailing/eos/internal/mcp"
	toolapiimpl "github.com/dreamSailing/eos/internal/toolapi/impl"
)

func TestMCPServerStdio_ListToolsAndResources(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	resp := client.request(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("initialize returned error: %#v", resp["error"])
	}

	resp = client.request(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]any{},
	})
	result := asMap(t, resp["result"])
	tools := asSlice(t, result["tools"])
	if len(tools) == 0 {
		t.Fatalf("expected tools, got none")
	}
	if !containsTool(tools, "eos_session_create") {
		t.Fatalf("expected eos_session_create in tools list")
	}
	if !containsTool(tools, "time_now") {
		t.Fatalf("expected time_now in tools list")
	}

	resp = client.request(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "resources/list",
		"params":  map[string]any{},
	})
	result = asMap(t, resp["result"])
	resources := asSlice(t, result["resources"])
	if len(resources) == 0 {
		t.Fatalf("expected resources, got none")
	}

	resp = client.request(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "resources/read",
		"params": map[string]any{
			"uri": "eos://sessions",
		},
	})
	result = asMap(t, resp["result"])
	contents := asSlice(t, result["contents"])
	if len(contents) == 0 {
		t.Fatalf("expected contents, got none")
	}
	first := asMap(t, contents[0])
	text := stringValue(first["text"])
	if !strings.Contains(text, "sessions") {
		t.Fatalf("expected sessions payload, got %q", text)
	}
}

func TestMCPServerStdio_DefaultSessionExposesAccessAndApprovalModes(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	_ = client.request(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
		},
	})

	resp := client.request(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "eos_session_get",
			"arguments": map[string]any{},
		},
	})
	result := asMap(t, resp["result"])
	structured := asMap(t, result["structuredContent"])
	session := asMap(t, structured["session"])
	if got := stringValue(session["access_mode"]); got != "workspace-write" {
		t.Fatalf("access_mode=%q, want workspace-write", got)
	}
	if got := stringValue(session["approval_mode"]); got != "on-request" {
		t.Fatalf("approval_mode=%q, want on-request", got)
	}
	if got := stringValue(session["sandbox_mode"]); got != "workspace" {
		t.Fatalf("sandbox_mode=%q, want workspace", got)
	}
}

func TestMCPServerStdio_AskUserQuestionInquiryFlow(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	_ = client.request(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
		},
	})

	resp := client.request(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "ask_user_question",
			"arguments": map[string]any{
				"question": "选择模式？",
				"options":  []string{"auto", "plan"},
			},
		},
	})
	result := asMap(t, resp["result"])
	if result["isError"] != true {
		t.Fatalf("expected inquiry-required error result, got %#v", result)
	}
	structured := asMap(t, result["structuredContent"])
	requestID := stringValue(structured["request_id"])
	if requestID == "" {
		t.Fatalf("expected request_id in structured content")
	}

	resp = client.request(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "eos_inquiry_resolve",
			"arguments": map[string]any{
				"request_id": requestID,
				"option":     "auto",
				"text":       "use auto",
			},
		},
	})
	result = asMap(t, resp["result"])
	if result["isError"] == true {
		t.Fatalf("expected inquiry resolve success, got %#v", result)
	}

	resp = client.request(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "ask_user_question",
			"arguments": map[string]any{
				"question": "选择模式？",
				"options":  []string{"auto", "plan"},
			},
		},
	})
	result = asMap(t, resp["result"])
	if result["isError"] == true {
		t.Fatalf("expected resolved inquiry success, got %#v", result)
	}
	structured = asMap(t, result["structuredContent"])
	if got := stringValue(structured["option"]); got != "auto" {
		t.Fatalf("expected option auto, got %q", got)
	}
	if got := stringValue(structured["text"]); got != "use auto" {
		t.Fatalf("expected text use auto, got %q", got)
	}
}

type testClient struct {
	in     *io.PipeWriter
	out    *bufio.Reader
	cancel context.CancelFunc
	done   chan error
}

func startTestServer(t *testing.T) (*testClient, func()) {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())

	srv, err := eosmcp.NewServer(eosmcp.ServerOptions{
		Transport:             "stdio",
		DefaultWorkspacePath:  "/workspace",
		DefaultSandboxMode:    "workspace",
		RequireApprovalDigest: true,
	}, toolapiimpl.NewServices())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- srv.RunStdio(ctx, inR, outW, io.Discard)
	}()

	client := &testClient{
		in:     inW,
		out:    bufio.NewReader(outR),
		cancel: cancel,
		done:   done,
	}

	cleanup := func() {
		cancel()
		_ = inW.Close()
		_ = outW.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("server did not stop")
		}
	}
	return client, cleanup
}

func (c *testClient) request(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, err := c.in.Write(append(b, '\n')); err != nil {
		t.Fatalf("write request: %v", err)
	}
	line, err := c.out.ReadString('\n')
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(line), &out); err != nil {
		t.Fatalf("unmarshal response: %v; line=%s", err, line)
	}
	return out
}

func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	out, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	return out
}

func asSlice(t *testing.T, v any) []any {
	t.Helper()
	out, ok := v.([]any)
	if !ok {
		t.Fatalf("expected slice, got %T", v)
	}
	return out
}

func containsTool(items []any, name string) bool {
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if stringValue(m["name"]) == name {
			return true
		}
	}
	return false
}

func stringValue(v any) string {
	text, _ := v.(string)
	return text
}
