package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"
)

// --- Malformed frame tests ---

func TestReadFrameRejectsMissingContentLength(t *testing.T) {
	// Header ends without Content-Length
	stream := NewStream(strings.NewReader("\r\nhello"), nil)
	if _, err := stream.ReadFrame(); err == nil {
		t.Fatal("ReadFrame() error = nil, want error for missing Content-Length")
	}
}

func TestReadFrameRejectsInvalidHeaderFormat(t *testing.T) {
	// Header line without colon separator
	stream := NewStream(strings.NewReader("BadHeaderLine\r\n\r\n"), nil)
	if _, err := stream.ReadFrame(); err == nil {
		t.Fatal("ReadFrame() error = nil, want error for invalid header format")
	}
}

func TestReadFrameRejectsNonNumericContentLength(t *testing.T) {
	stream := NewStream(strings.NewReader("Content-Length: abc\r\n\r\n"), nil)
	if _, err := stream.ReadFrame(); err == nil {
		t.Fatal("ReadFrame() error = nil, want error for non-numeric content length")
	}
}

func TestReadFrameRejectsZeroContentLength(t *testing.T) {
	stream := NewStream(strings.NewReader("Content-Length: 0\r\n\r\n"), nil)
	if _, err := stream.ReadFrame(); err == nil {
		t.Fatal("ReadFrame() error = nil, want error for zero content length")
	}
}

func TestReadFrameRejectsNegativeContentLength(t *testing.T) {
	stream := NewStream(strings.NewReader("Content-Length: -1\r\n\r\n"), nil)
	if _, err := stream.ReadFrame(); err == nil {
		t.Fatal("ReadFrame() error = nil, want error for negative content length")
	}
}

func TestReadFrameRejectsTruncatedPayload(t *testing.T) {
	// Claims 100 bytes but only provides 5
	stream := NewStream(strings.NewReader("Content-Length: 100\r\n\r\nshort"), nil)
	if _, err := stream.ReadFrame(); err == nil {
		t.Fatal("ReadFrame() error = nil, want error for truncated payload")
	}
}

func TestWriteFrameRejectsEmptyPayload(t *testing.T) {
	var buf bytes.Buffer
	stream := NewStream(nil, &buf)
	if err := stream.WriteFrame([]byte{}); err == nil {
		t.Fatal("WriteFrame() error = nil, want error for empty payload")
	}
}

func TestWriteFrameRejectsNilWriter(t *testing.T) {
	stream := NewStream(nil, nil)
	if err := stream.WriteFrame([]byte(`{}`)); err == nil {
		t.Fatal("WriteFrame() error = nil, want error for nil writer")
	}
}

func TestReadFrameRejectsNilReader(t *testing.T) {
	stream := NewStream(nil, nil)
	if _, err := stream.ReadFrame(); err == nil {
		t.Fatal("ReadFrame() error = nil, want error for nil reader")
	}
}

// --- Error code golden tests ---

func TestRouterHandleReturnsInvalidRequestForEmptyMethod(t *testing.T) {
	router := NewRouter()
	req, err := NewRequest(StringID("req-1"), "", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	// Force empty method (bypass NewRequest validation by constructing directly)
	req.Method = ""
	resp := router.Handle(context.Background(), req)
	if resp.Error == nil {
		t.Fatal("Handle() error = nil, want InvalidRequest")
	}
	if resp.Error.Code != CodeInvalidRequest {
		t.Fatalf("error code = %d, want %d (InvalidRequest)", resp.Error.Code, CodeInvalidRequest)
	}
}

func TestRouterHandleReturnsMethodNotFound(t *testing.T) {
	router := NewRouter()
	req, err := NewRequest(StringID("req-1"), "unknown/method", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	resp := router.Handle(context.Background(), req)
	if resp.Error == nil {
		t.Fatal("Handle() error = nil, want MethodNotFound")
	}
	if resp.Error.Code != CodeMethodNotFound {
		t.Fatalf("error code = %d, want %d (MethodNotFound)", resp.Error.Code, CodeMethodNotFound)
	}
	if resp.Error.Data == nil {
		t.Fatal("error data = nil, want method name in data")
	}
	var data map[string]any
	if err := json.Unmarshal(resp.Error.Data, &data); err != nil {
		t.Fatalf("Unmarshal(error data) error = %v", err)
	}
	if data["method"] != "unknown/method" {
		t.Fatalf("data.method = %v, want unknown/method", data["method"])
	}
}

func TestRouterHandleReturnsInvalidParams(t *testing.T) {
	router := NewRouter()
	if err := router.Register("fail", func(context.Context, Request) (any, *Error) {
		return nil, &Error{Code: CodeInvalidParams, Message: "missing required field", Data: json.RawMessage(`{"field":"name"}`)}
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	req, err := NewRequest(StringID("req-1"), "fail", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	resp := router.Handle(context.Background(), req)
	if resp.Error == nil {
		t.Fatal("Handle() error = nil, want InvalidParams")
	}
	if resp.Error.Code != CodeInvalidParams {
		t.Fatalf("error code = %d, want %d (InvalidParams)", resp.Error.Code, CodeInvalidParams)
	}
	if resp.Error.Message != "missing required field" {
		t.Fatalf("error message = %q, want missing required field", resp.Error.Message)
	}
}

func TestServeStreamErrorSequence(t *testing.T) {
	// Test that ServeStream properly handles a sequence of:
	// valid request → response, unknown method → error, notification → skip, valid request → response
	router := NewRouter()
	if err := router.Register("echo", func(_ context.Context, req Request) (any, *Error) {
		return map[string]any{"echoed": true}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	var input bytes.Buffer
	var output bytes.Buffer

	// 1. Valid request
	req1, _ := NewRequest(StringID("req-1"), "echo", nil)
	_ = NewStream(nil, &input).WriteMessage(req1)

	// 2. Unknown method → should produce error response
	req2, _ := NewRequest(StringID("req-2"), "unknown/method", nil)
	_ = NewStream(nil, &input).WriteMessage(req2)

	// 3. Notification (no ID) → should be skipped by ServeStream
	notif, _ := NewNotification("event", map[string]any{"type": "skip"})
	_ = NewStream(nil, &input).WriteMessage(notif)

	// 4. Another valid request
	req3, _ := NewRequest(StringID("req-3"), "echo", nil)
	_ = NewStream(nil, &input).WriteMessage(req3)

	// Run ServeStream with a timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	outputStream := NewStream(&input, &output)
	err := ServeStream(ctx, router, outputStream)
	// ServeStream will return context.Canceled or nil after processing all frames
	_ = err

	readStream := NewStream(&output, nil)

	// Read response 1 (success)
	decoded1, err := readStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(1) error = %v", err)
	}
	if decoded1.Kind != KindResponse || decoded1.Response == nil {
		t.Fatalf("response 1: %+v, want success response", decoded1)
	}
	if decoded1.Response.ID.String() != "req-1" {
		t.Fatalf("response 1 id = %q, want req-1", decoded1.Response.ID.String())
	}
	if decoded1.Response.Error != nil {
		t.Fatalf("response 1 has error: %+v", decoded1.Response.Error)
	}

	// Read response 2 (method not found error)
	decoded2, err := readStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(2) error = %v", err)
	}
	if decoded2.Kind != KindResponse || decoded2.Response == nil {
		t.Fatalf("response 2: %+v, want error response", decoded2)
	}
	if decoded2.Response.ID.String() != "req-2" {
		t.Fatalf("response 2 id = %q, want req-2", decoded2.Response.ID.String())
	}
	if decoded2.Response.Error == nil {
		t.Fatal("response 2 error = nil, want MethodNotFound")
	}
	if decoded2.Response.Error.Code != CodeMethodNotFound {
		t.Fatalf("response 2 error code = %d, want %d", decoded2.Response.Error.Code, CodeMethodNotFound)
	}

	// Notification should be skipped, no output for it

	// Read response 3 (success)
	decoded3, err := readStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(3) error = %v", err)
	}
	if decoded3.Kind != KindResponse || decoded3.Response == nil {
		t.Fatalf("response 3: %+v, want success response", decoded3)
	}
	if decoded3.Response.ID.String() != "req-3" {
		t.Fatalf("response 3 id = %q, want req-3", decoded3.Response.ID.String())
	}
}

// --- Golden sequence: request → response → notification → request → response ---

func TestStreamGoldenRequestResponseNotificationSequence(t *testing.T) {
	// Golden test: verify the exact wire sequence of:
	// 1. Client sends request
	// 2. Server sends response
	// 3. Server sends notification (unsolicited)
	// 4. Client sends another request
	// 5. Server sends response
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	clientStream := NewStream(clientConn, clientConn)
	serverStream := NewStream(serverConn, serverConn)

	// Setup server with echo handler + notification capability
	router := NewRouter()
	if err := router.Register("echo", func(_ context.Context, req Request) (any, *Error) {
		// Server sends a notification during request handling
		notif, _ := NewNotification(NotificationEvent, map[string]any{"triggered_by": req.Method})
		_ = serverStream.WriteMessage(notif)
		return map[string]any{"echoed": true}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- ServeStream(serverCtx, router, serverStream)
	}()

	// 1. Client sends first request
	req1, _ := NewRequest(StringID("first"), "echo", map[string]any{"data": "hello"})
	if err := clientStream.WriteMessage(req1); err != nil {
		t.Fatalf("WriteMessage(req1) error = %v", err)
	}

	// 2. Read notification (server sends it first during handler)
	decodedNotif, err := clientStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(notification) error = %v", err)
	}
	if decodedNotif.Kind != KindNotification || decodedNotif.Notification == nil {
		t.Fatalf("expected notification, got %+v", decodedNotif)
	}
	if decodedNotif.Notification.Method != NotificationEvent {
		t.Fatalf("notification method = %q, want %q", decodedNotif.Notification.Method, NotificationEvent)
	}
	var notifParams map[string]any
	if err := json.Unmarshal(decodedNotif.Notification.Params, &notifParams); err != nil {
		t.Fatalf("Unmarshal(notification params) error = %v", err)
	}
	if notifParams["triggered_by"] != "echo" {
		t.Fatalf("notification triggered_by = %v, want echo", notifParams["triggered_by"])
	}

	// 3. Read response
	decodedResp1, err := clientStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(resp1) error = %v", err)
	}
	if decodedResp1.Kind != KindResponse || decodedResp1.Response == nil {
		t.Fatalf("expected response, got %+v", decodedResp1)
	}
	if decodedResp1.Response.ID.String() != "first" {
		t.Fatalf("response id = %q, want first", decodedResp1.Response.ID.String())
	}

	// 4. Client sends second request
	req2, _ := NewRequest(StringID("second"), "echo", nil)
	if err := clientStream.WriteMessage(req2); err != nil {
		t.Fatalf("WriteMessage(req2) error = %v", err)
	}

	// 5. Read second notification
	decodedNotif2, err := clientStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(notification2) error = %v", err)
	}
	if decodedNotif2.Kind != KindNotification {
		t.Fatalf("expected notification2, got %+v", decodedNotif2)
	}

	// 6. Read second response
	decodedResp2, err := clientStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(resp2) error = %v", err)
	}
	if decodedResp2.Response.ID.String() != "second" {
		t.Fatalf("response2 id = %q, want second", decodedResp2.Response.ID.String())
	}

	cancelServer()
	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server shutdown")
	}
}

// --- Wire shape: no jsonrpc version field ---

func TestWireShapeNoJSONRPCVersionInMessages(t *testing.T) {
	// Verify that all message types on the wire do NOT include "jsonrpc" field
	// This is EOS-specific: we don't use JSON-RPC 2.0 version declaration

	// Request
	req, _ := NewRequest(StringID("r1"), "test", map[string]any{"k": "v"})
	data, _ := Marshal(req)
	if strings.Contains(string(data), `"jsonrpc"`) {
		t.Fatalf("request wire contains jsonrpc: %s", data)
	}

	// Notification
	notif, _ := NewNotification("event", map[string]any{"k": "v"})
	data, _ = Marshal(notif)
	if strings.Contains(string(data), `"jsonrpc"`) {
		t.Fatalf("notification wire contains jsonrpc: %s", data)
	}

	// Response
	resp, _ := NewResultResponse(StringID("r1"), map[string]any{"ok": true})
	data, _ = Marshal(resp)
	if strings.Contains(string(data), `"jsonrpc"`) {
		t.Fatalf("response wire contains jsonrpc: %s", data)
	}

	// Error response
	errResp, _ := NewErrorResponse(StringID("r1"), CodeMethodNotFound, "not found", nil)
	data, _ = Marshal(errResp)
	if strings.Contains(string(data), `"jsonrpc"`) {
		t.Fatalf("error response wire contains jsonrpc: %s", data)
	}
}

// --- Stream Close behavior ---

func TestStreamCloseIsIdempotent(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	stream := NewStream(clientConn, clientConn)

	if err := stream.Close(); err != nil {
		t.Fatalf("Close() first error = %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() second error = %v, want nil (idempotent)", err)
	}
	_ = serverConn.Close()
}

func TestServeStreamRejectsNilRouter(t *testing.T) {
	var buf bytes.Buffer
	err := ServeStream(context.Background(), nil, NewStream(&buf, &buf))
	if err == nil {
		t.Fatal("ServeStream() error = nil, want nil router error")
	}
	if !strings.Contains(err.Error(), "router is nil") {
		t.Fatalf("ServeStream() error = %q, want nil router", err.Error())
	}
}

func TestServeStreamRejectsNilStream(t *testing.T) {
	err := ServeStream(context.Background(), NewRouter(), nil)
	if err == nil {
		t.Fatal("ServeStream() error = nil, want nil stream error")
	}
	if !strings.Contains(err.Error(), "stream is nil") {
		t.Fatalf("ServeStream() error = %q, want nil stream", err.Error())
	}
}

// --- Notification through ServeStream skips ---

func TestServeStreamSkipsNotificationFrames(t *testing.T) {
	router := NewRouter()
	handled := make(chan struct{}, 1)
	if err := router.Register("echo", func(context.Context, Request) (any, *Error) {
		handled <- struct{}{}
		return map[string]bool{"ok": true}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	var input bytes.Buffer
	var output bytes.Buffer

	// Write a notification first (should be skipped)
	notif, _ := NewNotification(NotificationEvent, map[string]any{"type": "ignored"})
	_ = NewStream(nil, &input).WriteMessage(notif)

	// Then write a valid request
	req, _ := NewRequest(StringID("req-1"), "echo", nil)
	_ = NewStream(nil, &input).WriteMessage(req)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := ServeStream(ctx, router, NewStream(&input, &output))
	_ = err

	// Should only have one response (for the request, not the notification)
	readStream := NewStream(&output, nil)
	decoded, err := readStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if decoded.Kind != KindResponse {
		t.Fatalf("decoded=%+v, want only response (notification skipped)", decoded)
	}

	// Should be no more output
	select {
	case <-handled:
	default:
		t.Fatal("handler was not called")
	}
}

func TestServeStreamSkipsResponseFrames(t *testing.T) {
	// If a response frame comes in (from a client that misunderstands the protocol),
	// ServeStream should skip it, not try to route it
	router := NewRouter()
	if err := router.Register("echo", func(context.Context, Request) (any, *Error) {
		return map[string]bool{"ok": true}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	var input bytes.Buffer
	var output bytes.Buffer

	// Write a response frame (as if from a peer)
	resp, _ := NewResultResponse(StringID("some-id"), map[string]bool{"ok": true})
	_ = NewStream(nil, &input).WriteMessage(resp)

	// Write a valid request
	req, _ := NewRequest(StringID("req-1"), "echo", nil)
	_ = NewStream(nil, &input).WriteMessage(req)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := ServeStream(ctx, router, NewStream(&input, &output))
	_ = err

	// Should only have one response
	readStream := NewStream(&output, nil)
	decoded, err := readStream.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if decoded.Response.ID.String() != "req-1" {
		t.Fatalf("response id = %q, want req-1 (response frame should be skipped)", decoded.Response.ID.String())
	}
}

// --- Error code constants verification ---

func TestErrorCodesAreStandardJSONRPC(t *testing.T) {
	// Verify error codes match JSON-RPC 2.0 standard ranges
	// -32700: Parse error
	// -32600: Invalid Request
	// -32601: Method not found
	// -32602: Invalid params
	// -32603: Internal error
	// -32000 to -32099: Server error (reserved for implementation-defined)
	tests := []struct {
		name string
		code int
		want int
	}{
		{"ParseError", CodeParseError, -32700},
		{"InvalidRequest", CodeInvalidRequest, -32600},
		{"MethodNotFound", CodeMethodNotFound, -32601},
		{"InvalidParams", CodeInvalidParams, -32602},
		{"InternalError", CodeInternalError, -32603},
		{"Backpressure", CodeBackpressure, -32001},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.want {
				t.Errorf("code = %d, want %d", tt.code, tt.want)
			}
		})
	}
}

// --- Sequential request handling with slow responses ---

func TestServeStreamSequentialWithSlowResponses(t *testing.T) {
	router := NewRouter()
	callCount := 0
	if err := router.Register("slow", func(ctx context.Context, req Request) (any, *Error) {
		callCount++
		return map[string]any{"n": callCount}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	clientStream := NewStream(clientConn, clientConn)
	serverStream := NewStream(serverConn, serverConn)

	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- ServeStream(serverCtx, router, serverStream)
	}()

	// Send 3 requests sequentially, waiting for each response
	for i := 1; i <= 3; i++ {
		req, _ := NewRequest(StringID(string(rune('a'+i-1))), "slow", nil)
		if err := clientStream.WriteMessage(req); err != nil {
			t.Fatalf("WriteMessage(%d) error = %v", i, err)
		}

		decoded, err := clientStream.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage(%d) error = %v", i, err)
		}
		if decoded.Response == nil {
			t.Fatalf("response %d is nil", i)
		}
		expectedID := string(rune('a' + i - 1))
		if decoded.Response.ID.String() != expectedID {
			t.Fatalf("response %d id = %q, want %q", i, decoded.Response.ID.String(), expectedID)
		}
	}

	if callCount != 3 {
		t.Fatalf("callCount = %d, want 3", callCount)
	}
}

// --- Stream notifier edge cases ---

func TestStreamNotifierNilStream(t *testing.T) {
	notifier := StreamNotifier{Stream: nil}
	notif, _ := NewNotification("event", nil)
	err := notifier.Notify(context.Background(), notif)
	if err == nil {
		t.Fatal("Notify() error = nil, want nil stream error")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Fatalf("Notify() error = %q, want nil stream", err.Error())
	}
}

func TestStreamNotifierContextCanceled(t *testing.T) {
	var buf bytes.Buffer
	stream := NewStream(nil, &buf)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	notifier := StreamNotifier{Stream: stream}
	notif, _ := NewNotification("event", nil)
	err := notifier.Notify(ctx, notif)
	if err == nil {
		t.Fatal("Notify() error = nil, want context canceled")
	}
}
