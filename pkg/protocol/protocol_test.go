package protocol

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNewEventDefaults(t *testing.T) {
	before := time.Now()
	ev := NewEvent(EventTypeTextDelta, EventOptions{
		RequestID: "req_123",
		Payload:   TextPayloadMap(TextPayload{Text: "hello"}),
	})
	after := time.Now()

	if ev.Version != VersionV1 {
		t.Fatalf("version=%q, want %q", ev.Version, VersionV1)
	}
	if ev.EventID == "" {
		t.Fatal("expected generated event id")
	}
	if ev.EventType != EventTypeTextDelta {
		t.Fatalf("event_type=%q, want %q", ev.EventType, EventTypeTextDelta)
	}
	if ev.Source != SourceCore {
		t.Fatalf("source=%q, want %q", ev.Source, SourceCore)
	}
	if ev.Timestamp.Before(before) || ev.Timestamp.After(after.Add(2*time.Second)) {
		t.Fatalf("timestamp=%s outside expected window", ev.Timestamp)
	}
	if got := ev.Payload["text"]; got != "hello" {
		t.Fatalf("payload[text]=%v, want hello", got)
	}
}

func TestNewEventClonesPayload(t *testing.T) {
	payload := map[string]any{"text": "hello"}
	ev := NewEvent(EventTypeTextFinal, EventOptions{Payload: payload})
	payload["text"] = "mutated"

	if got := ev.Payload["text"]; got != "hello" {
		t.Fatalf("payload mutated, got %v", got)
	}
}

func TestEnvelopeJSONShape(t *testing.T) {
	ts := time.Date(2026, 4, 2, 20, 30, 0, 0, time.FixedZone("CST", 8*3600))
	ev := NewEvent(EventTypeApprovalReq, EventOptions{
		EventID:       "evt_fixed",
		SessionID:     "sess_01",
		ThreadID:      "thread_01",
		RequestID:     "req_01",
		CorrelationID: "approval_01",
		Timestamp:     ts,
		Source:        SourceServe,
		Payload: ApprovalRequestPayload(ApprovalRequest{
			ApprovalID: "approval_01",
			Title:      "执行确认",
			Message:    "是否继续？",
			RiskLevel:  "high",
			Options:    []string{"allow_once", "deny"},
		}),
	})

	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded["version"] != string(VersionV1) {
		t.Fatalf("version=%v, want %s", decoded["version"], VersionV1)
	}
	if decoded["event_type"] != string(EventTypeApprovalReq) {
		t.Fatalf("event_type=%v, want %s", decoded["event_type"], EventTypeApprovalReq)
	}
	if decoded["source"] != string(SourceServe) {
		t.Fatalf("source=%v, want %s", decoded["source"], SourceServe)
	}
	payload, ok := decoded["payload"].(map[string]any)
	if !ok {
		t.Fatalf("payload type=%T, want map[string]any", decoded["payload"])
	}
	if payload["approval_id"] != "approval_01" {
		t.Fatalf("approval_id=%v, want approval_01", payload["approval_id"])
	}
}

func TestRequestLifecycleEnvelopeCarriesStableFields(t *testing.T) {
	ts := time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC)
	ev := NewEvent(EventTypeRequestStarted, EventOptions{
		EventID:       "evt_req_start",
		SessionID:     "sess_01",
		ThreadID:      "thread_01",
		RequestID:     "req_01",
		CorrelationID: "req_01",
		Timestamp:     ts,
		Source:        SourceServe,
		Payload: map[string]any{
			"input":  "echo hi",
			"status": "running",
		},
	})

	if ev.Version != VersionV1 {
		t.Fatalf("version=%q, want %q", ev.Version, VersionV1)
	}
	if ev.EventID != "evt_req_start" {
		t.Fatalf("event_id=%q, want evt_req_start", ev.EventID)
	}
	if ev.EventType != EventTypeRequestStarted {
		t.Fatalf("event_type=%q, want %q", ev.EventType, EventTypeRequestStarted)
	}
	if ev.SessionID != "sess_01" || ev.ThreadID != "thread_01" {
		t.Fatalf("unexpected session/thread ids: %#v", ev)
	}
	if ev.RequestID != "req_01" || ev.CorrelationID != "req_01" {
		t.Fatalf("unexpected request ids: %#v", ev)
	}
	if !ev.Timestamp.Equal(ts) {
		t.Fatalf("timestamp=%s, want %s", ev.Timestamp, ts)
	}
	if got := ev.Payload["status"]; got != "running" {
		t.Fatalf("payload[status]=%v, want running", got)
	}
}

func TestValidateEnvelopeAcceptsApprovalEvent(t *testing.T) {
	ev := NewEvent(EventTypeApprovalReq, EventOptions{
		RequestID: "approval_01",
		Source:    SourceServe,
		Payload: ApprovalRequestPayload(ApprovalRequest{
			ApprovalID: "approval_01",
			Message:    "continue?",
		}),
	})

	if err := ValidateEnvelope(ev); err != nil {
		t.Fatalf("ValidateEnvelope() error = %v", err)
	}
}

func TestValidateEnvelopeRejectsMissingRequestID(t *testing.T) {
	ev := NewEvent(EventTypeRequestDone, EventOptions{
		Source: SourceCore,
		Payload: map[string]any{
			"status": "success",
		},
	})

	err := ValidateEnvelope(ev)
	if err == nil {
		t.Fatal("ValidateEnvelope() error = nil, want request_id failure")
	}
	if !strings.Contains(err.Error(), "request_id") {
		t.Fatalf("ValidateEnvelope() error = %v, want request_id mention", err)
	}
}

func TestValidateEnvelopeRejectsMissingSessionPayload(t *testing.T) {
	ev := NewEvent(EventTypeSessionUpdated, EventOptions{
		SessionID: "sess_01",
		Source:    SourceServe,
		Payload: map[string]any{
			"status": "idle",
		},
	})

	err := ValidateEnvelope(ev)
	if err == nil {
		t.Fatal("ValidateEnvelope() error = nil, want session payload failure")
	}
	if !strings.Contains(err.Error(), "payload.session_id") {
		t.Fatalf("ValidateEnvelope() error = %v, want payload.session_id mention", err)
	}
}
