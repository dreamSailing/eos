package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestStreamFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	stream := NewStream(strings.NewReader(""), &buf)
	req, err := NewRequest(StringID("req-1"), MethodStateSnapshot, map[string]any{"workspace_root": "C:/work"})
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	if err := stream.WriteMessage(req); err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}
	wire := buf.String()
	if !strings.HasPrefix(wire, "Content-Length: ") {
		t.Fatalf("wire=%q, want Content-Length header", wire)
	}
	if strings.Contains(wire, `"jsonrpc"`) {
		t.Fatalf("wire should not include jsonrpc version: %s", wire)
	}

	read := NewStream(strings.NewReader(wire), nil)
	decoded, err := read.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if decoded.Kind != KindRequest || decoded.Request == nil || decoded.Request.Method != MethodStateSnapshot {
		t.Fatalf("decoded=%+v, want state/snapshot request", decoded)
	}
	if decoded.Request.ID.String() != "req-1" {
		t.Fatalf("id=%q, want req-1", decoded.Request.ID.String())
	}
}

func TestServeStreamHandlesRequests(t *testing.T) {
	router := NewRouter()
	if err := router.Register("echo", func(context.Context, Request) (any, *Error) {
		return map[string]string{"ok": "yes"}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	var input bytes.Buffer
	req, err := NewRequest(StringID("req-1"), "echo", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	if err := NewStream(strings.NewReader(""), &input).WriteMessage(req); err != nil {
		t.Fatalf("WriteMessage(input) error = %v", err)
	}
	var output bytes.Buffer
	if err := ServeStream(context.Background(), router, NewStream(&input, &output)); err != nil {
		t.Fatalf("ServeStream() error = %v", err)
	}

	decoded, err := NewStream(&output, nil).ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(output) error = %v", err)
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

func TestStreamNotifierWritesNotification(t *testing.T) {
	var output bytes.Buffer
	stream := NewStream(nil, &output)
	notification, err := NewNotification(NotificationEvent, map[string]any{"event_id": "evt-1"})
	if err != nil {
		t.Fatalf("NewNotification() error = %v", err)
	}
	if err := (StreamNotifier{Stream: stream}).Notify(context.Background(), notification); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	decoded, err := NewStream(&output, nil).ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if decoded.Kind != KindNotification || decoded.Notification == nil || decoded.Notification.Method != NotificationEvent {
		t.Fatalf("decoded=%+v, want event notification", decoded)
	}
}

func TestServeStreamReturnsOnContextCancel(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- ServeStream(ctx, NewRouter(), NewStream(reader, &bytes.Buffer{}))
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ServeStream() error = %v, want context canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ServeStream cancellation")
	}
}

func TestStreamRejectsOversizedFrame(t *testing.T) {
	stream := NewStream(strings.NewReader("Content-Length: 5\r\n\r\nhello"), nil)
	stream.MaxMessageBytes = 4
	if _, err := stream.ReadFrame(); err == nil {
		t.Fatal("ReadFrame() error = nil, want oversized frame error")
	}
}
