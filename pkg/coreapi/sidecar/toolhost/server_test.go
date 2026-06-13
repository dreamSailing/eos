package toolhost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/dreamSailing/eos/pkg/protocol/jsonrpc"
)

func TestFakeHostDefault(t *testing.T) {
	host := &FakeHost{}
	resp, err := host.Execute(context.Background(), ExecuteRequest{
		Name:      "read_file",
		RequestID: "req_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q, want ok", resp.Status)
	}
	if resp.Name != "read_file" {
		t.Errorf("name = %q, want read_file", resp.Name)
	}
	if resp.RequestID != "req_1" {
		t.Errorf("request_id = %q, want req_1", resp.RequestID)
	}
	if !bytes.Contains(resp.Output, []byte(`"fake":true`)) {
		t.Errorf("output = %s, want fake:true", resp.Output)
	}
}

func TestFakeHostCustomCallback(t *testing.T) {
	host := &FakeHost{
		OnExecute: func(_ context.Context, req ExecuteRequest) (ExecuteResponse, error) {
			return ExecuteResponse{
				Name:      req.Name,
				RequestID: req.RequestID,
				Status:    "ok",
				Display:   "custom result",
				Output:    json.RawMessage(`{"custom":true}`),
			}, nil
		},
	}
	resp, err := host.Execute(context.Background(), ExecuteRequest{Name: "my_tool"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Display != "custom result" {
		t.Errorf("display = %q, want custom result", resp.Display)
	}
}

func TestFakeHostDefaultCatalog(t *testing.T) {
	host := &FakeHost{}
	defs, err := host.ListCatalog(context.Background(), CatalogRequest{})
	if err != nil {
		t.Fatalf("ListCatalog error: %v", err)
	}
	if len(defs) != 1 || defs[0].Name != "fake_tool" || defs[0].RiskLevel != "low" {
		t.Fatalf("defs=%+v, want fake low tool", defs)
	}
	if !bytes.Contains(defs[0].ParamsSchema, []byte(`"properties"`)) {
		t.Fatalf("params_schema=%s, want JSON schema", defs[0].ParamsSchema)
	}

	defs, err = host.ListCatalog(context.Background(), CatalogRequest{IncludeTools: []string{"missing"}})
	if err != nil {
		t.Fatalf("ListCatalog filtered error: %v", err)
	}
	if len(defs) != 0 {
		t.Fatalf("filtered defs=%+v, want empty", defs)
	}
}

func TestErrorHost(t *testing.T) {
	host := ErrorHost(fmt.Errorf("host unavailable"))
	_, err := host.Execute(context.Background(), ExecuteRequest{Name: "test"})
	if err == nil || err.Error() != "host unavailable" {
		t.Errorf("error = %v, want host unavailable", err)
	}
}

func TestServerServeWithFakeHost(t *testing.T) {
	host := &FakeHost{
		OnExecute: func(_ context.Context, req ExecuteRequest) (ExecuteResponse, error) {
			return ExecuteResponse{
				Name:      req.Name,
				RequestID: req.RequestID,
				Status:    "ok",
				Display:   fmt.Sprintf("executed %s", req.Name),
				Output:    json.RawMessage(`{"executed":true}`),
			}, nil
		},
	}

	rpcReq, _ := jsonrpc.NewRequest(jsonrpc.NumberID(1), MethodToolExecute, ExecuteRequest{
		Name:      "read_file",
		RequestID: "req_42",
		Args:      json.RawMessage(`{"path":"/tmp/test.txt"}`),
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
	if toolResp.Name != "read_file" {
		t.Errorf("name = %q, want read_file", toolResp.Name)
	}
	if toolResp.RequestID != "req_42" {
		t.Errorf("request_id = %q, want req_42", toolResp.RequestID)
	}
}

func TestServerToolCatalog(t *testing.T) {
	host := &FakeHost{
		OnCatalog: func(_ context.Context, req CatalogRequest) ([]ToolDefinition, error) {
			return []ToolDefinition{
				{Name: "read", RiskLevel: "low", Source: "test", ReadOnly: true, Invocable: true},
				{Name: "bash", RiskLevel: "high", Source: "test", Invocable: true},
			}, nil
		},
	}
	srv := NewServer(host)
	router := jsonrpc.NewRouter()
	if err := router.Register(MethodToolCatalog, srv.handleToolCatalog); err != nil {
		t.Fatalf("register: %v", err)
	}
	client := jsonrpc.NewInProcessClient(jsonrpc.InProcessServer{Router: router})

	var defs []ToolDefinition
	err := client.Call(context.Background(), MethodToolCatalog, CatalogRequest{
		WorkspaceRoot: "/workspace",
		IncludeTools:  []string{"read"},
	}, &defs)
	if err != nil {
		t.Fatalf("Call(tool/catalog) error: %v", err)
	}
	if len(defs) != 2 || defs[0].Name != "read" || defs[1].Name != "bash" {
		t.Fatalf("defs=%+v, want host returned catalog", defs)
	}
}

func TestServerToolCatalogInvalidParams(t *testing.T) {
	srv := NewServer(&FakeHost{})
	_, rpcErr := srv.handleToolCatalog(context.Background(), jsonrpc.Request{
		ID:     jsonrpc.NumberID(1),
		Method: MethodToolCatalog,
		Params: json.RawMessage(`{"include_tools":42}`),
	})
	if rpcErr == nil {
		t.Fatal("expected invalid params")
	}
	if rpcErr.Code != jsonrpc.CodeInvalidParams {
		t.Fatalf("code=%d, want %d", rpcErr.Code, jsonrpc.CodeInvalidParams)
	}
}

func TestServerInvalidParams(t *testing.T) {
	host := &FakeHost{}
	srv := NewServer(host)

	handler := srv.handleToolExecute

	_, rpcErr := handler(context.Background(), jsonrpc.Request{
		ID:     jsonrpc.NumberID(1),
		Method: MethodToolExecute,
		Params: json.RawMessage(`{}`),
	})
	if rpcErr == nil {
		t.Fatal("expected error for missing tool name")
	}
	if rpcErr.Code != jsonrpc.CodeInvalidParams {
		t.Errorf("code = %d, want %d", rpcErr.Code, jsonrpc.CodeInvalidParams)
	}
}

func TestServerMissingName(t *testing.T) {
	host := &FakeHost{}
	srv := NewServer(host)

	_, rpcErr := srv.handleToolExecute(context.Background(), jsonrpc.Request{
		ID:     jsonrpc.NumberID(1),
		Method: MethodToolExecute,
		Params: json.RawMessage(`{"name":""}`),
	})
	if rpcErr == nil {
		t.Fatal("expected error for empty tool name")
	}
	if rpcErr.Code != jsonrpc.CodeInvalidParams {
		t.Errorf("code = %d, want %d", rpcErr.Code, jsonrpc.CodeInvalidParams)
	}
	if !strings.Contains(rpcErr.Message, "tool name is required") {
		t.Errorf("message = %q, want tool name is required", rpcErr.Message)
	}
}

func TestServerHostError(t *testing.T) {
	host := ErrorHost(fmt.Errorf("internal failure"))
	srv := NewServer(host)

	_, rpcErr := srv.handleToolExecute(context.Background(), jsonrpc.Request{
		ID:     jsonrpc.NumberID(1),
		Method: MethodToolExecute,
		Params: marshalJSON(t, ExecuteRequest{Name: "fail_tool"}),
	})
	if rpcErr == nil {
		t.Fatal("expected error from host")
	}
	if rpcErr.Code != jsonrpc.CodeInternalError {
		t.Errorf("code = %d, want %d", rpcErr.Code, jsonrpc.CodeInternalError)
	}
	if !strings.Contains(rpcErr.Message, "internal failure") {
		t.Errorf("message = %q, want internal failure", rpcErr.Message)
	}
}

func TestServerCancelledContext(t *testing.T) {
	host := TimeoutHost(5 * time.Second)
	srv := NewServer(host)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, rpcErr := srv.handleToolExecute(ctx, jsonrpc.Request{
		ID:     jsonrpc.NumberID(1),
		Method: MethodToolExecute,
		Params: marshalJSON(t, ExecuteRequest{Name: "slow_tool", RequestID: "req_slow"}),
	})
	if rpcErr != nil {
		t.Fatalf("expected result for cancellation, got error: %v", rpcErr)
	}
	resp, ok := result.(ExecuteResponse)
	if !ok {
		t.Fatalf("expected ExecuteResponse, got %T", result)
	}
	if resp.Status != "error" {
		t.Errorf("status = %q, want error", resp.Status)
	}
	if !strings.Contains(resp.Error, "cancelled") {
		t.Errorf("error = %q, want cancelled", resp.Error)
	}
}

func TestServerMultipleRequests(t *testing.T) {
	callCount := 0
	host := &FakeHost{
		OnExecute: func(_ context.Context, req ExecuteRequest) (ExecuteResponse, error) {
			callCount++
			return ExecuteResponse{
				Name:      req.Name,
				RequestID: req.RequestID,
				Status:    "ok",
				Display:   fmt.Sprintf("call %d", callCount),
			}, nil
		},
	}

	srv := NewServer(host)
	router := jsonrpc.NewRouter()
	if err := router.Register(MethodToolExecute, srv.handleToolExecute); err != nil {
		t.Fatalf("register: %v", err)
	}
	client := jsonrpc.NewInProcessClient(jsonrpc.InProcessServer{Router: router})

	for i := 0; i < 5; i++ {
		var resp ExecuteResponse
		err := client.Call(context.Background(), MethodToolExecute, ExecuteRequest{
			Name:      fmt.Sprintf("tool_%d", i),
			RequestID: fmt.Sprintf("req_%d", i),
		}, &resp)
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if resp.Status != "ok" {
			t.Errorf("call %d: status = %q, want ok", i, resp.Status)
		}
		if resp.Name != fmt.Sprintf("tool_%d", i) {
			t.Errorf("call %d: name = %q, want tool_%d", i, resp.Name, i)
		}
	}

	if callCount != 5 {
		t.Errorf("callCount = %d, want 5", callCount)
	}
}

func TestExecuteRequestJSONRoundtrip(t *testing.T) {
	timeout := int64(5000)
	original := ExecuteRequest{
		SessionID:     "sess_1",
		TurnID:        "turn_1",
		RequestID:     "req_1",
		AgentID:       "agent_1",
		Name:          "read_file",
		Args:          json.RawMessage(`{"path":"/tmp/test"}`),
		TimeoutMs:     &timeout,
		WorkspaceRoot: "/workspace",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ExecuteRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.SessionID != original.SessionID {
		t.Errorf("session_id = %q, want %q", decoded.SessionID, original.SessionID)
	}
	if decoded.Name != original.Name {
		t.Errorf("name = %q, want %q", decoded.Name, original.Name)
	}
	if decoded.TimeoutMs == nil || *decoded.TimeoutMs != timeout {
		t.Errorf("timeout_ms = %v, want %d", decoded.TimeoutMs, timeout)
	}
	if decoded.WorkspaceRoot != original.WorkspaceRoot {
		t.Errorf("workspace_root = %q, want %q", decoded.WorkspaceRoot, original.WorkspaceRoot)
	}
}

func TestExecuteResponseJSONRoundtrip(t *testing.T) {
	duration := int64(150)
	original := ExecuteResponse{
		Name:       "read_file",
		RequestID:  "req_1",
		Status:     "ok",
		Display:    "file contents",
		Output:     json.RawMessage(`{"content":"hello"}`),
		Error:      "",
		DurationMs: &duration,
	}

	data, err := json.Marshal(original)
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
	if decoded.DurationMs == nil || *decoded.DurationMs != duration {
		t.Errorf("duration_ms = %v, want %d", decoded.DurationMs, duration)
	}
}

func marshalFrame(t *testing.T, v any) []byte {
	t.Helper()
	payload, err := jsonrpc.Marshal(v)
	if err != nil {
		t.Fatalf("marshal frame: %v", err)
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Content-Length: %d\r\n\r\n", len(payload))
	buf.Write(payload)
	return buf.Bytes()
}

func unmarshalFrame(data []byte, out any) error {
	reader := bytes.NewReader(data)
	stream := jsonrpc.NewStream(reader, io.Discard)
	payload, err := stream.ReadFrame()
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, out)
}

func marshalJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}
