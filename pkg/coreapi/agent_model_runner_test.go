package coreapi

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dreamSailing/eos/pkg/agentcore"
	"github.com/dreamSailing/eos/pkg/protocol"
)

type fakeTurnServiceForAgent struct {
	started     StartTurnRequest
	turn        Turn
	startErr    error
	interrupted TurnRef
}

func (s *fakeTurnServiceForAgent) Start(_ context.Context, req StartTurnRequest) (Turn, error) {
	s.started = req
	if s.startErr != nil {
		return Turn{}, s.startErr
	}
	if s.turn.ID == "" {
		s.turn = Turn{ID: req.TurnID, SessionID: req.SessionID, Status: "running", StartedAt: time.Now(), UpdatedAt: time.Now()}
	}
	return s.turn, nil
}

func (s *fakeTurnServiceForAgent) Interrupt(_ context.Context, ref TurnRef) error {
	s.interrupted = ref
	return nil
}

type fakeEventBusForAgent struct {
	filter EventFilter
	ch     chan protocol.Envelope
	err    error
}

func (b *fakeEventBusForAgent) Subscribe(_ context.Context, filter EventFilter) (<-chan protocol.Envelope, error) {
	b.filter = filter
	if b.err != nil {
		return nil, b.err
	}
	if b.ch == nil {
		b.ch = make(chan protocol.Envelope, 4)
	}
	return b.ch, nil
}

func (b *fakeEventBusForAgent) Publish(context.Context, protocol.Envelope) error { return nil }

func TestAgentTurnModelRunnerBuildsRoleAwareTurnAndWaitsForCompletion(t *testing.T) {
	turns := &fakeTurnServiceForAgent{}
	events := &fakeEventBusForAgent{ch: make(chan protocol.Envelope, 4)}
	runner := NewAgentTurnModelRunner(turns, events).WithSession("sess-1")
	req := agentcore.ModelRequest{
		Agent:           agentcore.Agent{ID: "agent/1", RoleID: "reviewer", Task: "inspect changes"},
		Role:            agentcore.Role{ID: "reviewer", SystemPrompt: "review carefully"},
		Task:            "inspect changes",
		Messages:        []agentcore.AgentMessage{{FromAgentID: "user", Body: "focus tests"}},
		AllowedTools:    []string{"read", "grep"},
		ContextStrategy: agentcore.ContextHybrid,
		Model:           "model-a",
		ReasoningEffort: "high",
	}
	events.ch <- protocol.NewEvent(protocol.EventTypeTextFinal, protocol.EventOptions{
		RequestID: "agent_turn_agent_1",
		Payload:   map[string]any{"text": "looks good"},
	})
	events.ch <- protocol.NewEvent(protocol.EventTypeRequestDone, protocol.EventOptions{
		RequestID: "agent_turn_agent_1",
		Payload:   map[string]any{"message": "done"},
	})

	resp, err := runner.RunModel(context.Background(), req)
	if err != nil {
		t.Fatalf("RunModel() error = %v", err)
	}
	if resp.Status != agentcore.AgentCompleted || resp.Text != "done" {
		t.Fatalf("resp=%+v, want completed done", resp)
	}
	if turns.started.SessionID != "sess-1" || turns.started.TurnID != "agent_turn_agent_1" {
		t.Fatalf("started=%+v, want session and stable turn id", turns.started)
	}
	if !strings.Contains(turns.started.Input, "Role: reviewer") ||
		!strings.Contains(turns.started.Input, "System prompt:") ||
		!strings.Contains(turns.started.Input, "Allowed tools: read, grep") ||
		!strings.Contains(turns.started.Input, "user: focus tests") {
		t.Fatalf("input=%q, missing role-aware sections", turns.started.Input)
	}
	if events.filter.SessionID != "sess-1" || events.filter.TurnID != "agent_turn_agent_1" || events.filter.AgentID != "agent/1" {
		t.Fatalf("filter=%+v, want session/turn/agent filter", events.filter)
	}
}

func TestAgentTurnModelRunnerReturnsFailureEventError(t *testing.T) {
	events := &fakeEventBusForAgent{ch: make(chan protocol.Envelope, 1)}
	events.ch <- protocol.NewEvent(protocol.EventTypeRequestFailed, protocol.EventOptions{
		RequestID: "agent_turn_agent-1",
		Payload:   map[string]any{"error": "boom"},
	})
	runner := NewAgentTurnModelRunner(&fakeTurnServiceForAgent{}, events)

	resp, err := runner.RunModel(context.Background(), agentcore.ModelRequest{
		Agent: agentcore.Agent{ID: "agent-1"},
		Role:  agentcore.Role{ID: "planner", SystemPrompt: "plan"},
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("RunModel() error = %v, want boom", err)
	}
	if resp.Status != agentcore.AgentFailed {
		t.Fatalf("status=%q, want failed", resp.Status)
	}
}

func TestAgentTurnModelRunnerInterruptsOnCancellation(t *testing.T) {
	turns := &fakeTurnServiceForAgent{}
	events := &fakeEventBusForAgent{ch: make(chan protocol.Envelope)}
	runner := NewAgentTurnModelRunner(turns, events)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resp, err := runner.RunModel(ctx, agentcore.ModelRequest{
		Agent: agentcore.Agent{ID: "agent-1"},
		Role:  agentcore.Role{ID: "planner", SystemPrompt: "plan"},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunModel() error = %v, want context canceled", err)
	}
	if resp.Status != agentcore.AgentCancelled {
		t.Fatalf("status=%q, want cancelled", resp.Status)
	}
	if turns.interrupted.TurnID != "agent_turn_agent-1" {
		t.Fatalf("interrupted=%+v, want agent turn interrupt", turns.interrupted)
	}
}
