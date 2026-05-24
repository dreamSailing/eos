package coreapi

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dreamSailing/eos/pkg/agentcore"
)

type fakeToolExecutor struct {
	seen   ToolRequest
	result ToolResult
	err    error
}

func (e *fakeToolExecutor) Execute(_ context.Context, req ToolRequest) (ToolResult, error) {
	e.seen = req
	return e.result, e.err
}

func TestAgentToolRunnerAdaptsToolExecutor(t *testing.T) {
	executor := &fakeToolExecutor{
		result: ToolResult{
			Name:    "bash",
			Display: "ok",
			Output:  json.RawMessage(`{"stdout":"hello"}`),
		},
	}
	runner := NewAgentToolRunner(executor).WithSession("sess-1", "turn-1")

	output, err := runner.RunTool(context.Background(), agentcore.ToolCall{
		Agent: agentcore.Agent{ID: "agent/1"},
		Name:  " bash ",
		Args:  json.RawMessage(`{"command":"echo hello"}`),
	})
	if err != nil {
		t.Fatalf("RunTool() error = %v", err)
	}
	if executor.seen.SessionID != "sess-1" || executor.seen.TurnID != "turn-1" || executor.seen.RequestID != "turn-1" {
		t.Fatalf("seen=%+v, want session/turn request ids", executor.seen)
	}
	if executor.seen.AgentID != "agent/1" || executor.seen.Name != "bash" || string(executor.seen.Args) != `{"command":"echo hello"}` {
		t.Fatalf("seen=%+v, want agent bash args", executor.seen)
	}
	if output.Name != "bash" || output.Display != "ok" || string(output.Output) != `{"stdout":"hello"}` {
		t.Fatalf("output=%+v, want bash ok output", output)
	}
}

func TestAgentToolRunnerCreatesStableRequestIDWithoutTurn(t *testing.T) {
	executor := &fakeToolExecutor{}
	runner := NewAgentToolRunner(executor)

	if _, err := runner.RunTool(context.Background(), agentcore.ToolCall{
		Agent: agentcore.Agent{ID: "agent/1"},
		Name:  "fs.read",
	}); err != nil {
		t.Fatalf("RunTool() error = %v", err)
	}
	if executor.seen.RequestID != "agent_agent_1_fs_read" {
		t.Fatalf("RequestID=%q, want stable agent request id", executor.seen.RequestID)
	}
}

func TestAgentToolRunnerPreservesExecutorError(t *testing.T) {
	wantErr := errors.New("denied")
	executor := &fakeToolExecutor{err: wantErr}
	runner := NewAgentToolRunner(executor)

	output, err := runner.RunTool(context.Background(), agentcore.ToolCall{
		Agent: agentcore.Agent{ID: "agent-1"},
		Name:  "write",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("RunTool() error = %v, want %v", err, wantErr)
	}
	if output.Error != "denied" {
		t.Fatalf("output.Error=%q, want denied", output.Error)
	}
}

func TestAgentToolRunnerRequiresExecutor(t *testing.T) {
	if _, err := (AgentToolRunner{}).RunTool(context.Background(), agentcore.ToolCall{Name: "bash"}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("RunTool() error = %v, want ErrUnsupported", err)
	}
}
