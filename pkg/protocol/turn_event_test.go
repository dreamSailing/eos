package protocol

import (
	"encoding/json"
	"testing"
)

func TestEnvelopeRoundTripsTurnAndAgentID(t *testing.T) {
	event := NewEvent(EventTypeTurnStarted, EventOptions{
		SessionID: "session-1",
		TurnID:    "turn-1",
		AgentID:   "agent-1",
		Payload:   map[string]any{"message": "started"},
	})

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded Envelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if decoded.TurnID != "turn-1" {
		t.Fatalf("TurnID=%q, want turn-1", decoded.TurnID)
	}
	if decoded.AgentID != "agent-1" {
		t.Fatalf("AgentID=%q, want agent-1", decoded.AgentID)
	}
}

func TestNormalizeRustTurnEventTypes(t *testing.T) {
	tests := map[EventType]EventType{
		EventTypeTurnStarted:        EventTypeRequestStarted,
		EventTypeTurnTextDelta:      EventTypeTextDelta,
		EventTypeTurnReasoningDelta: EventTypeTextReasoning,
		EventTypeTurnCompleted:      EventTypeRequestDone,
		EventTypeTurnError:          EventTypeRequestFailed,
		EventTypeTurnCancelled:      EventTypeRequestFailed,
		EventTypeTurnInterrupted:    EventTypeRequestFailed,
	}
	for input, want := range tests {
		if got := NormalizeEventType(input); got != want {
			t.Fatalf("NormalizeEventType(%q)=%q, want %q", input, got, want)
		}
	}
}
