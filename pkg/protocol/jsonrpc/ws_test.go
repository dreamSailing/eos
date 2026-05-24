package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func newWSTestServer(t *testing.T, router *Router) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		wsConn := NewWSConn(conn)
		_ = ServeWS(r.Context(), router, wsConn)
	}))
}

func newWSTestClient(t *testing.T, url string) *WSConn {
	t.Helper()
	conn, _, err := websocket.Dial(context.Background(), url, nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	return NewWSConn(conn)
}

func TestServeWSHandlesRequests(t *testing.T) {
	router := NewRouter()
	if err := router.Register("echo", func(context.Context, Request) (any, *Error) {
		return map[string]string{"ok": "yes"}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	srv := newWSTestServer(t, router)
	defer srv.Close()

	wsConn := newWSTestClient(t, srv.URL)

	req, err := NewRequest(StringID("req-1"), "echo", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	if err := wsConn.WriteMessage(context.Background(), req); err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}

	decoded, err := wsConn.ReadMessage(context.Background())
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if decoded.Kind != KindResponse || decoded.Response == nil || decoded.Response.ID.String() != "req-1" {
		t.Fatalf("decoded=%+v, want req-1 response", decoded)
	}
	var result map[string]string
	if err := json.Unmarshal(decoded.Response.Result, &result); err != nil {
		t.Fatalf("Unmarshal(result) error = %v", err)
	}
	if result["ok"] != "yes" {
		t.Fatalf("result=%v, want ok=yes", result)
	}
}

func TestServeWSHandlesMultipleRequests(t *testing.T) {
	router := NewRouter()
	if err := router.Register("echo", func(_ context.Context, req Request) (any, *Error) {
		return map[string]any{"method": req.Method}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	srv := newWSTestServer(t, router)
	defer srv.Close()

	wsConn := newWSTestClient(t, srv.URL)
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		id := string(rune('a' + i - 1))
		req, _ := NewRequest(StringID(id), "echo", nil)
		if err := wsConn.WriteMessage(ctx, req); err != nil {
			t.Fatalf("WriteMessage(%d) error = %v", i, err)
		}

		decoded, err := wsConn.ReadMessage(ctx)
		if err != nil {
			t.Fatalf("ReadMessage(%d) error = %v", i, err)
		}
		if decoded.Response == nil || decoded.Response.ID.String() != id {
			t.Fatalf("response %d: %+v, want id=%q", i, decoded, id)
		}
	}
}

func TestWSClientCallOverServeWS(t *testing.T) {
	router := NewRouter()
	if err := router.Register("echo", func(context.Context, Request) (any, *Error) {
		return map[string]string{"ok": "yes"}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	srv := newWSTestServer(t, router)
	defer srv.Close()

	wsConn := newWSTestClient(t, srv.URL)
	client := NewWSClient(wsConn)
	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer client.Close()

	var out map[string]string
	if err := client.Call(context.Background(), "echo", nil, &out); err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if out["ok"] != "yes" {
		t.Fatalf("out=%v, want ok=yes", out)
	}
}

func TestWSClientCallWithParams(t *testing.T) {
	router := NewRouter()
	if err := router.Register("add", func(_ context.Context, req Request) (any, *Error) {
		var params struct {
			A int `json:"a"`
			B int `json:"b"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, &Error{Code: CodeInvalidParams, Message: err.Error()}
		}
		return map[string]int{"sum": params.A + params.B}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	srv := newWSTestServer(t, router)
	defer srv.Close()

	wsConn := newWSTestClient(t, srv.URL)
	client := NewWSClient(wsConn)
	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer client.Close()

	var out map[string]int
	if err := client.Call(context.Background(), "add", map[string]int{"a": 3, "b": 4}, &out); err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if out["sum"] != 7 {
		t.Fatalf("out=%v, want sum=7", out)
	}
}

func TestWSClientReceivesNotifications(t *testing.T) {
	router := NewRouter()

	var serverConn *WSConn
	if err := router.Register("emit", func(_ context.Context, req Request) (any, *Error) {
		notification, err := NewNotification(NotificationEvent, map[string]string{"event_id": "evt-1"})
		if err != nil {
			return nil, &Error{Code: CodeInternalError, Message: err.Error()}
		}
		if err := serverConn.WriteMessage(context.Background(), notification); err != nil {
			return nil, &Error{Code: CodeInternalError, Message: err.Error()}
		}
		return map[string]bool{"ok": true}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		serverConn = NewWSConn(conn)
		_ = ServeWS(r.Context(), router, serverConn)
	}))
	defer srv.Close()

	ctx := context.Background()
	clientConn := newWSTestClient(t, srv.URL)

	notifications := make(chan Notification, 1)
	client := NewWSClient(clientConn, WithWSNotificationHandler(func(_ context.Context, notification Notification) error {
		notifications <- notification
		return nil
	}))
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer client.Close()

	var out map[string]bool
	if err := client.Call(ctx, "emit", nil, &out); err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if !out["ok"] {
		t.Fatalf("out=%v, want ok=true", out)
	}

	select {
	case notification := <-notifications:
		if notification.Method != NotificationEvent {
			t.Fatalf("notification method=%q, want %q", notification.Method, NotificationEvent)
		}
		var payload map[string]string
		if err := json.Unmarshal(notification.Params, &payload); err != nil {
			t.Fatalf("Unmarshal(notification params) error = %v", err)
		}
		if payload["event_id"] != "evt-1" {
			t.Fatalf("payload=%v, want event_id=evt-1", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for notification")
	}
}

func TestWSClientCloseReleasesPendingCall(t *testing.T) {
	readDone := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		wsConn := NewWSConn(conn)
		_, _ = wsConn.ReadMessage(r.Context())
		close(readDone)
	}))
	defer srv.Close()

	ctx := context.Background()
	wsConn := newWSTestClient(t, srv.URL)

	client := NewWSClient(wsConn)
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	callDone := make(chan error, 1)
	go func() {
		var out map[string]string
		callDone <- client.Call(ctx, "wait", nil, &out)
	}()

	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server to read request")
	}

	_ = client.Close()
	select {
	case err := <-callDone:
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("Call() error = %v, want io.ErrClosedPipe", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for pending call to be released")
	}
}

func TestWSClientRequiresStart(t *testing.T) {
	srv := newWSTestServer(t, NewRouter())
	defer srv.Close()

	wsConn := newWSTestClient(t, srv.URL)
	client := NewWSClient(wsConn)
	var out map[string]string
	if err := client.Call(context.Background(), "echo", nil, &out); err == nil {
		t.Fatal("Call() error = nil, want not started error")
	}
}

func TestWSClientMethodNotFound(t *testing.T) {
	srv := newWSTestServer(t, NewRouter())
	defer srv.Close()

	wsConn := newWSTestClient(t, srv.URL)
	client := NewWSClient(wsConn)
	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer client.Close()

	var out map[string]string
	err := client.Call(context.Background(), "unknown/method", nil, &out)
	if err == nil {
		t.Fatal("Call() error = nil, want method not found")
	}
	if !strings.Contains(err.Error(), "32601") {
		t.Fatalf("error = %v, want code 32601", err)
	}
}

func TestServeWSRejectsNilRouter(t *testing.T) {
	err := ServeWS(context.Background(), nil, &WSConn{})
	if err == nil {
		t.Fatal("ServeWS() error = nil, want nil router error")
	}
	if !strings.Contains(err.Error(), "router is nil") {
		t.Fatalf("ServeWS() error = %q, want nil router", err.Error())
	}
}

func TestServeWSRejectsNilConn(t *testing.T) {
	err := ServeWS(context.Background(), NewRouter(), nil)
	if err == nil {
		t.Fatal("ServeWS() error = nil, want nil conn error")
	}
	if !strings.Contains(err.Error(), "conn is nil") {
		t.Fatalf("ServeWS() error = %q, want nil conn", err.Error())
	}
}

func TestServeWSSkipsNotificationFrames(t *testing.T) {
	router := NewRouter()
	handled := make(chan struct{}, 1)
	if err := router.Register("echo", func(context.Context, Request) (any, *Error) {
		handled <- struct{}{}
		return map[string]bool{"ok": true}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	srv := newWSTestServer(t, router)
	defer srv.Close()

	wsConn := newWSTestClient(t, srv.URL)
	ctx := context.Background()

	notif, _ := NewNotification(NotificationEvent, map[string]any{"type": "ignored"})
	if err := wsConn.WriteMessage(ctx, notif); err != nil {
		t.Fatalf("WriteMessage(notification) error = %v", err)
	}

	req, _ := NewRequest(StringID("req-1"), "echo", nil)
	if err := wsConn.WriteMessage(ctx, req); err != nil {
		t.Fatalf("WriteMessage(request) error = %v", err)
	}

	decoded, err := wsConn.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if decoded.Kind != KindResponse || decoded.Response == nil || decoded.Response.ID.String() != "req-1" {
		t.Fatalf("decoded=%+v, want req-1 response", decoded)
	}

	select {
	case <-handled:
	default:
		t.Fatal("handler was not called")
	}
}

func TestWSServerClosesOnClientDisconnect(t *testing.T) {
	router := NewRouter()
	if err := router.Register("echo", func(context.Context, Request) (any, *Error) {
		return map[string]bool{"ok": true}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	srv := newWSTestServer(t, router)
	defer srv.Close()

	wsConn := newWSTestClient(t, srv.URL)
	ctx := context.Background()

	req, _ := NewRequest(StringID("req-1"), "echo", nil)
	if err := wsConn.WriteMessage(ctx, req); err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}
	_, err := wsConn.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}

	wsConn.Close(websocket.StatusNormalClosure, "")
}

func TestWSWireShapeNoJSONRPCVersion(t *testing.T) {
	req, _ := NewRequest(StringID("r1"), "test", map[string]any{"k": "v"})
	data, _ := Marshal(req)
	if strings.Contains(string(data), `"jsonrpc"`) {
		t.Fatalf("request wire contains jsonrpc: %s", data)
	}

	notif, _ := NewNotification("event", map[string]any{"k": "v"})
	data, _ = Marshal(notif)
	if strings.Contains(string(data), `"jsonrpc"`) {
		t.Fatalf("notification wire contains jsonrpc: %s", data)
	}

	resp, _ := NewResultResponse(StringID("r1"), map[string]any{"ok": true})
	data, _ = Marshal(resp)
	if strings.Contains(string(data), `"jsonrpc"`) {
		t.Fatalf("response wire contains jsonrpc: %s", data)
	}

	errResp, _ := NewErrorResponse(StringID("r1"), CodeMethodNotFound, "not found", nil)
	data, _ = Marshal(errResp)
	if strings.Contains(string(data), `"jsonrpc"`) {
		t.Fatalf("error response wire contains jsonrpc: %s", data)
	}
}

func TestWSNotifierNilConn(t *testing.T) {
	notifier := WSNotifier{Conn: nil}
	notif, _ := NewNotification("event", nil)
	err := notifier.Notify(context.Background(), notif)
	if err == nil {
		t.Fatal("Notify() error = nil, want nil conn error")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Fatalf("Notify() error = %q, want nil conn", err.Error())
	}
}

func TestWSNotifierContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	notifier := WSNotifier{Conn: nil}
	notif, _ := NewNotification("event", nil)
	err := notifier.Notify(ctx, notif)
	if err == nil {
		t.Fatal("Notify() error = nil, want context canceled")
	}
}

func TestWSNotifierWritesNotification(t *testing.T) {
	router := NewRouter()
	srv := newWSTestServer(t, router)
	defer srv.Close()

	wsConn := newWSTestClient(t, srv.URL)

	notifier := WSNotifier{Conn: wsConn}
	notif, _ := NewNotification(NotificationEvent, map[string]any{"event_id": "evt-1"})
	if err := notifier.Notify(context.Background(), notif); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
}

func TestServeWSReturnsOnContextCancel(t *testing.T) {
	router := NewRouter()

	ctx, cancel := context.WithCancel(context.Background())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		wsConn := NewWSConn(conn)
		done := make(chan error, 1)
		go func() {
			done <- ServeWS(ctx, router, wsConn)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()

	wsConn := newWSTestClient(t, srv.URL)
	req, _ := NewRequest(StringID("req-1"), "echo", nil)
	_ = wsConn.WriteMessage(context.Background(), req)
	_, _ = wsConn.ReadMessage(context.Background())

	cancel()
}

func TestWSClientErrorSequence(t *testing.T) {
	router := NewRouter()
	if err := router.Register("echo", func(_ context.Context, req Request) (any, *Error) {
		return map[string]any{"echoed": true}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	srv := newWSTestServer(t, router)
	defer srv.Close()

	wsConn := newWSTestClient(t, srv.URL)
	client := NewWSClient(wsConn)
	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer client.Close()

	var out1 map[string]any
	if err := client.Call(context.Background(), "echo", nil, &out1); err != nil {
		t.Fatalf("Call(echo) error = %v", err)
	}
	if !out1["echoed"].(bool) {
		t.Fatalf("out1=%v, want echoed=true", out1)
	}

	var out2 map[string]any
	err := client.Call(context.Background(), "unknown/method", nil, &out2)
	if err == nil {
		t.Fatal("Call(unknown) error = nil, want method not found")
	}

	var out3 map[string]any
	if err := client.Call(context.Background(), "echo", nil, &out3); err != nil {
		t.Fatalf("Call(echo again) error = %v", err)
	}
	if !out3["echoed"].(bool) {
		t.Fatalf("out3=%v, want echoed=true", out3)
	}
}

func TestWSClientImplementsRequester(t *testing.T) {
	srv := newWSTestServer(t, NewRouter())
	defer srv.Close()

	wsConn := newWSTestClient(t, srv.URL)
	client := NewWSClient(wsConn)

	var _ Requester = client
}
