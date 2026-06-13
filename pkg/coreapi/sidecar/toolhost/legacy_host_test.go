//go:build legacy

package toolhost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dreamSailing/eos/pkg/protocol/jsonrpc"
)

func TestLegacyHostExecuteSuccess(t *testing.T) {
	runner := ToolRunnerFunc(func(_ context.Context, name string, _ json.RawMessage) (json.RawMessage, string, error) {
		return json.RawMessage(`{"result":"ok"}`), fmt.Sprintf("executed %s", name), nil
	})
	host := &LegacyHost{Runner: runner}

	resp, err := host.Execute(context.Background(), ExecuteRequest{
		Name:      "read",
		RequestID: "req_1",
		Args:      json.RawMessage(`{"path":"/tmp/test"}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q, want ok", resp.Status)
	}
	if resp.Name != "read" {
		t.Errorf("name = %q, want read", resp.Name)
	}
	if resp.RequestID != "req_1" {
		t.Errorf("request_id = %q, want req_1", resp.RequestID)
	}
	if !bytes.Contains(resp.Output, []byte(`"result":"ok"`)) {
		t.Errorf("output = %s, want result:ok", resp.Output)
	}
	if resp.DurationMs == nil || *resp.DurationMs < 0 {
		t.Errorf("duration_ms = %v, want >= 0", resp.DurationMs)
	}
}

func TestLegacyHostRunnerError(t *testing.T) {
	runner := ToolRunnerFunc(func(_ context.Context, _ string, _ json.RawMessage) (json.RawMessage, string, error) {
		return nil, "display msg", fmt.Errorf("tool not found: unknown_tool")
	})
	host := &LegacyHost{Runner: runner}

	resp, err := host.Execute(context.Background(), ExecuteRequest{
		Name:      "unknown_tool",
		RequestID: "req_err",
	})
	if err != nil {
		t.Fatalf("Execute should not return Go error for tool-level errors: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("status = %q, want error", resp.Status)
	}
	if !strings.Contains(resp.Error, "tool not found") {
		t.Errorf("error = %q, want contains 'tool not found'", resp.Error)
	}
	if resp.Name != "unknown_tool" {
		t.Errorf("name = %q, want unknown_tool", resp.Name)
	}
}

func TestLegacyHostNilRunner(t *testing.T) {
	host := &LegacyHost{Runner: nil}

	_, err := host.Execute(context.Background(), ExecuteRequest{Name: "read"})
	if err == nil {
		t.Fatal("expected error for nil runner")
	}
	if !strings.Contains(err.Error(), "runner not initialized") {
		t.Errorf("error = %q, want 'runner not initialized'", err.Error())
	}
}

func TestLegacyHostListCatalogUsesExplicitCatalogRunner(t *testing.T) {
	host := &LegacyHost{
		Runner: ToolRunnerFunc(func(context.Context, string, json.RawMessage) (json.RawMessage, string, error) {
			return json.RawMessage(`{}`), "ok", nil
		}),
		Catalog: ToolCatalogRunnerFunc(func(_ context.Context, req CatalogRequest) ([]ToolDefinition, error) {
			if req.WorkspaceRoot != "/workspace" {
				t.Fatalf("workspace_root=%q, want /workspace", req.WorkspaceRoot)
			}
			return []ToolDefinition{{Name: "read", RiskLevel: "low", ReadOnly: true, Invocable: true}}, nil
		}),
	}

	defs, err := host.ListCatalog(context.Background(), CatalogRequest{WorkspaceRoot: "/workspace"})
	if err != nil {
		t.Fatalf("ListCatalog error: %v", err)
	}
	if len(defs) != 1 || defs[0].Name != "read" {
		t.Fatalf("defs=%+v, want read", defs)
	}
}

func TestLegacyHostListCatalogUsesRunnerWhenSupported(t *testing.T) {
	runner := catalogRunnerBoth{}
	host := &LegacyHost{Runner: runner}

	defs, err := host.ListCatalog(context.Background(), CatalogRequest{})
	if err != nil {
		t.Fatalf("ListCatalog error: %v", err)
	}
	if len(defs) != 1 || defs[0].Name != "bash" {
		t.Fatalf("defs=%+v, want bash", defs)
	}
}

func TestLegacyHostListCatalogWithoutRunner(t *testing.T) {
	host := &LegacyHost{Runner: ToolRunnerFunc(func(context.Context, string, json.RawMessage) (json.RawMessage, string, error) {
		return json.RawMessage(`{}`), "ok", nil
	})}
	_, err := host.ListCatalog(context.Background(), CatalogRequest{})
	if err == nil || !strings.Contains(err.Error(), "catalog runner not initialized") {
		t.Fatalf("error=%v, want catalog runner not initialized", err)
	}
}

func TestLegacyHostTimeout(t *testing.T) {
	runner := ToolRunnerFunc(func(ctx context.Context, _ string, _ json.RawMessage) (json.RawMessage, string, error) {
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-time.After(5 * time.Second):
			return json.RawMessage(`{}`), "ok", nil
		}
	})
	host := &LegacyHost{Runner: runner}

	timeout := int64(50)
	resp, err := host.Execute(context.Background(), ExecuteRequest{
		Name:      "slow_tool",
		RequestID: "req_timeout",
		TimeoutMs: &timeout,
	})
	if err != nil {
		t.Fatalf("Execute should return structured error, not Go error: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("status = %q, want error", resp.Status)
	}
	if !strings.Contains(resp.Error, "context deadline exceeded") {
		t.Errorf("error = %q, want contains 'context deadline exceeded'", resp.Error)
	}
}

type catalogRunnerBoth struct{}

func (catalogRunnerBoth) ExecuteTool(context.Context, string, json.RawMessage) (json.RawMessage, string, error) {
	return json.RawMessage(`{}`), "ok", nil
}

func (catalogRunnerBoth) ListTools(context.Context, CatalogRequest) ([]ToolDefinition, error) {
	return []ToolDefinition{{Name: "bash", RiskLevel: "high", Invocable: true}}, nil
}

func TestLegacyHostContextCancel(t *testing.T) {
	runner := ToolRunnerFunc(func(ctx context.Context, _ string, _ json.RawMessage) (json.RawMessage, string, error) {
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-time.After(5 * time.Second):
			return json.RawMessage(`{}`), "ok", nil
		}
	})
	host := &LegacyHost{Runner: runner}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resp, err := host.Execute(ctx, ExecuteRequest{Name: "cancel_tool", RequestID: "req_cancel"})
	if err != nil {
		t.Fatalf("Execute should return structured error, not Go error: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("status = %q, want error", resp.Status)
	}
	if !strings.Contains(resp.Error, "context canceled") {
		t.Errorf("error = %q, want contains 'context canceled'", resp.Error)
	}
}

func TestLegacyHostWorkspaceRootInContext(t *testing.T) {
	var capturedRoot string
	runner := ToolRunnerFunc(func(ctx context.Context, _ string, _ json.RawMessage) (json.RawMessage, string, error) {
		capturedRoot = WorkspaceRootFromCtx(ctx)
		return json.RawMessage(`{}`), "ok", nil
	})
	host := &LegacyHost{Runner: runner}

	_, err := host.Execute(context.Background(), ExecuteRequest{
		Name:          "read",
		WorkspaceRoot: "/workspace/project",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedRoot != "/workspace/project" {
		t.Errorf("workspace root = %q, want /workspace/project", capturedRoot)
	}
}

func TestLegacyHostRequestIDInContext(t *testing.T) {
	var capturedID string
	runner := ToolRunnerFunc(func(ctx context.Context, _ string, _ json.RawMessage) (json.RawMessage, string, error) {
		capturedID = RequestIDFromCtx(ctx)
		return json.RawMessage(`{}`), "ok", nil
	})
	host := &LegacyHost{Runner: runner}

	_, err := host.Execute(context.Background(), ExecuteRequest{
		Name:      "read",
		RequestID: "req_ctx_42",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedID != "req_ctx_42" {
		t.Errorf("request ID = %q, want req_ctx_42", capturedID)
	}
}

func TestLegacyHostInvalidArgsNoRealExecution(t *testing.T) {
	executed := false
	runner := ToolRunnerFunc(func(_ context.Context, _ string, args json.RawMessage) (json.RawMessage, string, error) {
		executed = true
		var params map[string]interface{}
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, "", fmt.Errorf("invalid args: %w", err)
		}
		return json.RawMessage(`{}`), "ok", nil
	})
	host := &LegacyHost{Runner: runner}

	resp, err := host.Execute(context.Background(), ExecuteRequest{
		Name:      "read",
		RequestID: "req_invalid",
		Args:      json.RawMessage(`{invalid json`),
	})
	if err != nil {
		t.Fatalf("Execute should not return Go error: %v", err)
	}
	if !executed {
		t.Fatal("runner should have been called — LegacyHost passes raw args through")
	}
	if resp.Status != "error" {
		t.Errorf("status = %q, want error (runner should reject invalid args)", resp.Status)
	}
}

func TestLegacyHostNoArgsDefaultsToEmpty(t *testing.T) {
	runner := ToolRunnerFunc(func(_ context.Context, _ string, args json.RawMessage) (json.RawMessage, string, error) {
		if len(args) != 0 {
			return nil, "", fmt.Errorf("expected empty args, got %s", args)
		}
		return json.RawMessage(`{}`), "ok", nil
	})
	host := &LegacyHost{Runner: runner}

	resp, err := host.Execute(context.Background(), ExecuteRequest{
		Name:      "time_now",
		RequestID: "req_noargs",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q, want ok", resp.Status)
	}
}

func TestLegacyHostServerIntegration(t *testing.T) {
	runner := ToolRunnerFunc(func(_ context.Context, name string, _ json.RawMessage) (json.RawMessage, string, error) {
		return json.RawMessage(fmt.Sprintf(`{"tool":"%s"}`, name)), fmt.Sprintf("executed %s", name), nil
	})
	host := &LegacyHost{Runner: runner}

	rpcReq, _ := jsonrpc.NewRequest(jsonrpc.NumberID(1), MethodToolExecute, ExecuteRequest{
		Name:      "read",
		RequestID: "req_server",
		Args:      json.RawMessage(`{"path":"/tmp/test"}`),
	})

	input := &bytes.Buffer{}
	output := &bytes.Buffer{}

	frame := marshalFrame(t, rpcReq)
	input.Write(frame)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		srv := NewServer(host)
		done <- srv.Serve(ctx, input, output)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("serve error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not shut down")
	}

	result := output.Bytes()
	if len(result) == 0 {
		t.Fatal("expected response output")
	}

	var resp jsonrpc.Response
	if err := unmarshalFrame(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("response error: %d %s", resp.Error.Code, resp.Error.Message)
	}

	var toolResp ExecuteResponse
	if err := json.Unmarshal(resp.Result, &toolResp); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}
	if toolResp.Status != "ok" {
		t.Errorf("status = %q, want ok", toolResp.Status)
	}
	if toolResp.Name != "read" {
		t.Errorf("name = %q, want read", toolResp.Name)
	}
	if toolResp.RequestID != "req_server" {
		t.Errorf("request_id = %q, want req_server", toolResp.RequestID)
	}
}

func TestToolRunnerFuncAdapter(t *testing.T) {
	called := false
	fn := ToolRunnerFunc(func(_ context.Context, name string, args json.RawMessage) (json.RawMessage, string, error) {
		called = true
		return json.RawMessage(`{}`), "ok", nil
	})

	output, display, err := fn.ExecuteTool(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("function was not called")
	}
	if display != "ok" {
		t.Errorf("display = %q, want ok", display)
	}
	if !bytes.Equal(output, []byte(`{}`)) {
		t.Errorf("output = %s, want {}", output)
	}
}

func TestLegacyHostJSONRoundtrip(t *testing.T) {
	runner := ToolRunnerFunc(func(_ context.Context, _ string, _ json.RawMessage) (json.RawMessage, string, error) {
		return json.RawMessage(`{"content":"hello world"}`), "read file", nil
	})
	host := &LegacyHost{Runner: runner}

	timeout := int64(5000)
	resp, err := host.Execute(context.Background(), ExecuteRequest{
		SessionID:     "sess_1",
		TurnID:        "turn_1",
		RequestID:     "req_roundtrip",
		AgentID:       "agent_1",
		Name:          "read",
		Args:          json.RawMessage(`{"path":"/tmp/test"}`),
		TimeoutMs:     &timeout,
		WorkspaceRoot: "/workspace",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ExecuteResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Status != "ok" {
		t.Errorf("status = %q, want ok", decoded.Status)
	}
	if decoded.RequestID != "req_roundtrip" {
		t.Errorf("request_id = %q, want req_roundtrip", decoded.RequestID)
	}
	if decoded.DurationMs == nil || *decoded.DurationMs < 0 {
		t.Errorf("duration_ms = %v, want >= 0", decoded.DurationMs)
	}
}
