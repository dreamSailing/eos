package core

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/dreamSailing/vb-coding/internal/bridge"
	"github.com/dreamSailing/vb-coding/pkg/protocol"
)

func TestToRuntimeMode(t *testing.T) {
	if got := toRuntimeMode("手动确认"); got != "manual" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := toRuntimeMode("计划优先"); got != "plan" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := toRuntimeMode("自动无人值守"); got != "auto" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := toRuntimeMode("unknown"); got != "auto" {
		t.Fatalf("unexpected mode: %s", got)
	}
}

func TestFromRuntimeMode(t *testing.T) {
	if got := fromRuntimeMode("manual"); got != "手动确认" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := fromRuntimeMode("plan"); got != "计划优先" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := fromRuntimeMode("auto"); got != "自动无人值守" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := fromRuntimeMode("unknown"); got != "自动无人值守" {
		t.Fatalf("unexpected mode: %s", got)
	}
}

func TestFilterTrustedWorkspaces(t *testing.T) {
	target := filepath.Join("C:", "Users", "tester", "demo")
	trusted := []string{
		target,
		filepath.Join("C:", "Users", "tester", "keep"),
	}
	filtered, changed := filterTrustedWorkspaces(trusted, filepath.Join("C:", "Users", "tester", "demo", "."))
	if !changed {
		t.Fatal("expected target workspace to be removed from trusted list")
	}
	want := []string{filepath.Join("C:", "Users", "tester", "keep")}
	if !reflect.DeepEqual(filtered, want) {
		t.Fatalf("filtered=%v, want %v", filtered, want)
	}
}

func TestFilterTrustedWorkspacesNoMatch(t *testing.T) {
	trusted := []string{filepath.Join("C:", "Users", "tester", "keep")}
	filtered, changed := filterTrustedWorkspaces(trusted, filepath.Join("C:", "Users", "tester", "demo"))
	if changed {
		t.Fatal("expected no trusted workspace removal")
	}
	if !reflect.DeepEqual(filtered, trusted) {
		t.Fatalf("filtered=%v, want %v", filtered, trusted)
	}
}

func TestLegacyEventToProtocolMapsTextFinal(t *testing.T) {
	ts := time.Date(2026, 4, 2, 21, 0, 0, 0, time.UTC)
	ev := legacyEventToProtocol(Event{
		Type:      "TextFinal",
		RequestID: "req_01",
		Message:   "done",
	}, "sess_01", "thread_01", ts)

	if ev.EventType != protocol.EventTypeTextFinal {
		t.Fatalf("event_type=%q, want %q", ev.EventType, protocol.EventTypeTextFinal)
	}
	if ev.SessionID != "sess_01" {
		t.Fatalf("session_id=%q, want sess_01", ev.SessionID)
	}
	if ev.ThreadID != "thread_01" {
		t.Fatalf("thread_id=%q, want thread_01", ev.ThreadID)
	}
	if ev.RequestID != "req_01" {
		t.Fatalf("request_id=%q, want req_01", ev.RequestID)
	}
	if got := ev.Payload["text"]; got != "done" {
		t.Fatalf("payload[text]=%v, want done", got)
	}
	if !ev.Timestamp.Equal(ts) {
		t.Fatalf("timestamp=%s, want %s", ev.Timestamp, ts)
	}
}

func TestLegacyEventToProtocolMapsApproval(t *testing.T) {
	ev := legacyEventToProtocol(Event{
		Type:      "ConfirmRequired",
		RequestID: "approval_01",
		Message:   "继续执行危险操作？",
		Data: map[string]any{
			"risk_level": "high",
			"options":    []string{"allow_once", "deny"},
		},
	}, "sess_01", "thread_01", time.Unix(0, 0))

	if ev.EventType != protocol.EventTypeApprovalReq {
		t.Fatalf("event_type=%q, want %q", ev.EventType, protocol.EventTypeApprovalReq)
	}
	if got := ev.Payload["approval_id"]; got != "approval_01" {
		t.Fatalf("payload[approval_id]=%v, want approval_01", got)
	}
	if got := ev.Payload["message"]; got != "继续执行危险操作？" {
		t.Fatalf("payload[message]=%v, want question", got)
	}
	if got := ev.Payload["risk_level"]; got != "high" {
		t.Fatalf("payload[risk_level]=%v, want high", got)
	}
}

func TestLegacyEventToProtocolMapsInquiry(t *testing.T) {
	ev := legacyEventToProtocol(Event{
		Type:      "Inquiry",
		RequestID: "inq_01",
		Message:   "选择一个工作区",
		Data: map[string]any{
			"options": []string{"A", "B"},
		},
	}, "", "", time.Unix(0, 0))

	if ev.EventType != protocol.EventTypeInquiryReq {
		t.Fatalf("event_type=%q, want %q", ev.EventType, protocol.EventTypeInquiryReq)
	}
	if got := ev.Payload["inquiry_id"]; got != "inq_01" {
		t.Fatalf("payload[inquiry_id]=%v, want inq_01", got)
	}
	if got := ev.Payload["question"]; got != "选择一个工作区" {
		t.Fatalf("payload[question]=%v, want message", got)
	}
}

func TestBridgeEventToProtocolMapsApprovalRequired(t *testing.T) {
	ev, ok := bridgeEventToProtocol(bridge.Event{
		Type: "approval.required",
		RID:  "req-1",
		Data: map[string]any{
			"approval_id": "req-1",
			"message":     "需要确认",
		},
	}, "session-a", "thread-a", "", time.Unix(1710000000, 0))
	if !ok {
		t.Fatalf("bridgeEventToProtocol should map approval.required")
	}
	if ev.EventType != protocol.EventTypeApprovalReq {
		t.Fatalf("EventType=%q, want %q", ev.EventType, protocol.EventTypeApprovalReq)
	}
	if ev.RequestID != "req-1" {
		t.Fatalf("RequestID=%q, want req-1", ev.RequestID)
	}
	if got := ev.Payload["message"]; got != "需要确认" {
		t.Fatalf("payload message=%v, want 需要确认", got)
	}
}

func TestBridgeEventToProtocolUsesFallbackRequestID(t *testing.T) {
	ev, ok := bridgeEventToProtocol(bridge.Event{
		Type: "text.delta",
		Data: map[string]any{
			"text": "hello",
		},
	}, "session-a", "thread-a", "req-fallback", time.Unix(1710000001, 0))
	if !ok {
		t.Fatalf("bridgeEventToProtocol should map text.delta")
	}
	if ev.RequestID != "req-fallback" {
		t.Fatalf("RequestID=%q, want req-fallback", ev.RequestID)
	}
	if ev.CorrelationID != "req-fallback" {
		t.Fatalf("CorrelationID=%q, want req-fallback", ev.CorrelationID)
	}
}

func TestMapBridgeEventSupportsProtocolTextDelta(t *testing.T) {
	ev, ok := mapBridgeEvent(bridge.Event{
		Type: "text.delta",
		RID:  "req-2",
		Data: map[string]any{
			"text": "hello",
		},
	})
	if !ok {
		t.Fatalf("mapBridgeEvent should map text.delta")
	}
	if ev.Type != "TextDelta" {
		t.Fatalf("Type=%q, want TextDelta", ev.Type)
	}
	if ev.Message != "hello" {
		t.Fatalf("Message=%q, want hello", ev.Message)
	}
}
