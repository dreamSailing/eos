package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dreamSailing/eos/pkg/coreapi"
	"github.com/dreamSailing/eos/pkg/protocol"
	protocoljsonrpc "github.com/dreamSailing/eos/pkg/protocol/jsonrpc"
)

// --- Error golden tests: malformed request through ServeStream ---

func TestMalformedParamsThroughServeStreamReturnsInvalidParams(t *testing.T) {
	// When a request with malformed JSON params goes through the full ServeStream path
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    fakeStateService{},
		sessions: &fakeSessionService{},
	}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	var input bytes.Buffer
	var output bytes.Buffer

	// Write a request with invalid JSON params (string instead of object)
	req := protocoljsonrpc.Request{
		ID:     protocoljsonrpc.StringID("req-bad-params"),
		Method: protocoljsonrpc.MethodSessionList,
		Params: json.RawMessage(`"not an object"`),
	}
	_ = protocoljsonrpc.NewStream(nil, &input).WriteMessage(req)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := protocoljsonrpc.ServeStream(ctx, router, protocoljsonrpc.NewStream(&input, &output))
	// ServeStream may return nil or context.Canceled depending on timing
	_ = err

	// Read the error response
	decoded, err := protocoljsonrpc.NewStream(&output, nil).ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if decoded.Kind != protocoljsonrpc.KindResponse || decoded.Response == nil {
		t.Fatalf("expected error response, got %+v", decoded)
	}
	if decoded.Response.Error == nil {
		t.Fatal("response error = nil, want invalid params")
	}
	if decoded.Response.Error.Code != protocoljsonrpc.CodeInvalidParams {
		t.Fatalf("error code = %d, want %d (InvalidParams)", decoded.Response.Error.Code, protocoljsonrpc.CodeInvalidParams)
	}
	if !strings.Contains(decoded.Response.Error.Message, "invalid params") {
		t.Fatalf("error message = %q, want invalid params", decoded.Response.Error.Message)
	}
}

func TestUnknownMethodThroughServeStreamReturnsMethodNotFound(t *testing.T) {
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    fakeStateService{},
		sessions: &fakeSessionService{},
	}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	var input bytes.Buffer
	var output bytes.Buffer

	req, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-unknown"), "unknown/fake/method", nil)
	_ = protocoljsonrpc.NewStream(nil, &input).WriteMessage(req)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := protocoljsonrpc.ServeStream(ctx, router, protocoljsonrpc.NewStream(&input, &output))
	_ = err

	decoded, err := protocoljsonrpc.NewStream(&output, nil).ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if decoded.Response.Error == nil {
		t.Fatal("response error = nil, want method not found")
	}
	if decoded.Response.Error.Code != protocoljsonrpc.CodeMethodNotFound {
		t.Fatalf("error code = %d, want %d (MethodNotFound)", decoded.Response.Error.Code, protocoljsonrpc.CodeMethodNotFound)
	}
}

func TestEngineErrorThroughServeStreamReturnsInternalError(t *testing.T) {
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    fakeStateService{err: errors.New("workspace not found")},
		sessions: &fakeSessionService{},
	}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	var input bytes.Buffer
	var output bytes.Buffer

	req, _ := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-err"), protocoljsonrpc.MethodStateSnapshot, nil)
	_ = protocoljsonrpc.NewStream(nil, &input).WriteMessage(req)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := protocoljsonrpc.ServeStream(ctx, router, protocoljsonrpc.NewStream(&input, &output))
	_ = err

	decoded, err := protocoljsonrpc.NewStream(&output, nil).ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if decoded.Response.Error == nil {
		t.Fatal("response error = nil, want internal error")
	}
	if decoded.Response.Error.Code != protocoljsonrpc.CodeInternalError {
		t.Fatalf("error code = %d, want %d (InternalError)", decoded.Response.Error.Code, protocoljsonrpc.CodeInternalError)
	}
}

// --- Unsupported service returns error ---

func TestUnsupportedServiceReturnsInternalError(t *testing.T) {
	// When a service is nil (unsupported), the handler should return internal error
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    nil, // nil state service
		sessions: &fakeSessionService{},
	}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var out coreapi.StateSnapshot
	err := client.Call(context.Background(), protocoljsonrpc.MethodStateSnapshot, nil, &out)
	if err == nil {
		t.Fatal("Call() error = nil, want unsupported error")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("error = %v, want unsupported", err)
	}
}

// --- Notification wire shape: no jsonrpc version field ---

func TestEventNotificationWireShapeNoJSONRPCVersion(t *testing.T) {
	// Verify that event notifications on the wire do NOT include "jsonrpc" field
	events := &fakeEvents{ch: make(chan protocol.Envelope, 1)}
	defer close(events.ch)
	notifier := captureNotifier{ch: make(chan protocoljsonrpc.Notification, 1)}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    fakeStateService{},
		sessions: &fakeSessionService{},
		events:   events,
	}, Options{Notifier: notifier}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Trigger event subscribe
	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var sub coreapi.EventSubscription
	if err := client.Call(context.Background(), protocoljsonrpc.MethodEventSubscribe, coreapi.EventFilter{
		SessionID: "sess-wire",
	}, &sub); err != nil {
		t.Fatalf("Call(event/subscribe) error = %v", err)
	}
	if sub.ID == "" {
		t.Fatal("subscription ID is empty")
	}

	// Publish an event
	ev := protocol.NewEvent(protocol.EventTypeTextDelta, protocol.EventOptions{
		EventID:   "evt-wire",
		SessionID: "sess-wire",
		RequestID: "turn-wire",
		Payload:   protocol.TextPayloadMap(protocol.TextPayload{Text: "wire test"}),
	})
	events.ch <- ev

	// Read the notification
	select {
	case notif := <-notifier.ch:
		// Marshal the notification to check wire shape
		data, err := protocoljsonrpc.Marshal(notif)
		if err != nil {
			t.Fatalf("Marshal(notification) error = %v", err)
		}
		if strings.Contains(string(data), `"jsonrpc"`) {
			t.Fatalf("event notification wire contains jsonrpc: %s", data)
		}

		// Verify the envelope inside params also doesn't have jsonrpc
		var params map[string]any
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			t.Fatalf("Unmarshal(notification params) error = %v", err)
		}
		if _, ok := params["jsonrpc"]; ok {
			t.Fatalf("notification params should not have jsonrpc key: %+v", params)
		}

		// Verify the envelope fields are correct
		var envelope protocol.Envelope
		if err := json.Unmarshal(notif.Params, &envelope); err != nil {
			t.Fatalf("Unmarshal(envelope) error = %v", err)
		}
		if envelope.EventID != "evt-wire" || envelope.EventType != protocol.EventTypeTextDelta {
			t.Fatalf("envelope=%+v, want evt-wire text.delta", envelope)
		}
	case <-timeout():
		t.Fatal("timeout waiting for notification")
	}
}

func TestInitializedNotificationWireShapeNoJSONRPCVersion(t *testing.T) {
	// Verify initialized notification wire shape
	notifier := captureNotifier{ch: make(chan protocoljsonrpc.Notification, 1)}
	notif, err := protocoljsonrpc.NewNotification(protocoljsonrpc.NotificationInitialized, map[string]any{
		"server": "eos-core",
	})
	if err != nil {
		t.Fatalf("NewNotification() error = %v", err)
	}

	if err := notifier.Notify(context.Background(), notif); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	select {
	case got := <-notifier.ch:
		data, err := protocoljsonrpc.Marshal(got)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if strings.Contains(string(data), `"jsonrpc"`) {
			t.Fatalf("initialized notification wire contains jsonrpc: %s", data)
		}
		if got.Method != protocoljsonrpc.NotificationInitialized {
			t.Fatalf("method = %q, want %q", got.Method, protocoljsonrpc.NotificationInitialized)
		}
	case <-timeout():
		t.Fatal("timeout waiting for notification")
	}
}

func TestStateChangedNotificationWireShapeNoJSONRPCVersion(t *testing.T) {
	// Verify state/changed notification wire shape
	notifier := captureNotifier{ch: make(chan protocoljsonrpc.Notification, 1)}
	notif, err := protocoljsonrpc.NewNotification(protocoljsonrpc.NotificationStateChanged, map[string]any{
		"topic": "sessions",
	})
	if err != nil {
		t.Fatalf("NewNotification() error = %v", err)
	}

	if err := notifier.Notify(context.Background(), notif); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	select {
	case got := <-notifier.ch:
		data, err := protocoljsonrpc.Marshal(got)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if strings.Contains(string(data), `"jsonrpc"`) {
			t.Fatalf("state/changed notification wire contains jsonrpc: %s", data)
		}
		if got.Method != protocoljsonrpc.NotificationStateChanged {
			t.Fatalf("method = %q, want %q", got.Method, protocoljsonrpc.NotificationStateChanged)
		}
	case <-timeout():
		t.Fatal("timeout waiting for notification")
	}
}

// --- Error response wire shape ---

func TestErrorResponseWireShapeNoJSONRPCVersion(t *testing.T) {
	// Verify that error responses don't include jsonrpc version
	resp, _ := protocoljsonrpc.NewErrorResponse(protocoljsonrpc.StringID("req-err"),
		protocoljsonrpc.CodeMethodNotFound, "method not found",
		map[string]any{"method": "unknown/test"})
	data, err := protocoljsonrpc.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(data), `"jsonrpc"`) {
		t.Fatalf("error response wire contains jsonrpc: %s", data)
	}

	// Verify error response structure
	if resp.ID.String() != "req-err" {
		t.Fatalf("id = %q, want req-err", resp.ID.String())
	}
	if resp.Error.Code != protocoljsonrpc.CodeMethodNotFound {
		t.Fatalf("error code = %d, want %d", resp.Error.Code, protocoljsonrpc.CodeMethodNotFound)
	}
	if resp.Error.Message != "method not found" {
		t.Fatalf("error message = %q, want method not found", resp.Error.Message)
	}
	if resp.Error.Data == nil {
		t.Fatal("error data = nil, want method info")
	}
}

func TestErrorResponseWireShapeWithData(t *testing.T) {
	// Verify error response with structured data field
	data := map[string]any{"field": "name", "reason": "required"}
	resp, _ := protocoljsonrpc.NewErrorResponse(protocoljsonrpc.NumberID(42),
		protocoljsonrpc.CodeInvalidParams, "invalid params", data)

	wire, err := protocoljsonrpc.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Verify round-trip
	var parsed map[string]any
	if err := json.Unmarshal(wire, &parsed); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if parsed["id"] != float64(42) {
		t.Fatalf("parsed id = %v, want 42", parsed["id"])
	}
	errObj, ok := parsed["error"].(map[string]any)
	if !ok {
		t.Fatalf("parsed error = %v, want object", parsed["error"])
	}
	if errObj["code"] != float64(-32602) {
		t.Fatalf("error code = %v, want -32602", errObj["code"])
	}
	if errObj["data"] == nil {
		t.Fatal("error data = nil, want structured data")
	}
}

// --- Full sequence: initialize -> state/snapshot -> session/list through InProcess ---

func TestFullSequenceInitializeStateSnapshotSessionList(t *testing.T) {
	// Golden sequence test: verify a complete request sequence through InProcess
	state := fakeStateService{snapshot: coreapi.StateSnapshot{
		ForegroundWorkspace: "C:/work/golden",
	}}
	sessions := &fakeSessionService{items: []coreapi.Session{
		{ID: "sess-golden-1", WorkspaceRoot: "C:/work/golden", Metadata: map[string]any{"title": "golden session"}},
	}}

	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    state,
		sessions: sessions,
	}, Options{ServerName: "golden-server", ProtocolVersion: "v1"}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})

	// Step 1: initialize
	var initResult InitializeResult
	if err := client.Call(context.Background(), protocoljsonrpc.MethodInitialize, nil, &initResult); err != nil {
		t.Fatalf("Call(initialize) error = %v", err)
	}
	if initResult.ServerName != "golden-server" {
		t.Fatalf("ServerName = %q, want golden-server", initResult.ServerName)
	}
	if initResult.ProtocolVersion != "v1" {
		t.Fatalf("ProtocolVersion = %q, want v1", initResult.ProtocolVersion)
	}
	if !contains(initResult.Methods, protocoljsonrpc.MethodInitialize) ||
		!contains(initResult.Methods, protocoljsonrpc.MethodStateSnapshot) ||
		!contains(initResult.Methods, protocoljsonrpc.MethodSessionList) {
		t.Fatalf("methods missing required entries: %+v", initResult.Methods)
	}

	// Step 2: state/snapshot
	var snapshot coreapi.StateSnapshot
	if err := client.Call(context.Background(), protocoljsonrpc.MethodStateSnapshot, nil, &snapshot); err != nil {
		t.Fatalf("Call(state/snapshot) error = %v", err)
	}
	if snapshot.ForegroundWorkspace != "C:/work/golden" {
		t.Fatalf("ForegroundWorkspace = %q, want C:/work/golden", snapshot.ForegroundWorkspace)
	}

	// Step 3: session/list
	var sessionItems []coreapi.Session
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionList, coreapi.ListSessionsRequest{
		WorkspaceRoot: "C:/work/golden",
	}, &sessionItems); err != nil {
		t.Fatalf("Call(session/list) error = %v", err)
	}
	if len(sessionItems) != 1 || sessionItems[0].ID != "sess-golden-1" {
		t.Fatalf("sessions = %+v, want sess-golden-1", sessionItems)
	}
	if sessions.seen.WorkspaceRoot != "C:/work/golden" {
		t.Fatalf("seen workspace = %q, want C:/work/golden", sessions.seen.WorkspaceRoot)
	}
}

// --- InProcessClient error propagation ---

func TestInProcessClientPropagatesJSONRPCError(t *testing.T) {
	router := protocoljsonrpc.NewRouter()
	if err := router.Register("fail", func(context.Context, protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		return nil, &protocoljsonrpc.Error{Code: protocoljsonrpc.CodeInvalidParams, Message: "test failure"}
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var out map[string]any
	err := client.Call(context.Background(), "fail", nil, &out)
	if err == nil {
		t.Fatal("Call() error = nil, want jsonrpc error")
	}
	if !strings.Contains(err.Error(), "32602") {
		t.Fatalf("error = %v, want code 32602", err)
	}
	if !strings.Contains(err.Error(), "test failure") {
		t.Fatalf("error = %v, want test failure", err)
	}
}

// --- Decode error: malformed JSON frame ---

func TestDecodeRejectsMalformedJSON(t *testing.T) {
	_, err := protocoljsonrpc.Decode([]byte(`{not json`))
	if err == nil {
		t.Fatal("Decode() error = nil, want parse error for malformed JSON")
	}
}

func TestDecodeRejectsEmptyPayload(t *testing.T) {
	_, err := protocoljsonrpc.Decode([]byte{})
	if err == nil {
		t.Fatal("Decode() error = nil, want error for empty payload")
	}
}

func timeout() <-chan time.Time {
	return time.After(2 * time.Second)
}
