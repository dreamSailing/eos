package tools

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestTraceIDContext(t *testing.T) {
	ctx := context.Background()
	if got := TraceIDFromContext(ctx); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	ctx = WithTraceID(ctx, "  rid-1 ")
	if got := TraceIDFromContext(ctx); got != "rid-1" {
		t.Fatalf("expected rid-1, got %q", got)
	}
}

func TestNotifyToolCall(t *testing.T) {
	var calls int32
	OnToolCall = func(traceID string, toolName string) {
		if traceID != "rid-1" {
			t.Fatalf("unexpected trace id: %q", traceID)
		}
		if toolName != "grep" {
			t.Fatalf("unexpected tool name: %q", toolName)
		}
		atomic.AddInt32(&calls, 1)
	}
	t.Cleanup(func() { OnToolCall = nil })
	ctx := WithTraceID(context.Background(), "rid-1")
	NotifyToolCall(ctx, "Grep")
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 call, got %d", got)
	}
}

func TestNotifyToolResult(t *testing.T) {
	var gotOK bool
	OnToolResult = func(traceID string, toolName string, success bool) {
		if traceID != "rid-1" {
			t.Fatalf("unexpected trace id: %q", traceID)
		}
		if toolName != "read" {
			t.Fatalf("unexpected tool name: %q", toolName)
		}
		gotOK = success
	}
	t.Cleanup(func() { OnToolResult = nil })
	ctx := WithTraceID(context.Background(), "rid-1")
	NotifyToolResult(ctx, "READ", true)
	if !gotOK {
		t.Fatalf("expected success=true")
	}
}

