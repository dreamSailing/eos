package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestAgentToolMissingPrompt(t *testing.T) {
	mgr := NewManager()
	result := mgr.agentToolStructured(context.Background(), map[string]interface{}{})
	if result.Status != "error" {
		t.Errorf("expected error for missing prompt, got %s", result.Status)
	}
}

func TestAgentToolEmptyPrompt(t *testing.T) {
	mgr := NewManager()
	result := mgr.agentToolStructured(context.Background(), map[string]interface{}{
		"prompt": "  ",
	})
	if result.Status != "error" {
		t.Errorf("expected error for empty prompt, got %s", result.Status)
	}
}

func TestAgentToolSyncNoExecutor(t *testing.T) {
	mgr := NewManager()
	AgentToolExecutor = nil
	defer func() { AgentToolExecutor = nil }()

	result := mgr.agentToolStructured(context.Background(), map[string]interface{}{
		"prompt":        "search for auth files",
		"subagent_type": "explore",
	})
	if result.Status != "error" {
		t.Errorf("expected error when no executor, got %s", result.Status)
	}
	if !strings.Contains(result.Error, "not registered") {
		t.Errorf("error should mention 'not registered', got: %s", result.Error)
	}
}

func TestAgentToolSyncSuccess(t *testing.T) {
	mgr := NewManager()
	AgentToolExecutor = func(ctx context.Context, prompt, subagentType, description, model string) (string, error) {
		return "Found 3 auth files: auth.go, auth_test.go, middleware.go", nil
	}
	defer func() { AgentToolExecutor = nil }()

	result := mgr.agentToolStructured(context.Background(), map[string]interface{}{
		"prompt":        "search for auth files",
		"subagent_type": "explore",
		"description":   "Find auth files",
	})

	if result.Status != "success" {
		t.Errorf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Data["subagent_type"] != "explore" {
		t.Errorf("expected subagent_type=explore, got %v", result.Data["subagent_type"])
	}
}

func TestAgentToolSyncFailure(t *testing.T) {
	mgr := NewManager()
	AgentToolExecutor = func(ctx context.Context, prompt, subagentType, description, model string) (string, error) {
		return "", errors.New("model timeout")
	}
	defer func() { AgentToolExecutor = nil }()

	result := mgr.agentToolStructured(context.Background(), map[string]interface{}{
		"prompt": "test prompt",
	})

	if result.Status != "error" {
		t.Errorf("expected error, got %s", result.Status)
	}
	if !strings.Contains(result.Error, "model timeout") {
		t.Errorf("error should contain 'model timeout', got: %s", result.Error)
	}
}

func TestAgentToolBackgroundNoExecutor(t *testing.T) {
	mgr := NewManager()
	AgentToolBackgroundExecutor = nil
	defer func() { AgentToolBackgroundExecutor = nil }()

	result := mgr.agentToolStructured(context.Background(), map[string]interface{}{
		"prompt":           "background task",
		"run_in_background": true,
	})
	if result.Status != "error" {
		t.Errorf("expected error when no background executor, got %s", result.Status)
	}
}

func TestAgentToolBackgroundSuccess(t *testing.T) {
	mgr := NewManager()
	AgentToolBackgroundExecutor = func(ctx context.Context, prompt, subagentType, description, model string) (string, error) {
		return "task_abc123", nil
	}
	defer func() { AgentToolBackgroundExecutor = nil }()

	result := mgr.agentToolStructured(context.Background(), map[string]interface{}{
		"prompt":           "run tests",
		"run_in_background": true,
	})

	if result.Status != "success" {
		t.Errorf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Data["task_id"] != "task_abc123" {
		t.Errorf("expected task_id=task_abc123, got %v", result.Data["task_id"])
	}
	if result.Data["background"] != true {
		t.Error("expected background=true")
	}
}
