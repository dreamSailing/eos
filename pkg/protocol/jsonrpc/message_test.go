package jsonrpc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dreamSailing/eos/pkg/protocol"
)

func TestRequestDoesNotEmitJSONRPCVersion(t *testing.T) {
	req, err := NewRequest(StringID("req_1"), "initialize", map[string]any{"client": "tui"})
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	data, err := Marshal(req)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	if strings.Contains(string(data), "jsonrpc") {
		t.Fatalf("wire message should not contain jsonrpc version: %s", data)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if decoded.Kind != KindRequest {
		t.Fatalf("kind=%s, want %s", decoded.Kind, KindRequest)
	}
	if decoded.Request.ID.String() != "req_1" {
		t.Fatalf("id=%q, want req_1", decoded.Request.ID.String())
	}
}

func TestDecodeNotificationWithProtocolEnvelopePayload(t *testing.T) {
	ev := protocol.NewEvent(protocol.EventTypeTextDelta, protocol.EventOptions{
		EventID:   "evt_fixed",
		RequestID: "turn_1",
		Timestamp: time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC),
		Payload:   map[string]any{"text": "hello"},
	})
	notification, err := NewNotification("event", ev)
	if err != nil {
		t.Fatalf("NewNotification() error = %v", err)
	}
	data, err := Marshal(notification)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if decoded.Kind != KindNotification {
		t.Fatalf("kind=%s, want %s", decoded.Kind, KindNotification)
	}

	var got protocol.Envelope
	if err := json.Unmarshal(decoded.Notification.Params, &got); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if got.EventType != protocol.EventTypeTextDelta {
		t.Fatalf("event_type=%q, want %q", got.EventType, protocol.EventTypeTextDelta)
	}
}

func TestResponseRejectsResultAndErrorTogether(t *testing.T) {
	resp := Response{
		ID:     NumberID(9),
		Result: json.RawMessage(`{"ok":true}`),
		Error:  &Error{Code: CodeInternalError, Message: "boom"},
	}
	err := resp.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want conflict")
	}
	if !strings.Contains(err.Error(), "both result and error") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestDecodeNumericRequestID(t *testing.T) {
	decoded, err := Decode([]byte(`{"id":42,"method":"state/snapshot"}`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if decoded.Request.ID.String() != "42" {
		t.Fatalf("id=%q, want 42", decoded.Request.ID.String())
	}
}

func TestRouterAndInProcessClient(t *testing.T) {
	router := NewRouter()
	if err := router.Register(MethodStateSnapshot, func(_ context.Context, req Request) (any, *Error) {
		return map[string]any{"method": req.Method, "ok": true}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := NewInProcessClient(InProcessServer{Router: router})
	var out map[string]any
	if err := client.Call(context.Background(), MethodStateSnapshot, nil, &out); err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if out["method"] != MethodStateSnapshot {
		t.Fatalf("method=%v, want %s", out["method"], MethodStateSnapshot)
	}
	if out["ok"] != true {
		t.Fatalf("ok=%v, want true", out["ok"])
	}
}
