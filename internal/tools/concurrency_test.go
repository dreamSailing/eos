package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"testing"
)

func TestIsConcurrencySafeByDefinition(t *testing.T) {
	tests := []struct {
		tool  string
		safe  bool
	}{
		{"read", true},
		{"search", true},
		{"time_now", true},
		{"tool_search", true},
		{"edit", false},
		{"fs", false},
		{"bash", false},
		{"unknown_tool_xyz", false},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			got := IsConcurrencySafeByDefinition(tt.tool)
			if got != tt.safe {
				t.Errorf("IsConcurrencySafeByDefinition(%q) = %v, want %v", tt.tool, got, tt.safe)
			}
		})
	}
}

func TestBatchExecutorExecuteConcurrent(t *testing.T) {
	mgr := NewManager()
	be := NewBatchExecutor(mgr)

	calls := []ToolCall{
		{Tool: "time_now", Parameters: map[string]interface{}{}},
		{Tool: "time_now", Parameters: map[string]interface{}{}},
		{Tool: "time_now", Parameters: map[string]interface{}{}},
	}

	results := be.ExecuteConcurrent(context.Background(), calls)

	if len(results) != len(calls) {
		t.Fatalf("expected %d results, got %d", len(calls), len(results))
	}

	for i, r := range results {
		if r.Status != "success" {
			t.Errorf("result[%d]: expected success, got %s (error: %s)", i, r.Status, r.Error)
		}
	}
}

func TestBatchExecutorMixedSafeUnsafe(t *testing.T) {
	mgr := NewManager()
	be := NewBatchExecutor(mgr)

	calls := []ToolCall{
		{Tool: "time_now", Parameters: map[string]interface{}{}},
		{Tool: "bash", Parameters: map[string]interface{}{"command": "echo hello"}},
		{Tool: "time_now", Parameters: map[string]interface{}{}},
	}

	results := be.ExecuteConcurrent(context.Background(), calls)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
}

func TestBatchExecutorCancellation(t *testing.T) {
	mgr := NewManager()
	be := NewBatchExecutor(mgr)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := []ToolCall{
		{Tool: "time_now", Parameters: map[string]interface{}{}},
	}

	results := be.ExecuteConcurrent(ctx, calls)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Result should be canceled
	if results[0].Status != "error" {
		t.Errorf("expected error status for canceled context, got %s", results[0].Status)
	}
}

func TestManagerExecuteBatchPartitioned(t *testing.T) {
	mgr := NewManager()
	calls := []ToolCall{
		{Tool: "time_now", Parameters: map[string]interface{}{}},
		{Tool: "time_now", Parameters: map[string]interface{}{}},
	}

	results := mgr.ExecuteBatchPartitioned(context.Background(), calls)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}
