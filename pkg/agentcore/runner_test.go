package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeModelRunner struct {
	req  ModelRequest
	resp ModelResponse
	err  error
}

func (r *fakeModelRunner) RunModel(_ context.Context, req ModelRequest) (ModelResponse, error) {
	r.req = req
	return r.resp, r.err
}

type fakeAgentEventSink struct {
	events []AgentEvent
}

func (s *fakeAgentEventSink) PublishAgentEvent(_ context.Context, event AgentEvent) error {
	s.events = append(s.events, event)
	return nil
}

type fakeToolRunner struct {
	call   ToolCall
	output ToolOutput
	err    error
}

func (r *fakeToolRunner) RunTool(_ context.Context, call ToolCall) (ToolOutput, error) {
	r.call = call
	return r.output, r.err
}

func TestRunnerRunOnceUsesModelAdapterAndMailbox(t *testing.T) {
	registry := NewRegistry(NewDefaultRoleRegistry())
	box := NewMailbox()
	agent, err := registry.RegisterRootWithTask("senior_dev", "build feature")
	if err != nil {
		t.Fatalf("RegisterRootWithTask() error = %v", err)
	}
	if err := box.Send(MailboxMessage{FromAgentID: "user", ToAgentID: agent.ID, Body: "continue"}); err != nil {
		t.Fatalf("Mailbox.Send() error = %v", err)
	}
	model := &fakeModelRunner{resp: ModelResponse{Text: "done"}}
	sink := &fakeAgentEventSink{}
	runner := NewRunner(registry, box, WithModelRunner(model), WithAgentEventSink(sink))

	result, err := runner.RunOnce(context.Background(), agent.ID, json.RawMessage(`{"trace":true}`))
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Agent.Status != AgentCompleted || result.Output != "done" {
		t.Fatalf("result=%+v, want completed output", result)
	}
	if model.req.Agent.ID != agent.ID || model.req.Role.ID != "senior-dev" || model.req.Task != "build feature" {
		t.Fatalf("model request=%+v, want agent role task", model.req)
	}
	if len(model.req.Messages) != 1 || model.req.Messages[0].Body != "continue" {
		t.Fatalf("messages=%+v, want mailbox message", model.req.Messages)
	}
	if string(model.req.Options) != `{"trace":true}` {
		t.Fatalf("options=%s, want trace option", model.req.Options)
	}
	if got, _ := registry.Get(agent.ID); got.Status != AgentCompleted {
		t.Fatalf("registry status=%q, want completed", got.Status)
	}
	if len(sink.events) != 2 || sink.events[0].Status != AgentRunning || sink.events[1].Status != AgentCompleted {
		t.Fatalf("events=%+v, want running/completed", sink.events)
	}
}

func TestRunnerRunOnceMarksFailureWhenModelAdapterFails(t *testing.T) {
	registry := NewRegistry(NewDefaultRoleRegistry())
	agent, err := registry.RegisterRoot("reviewer")
	if err != nil {
		t.Fatalf("RegisterRoot() error = %v", err)
	}
	modelErr := errors.New("model unavailable")
	sink := &fakeAgentEventSink{}
	runner := NewRunner(registry, NewMailbox(), WithModelRunner(&fakeModelRunner{err: modelErr}), WithAgentEventSink(sink))

	if _, err := runner.RunOnce(context.Background(), agent.ID, nil); !errors.Is(err, modelErr) {
		t.Fatalf("RunOnce() error = %v, want model error", err)
	}
	got, _ := registry.Get(agent.ID)
	if got.Status != AgentFailed {
		t.Fatalf("status=%q, want failed", got.Status)
	}
	if len(sink.events) != 2 || sink.events[1].Status != AgentFailed || sink.events[1].Error != modelErr.Error() {
		t.Fatalf("events=%+v, want failed event", sink.events)
	}
}

func TestRunnerRunOnceRequiresModelAdapter(t *testing.T) {
	registry := NewRegistry(NewDefaultRoleRegistry())
	agent, err := registry.RegisterRoot("planner")
	if err != nil {
		t.Fatalf("RegisterRoot() error = %v", err)
	}
	runner := NewRunner(registry, NewMailbox())

	if _, err := runner.RunOnce(context.Background(), agent.ID, nil); !errors.Is(err, ErrNoModelRunner) {
		t.Fatalf("RunOnce() error = %v, want ErrNoModelRunner", err)
	}
	got, _ := registry.Get(agent.ID)
	if got.Status != AgentFailed {
		t.Fatalf("status=%q, want failed", got.Status)
	}
}

func TestRunnerRunToolEnforcesRoleAllowedTools(t *testing.T) {
	roles, err := NewRoleRegistry([]Role{{
		ID:           "tool-user",
		SystemPrompt: "use tools",
		AllowedTools: []string{"read", "fs/*"},
	}})
	if err != nil {
		t.Fatalf("NewRoleRegistry() error = %v", err)
	}
	registry := NewRegistry(roles)
	agent, err := registry.RegisterRoot("tool-user")
	if err != nil {
		t.Fatalf("RegisterRoot() error = %v", err)
	}
	tools := &fakeToolRunner{output: ToolOutput{Name: "read", Display: "ok", Output: json.RawMessage(`{"text":"hello"}`)}}
	sink := &fakeAgentEventSink{}
	runner := NewRunner(registry, NewMailbox(), WithToolRunner(tools), WithAgentEventSink(sink))

	output, err := runner.RunTool(context.Background(), agent.ID, "read", json.RawMessage(`{"path":"README.md"}`))
	if err != nil {
		t.Fatalf("RunTool(read) error = %v", err)
	}
	if tools.call.Agent.ID != agent.ID || tools.call.Name != "read" || string(tools.call.Args) != `{"path":"README.md"}` {
		t.Fatalf("tool call=%+v, want agent/read args", tools.call)
	}
	if output.Display != "ok" {
		t.Fatalf("output=%+v, want ok", output)
	}
	if len(sink.events) != 1 || sink.events[0].Message != "ok" {
		t.Fatalf("events=%+v, want tool progress event", sink.events)
	}

	if _, err := runner.RunTool(context.Background(), agent.ID, "fs/write", nil); err != nil {
		t.Fatalf("RunTool(fs/write) error = %v", err)
	}
	if _, err := runner.RunTool(context.Background(), agent.ID, "fs", nil); err != nil {
		t.Fatalf("RunTool(fs) error = %v", err)
	}
	if _, err := runner.RunTool(context.Background(), agent.ID, "fs.read", nil); err != nil {
		t.Fatalf("RunTool(fs.read) error = %v", err)
	}
	if _, err := runner.RunTool(context.Background(), agent.ID, "bash", nil); !errors.Is(err, ErrToolNotAllowed) {
		t.Fatalf("RunTool(bash) error = %v, want ErrToolNotAllowed", err)
	}
}

func TestRunnerRunToolRequiresToolRunner(t *testing.T) {
	registry := NewRegistry(NewDefaultRoleRegistry())
	agent, err := registry.RegisterRoot("planner")
	if err != nil {
		t.Fatalf("RegisterRoot() error = %v", err)
	}
	runner := NewRunner(registry, NewMailbox())

	if _, err := runner.RunTool(context.Background(), agent.ID, "read", nil); !errors.Is(err, ErrNoToolRunner) {
		t.Fatalf("RunTool() error = %v, want ErrNoToolRunner", err)
	}
}
