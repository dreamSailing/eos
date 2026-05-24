package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestStreamClientCallOverServeStream(t *testing.T) {
	router := NewRouter()
	if err := router.Register("echo", func(context.Context, Request) (any, *Error) {
		return map[string]string{"ok": "yes"}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	clientStream, serverStream, cleanup := newStreamPipe(t)
	defer cleanup()

	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- ServeStream(serverCtx, router, serverStream)
	}()

	client := NewStreamClient(clientStream)
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

	cancelServer()
	_ = client.Close()
	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ServeStream shutdown")
	}
}

func TestStreamClientReceivesNotifications(t *testing.T) {
	clientStream, serverStream, cleanup := newStreamPipe(t)
	defer cleanup()

	router := NewRouter()
	if err := router.Register("emit", func(context.Context, Request) (any, *Error) {
		notification, err := NewNotification(NotificationEvent, map[string]string{"event_id": "evt-1"})
		if err != nil {
			return nil, &Error{Code: CodeInternalError, Message: err.Error()}
		}
		if err := serverStream.WriteMessage(notification); err != nil {
			return nil, &Error{Code: CodeInternalError, Message: err.Error()}
		}
		return map[string]bool{"ok": true}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- ServeStream(serverCtx, router, serverStream)
	}()

	notifications := make(chan Notification, 1)
	client := NewStreamClient(clientStream, WithNotificationHandler(func(_ context.Context, notification Notification) error {
		notifications <- notification
		return nil
	}))
	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer client.Close()

	var out map[string]bool
	if err := client.Call(context.Background(), "emit", nil, &out); err != nil {
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

	cancelServer()
	_ = client.Close()
	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ServeStream shutdown")
	}
}

func TestStreamClientCloseReleasesPendingCall(t *testing.T) {
	clientStream, serverStream, cleanup := newStreamPipe(t)
	defer cleanup()

	client := NewStreamClient(clientStream)
	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	readDone := make(chan struct{})
	go func() {
		_, _ = serverStream.ReadMessage()
		close(readDone)
	}()

	callDone := make(chan error, 1)
	go func() {
		var out map[string]string
		callDone <- client.Call(context.Background(), "wait", nil, &out)
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

func TestStreamClientRequiresStart(t *testing.T) {
	clientStream, _, cleanup := newStreamPipe(t)
	defer cleanup()

	client := NewStreamClient(clientStream)
	var out map[string]string
	if err := client.Call(context.Background(), "echo", nil, &out); err == nil {
		t.Fatal("Call() error = nil, want not started error")
	}
}

func newStreamPipe(t *testing.T) (*Stream, *Stream, func()) {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	cleanup := func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	}
	return NewStream(clientConn, clientConn), NewStream(serverConn, serverConn), cleanup
}
