package ui

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"testing"

	"github.com/dreamSailing/eos/internal/bridge"
	"github.com/dreamSailing/eos/pkg/protocol"
)

func TestConvertEventApprovalRequired(t *testing.T) {
	msg := ConvertEvent(bridge.Event{
		Type: "approval.required",
		RID:  "req-1",
		Data: map[string]any{
			"approval_id": "req-1",
			"message":     "准备执行高风险步骤",
			"options":     []string{"allow_once", "allow_session", "deny"},
		},
	})

	req, ok := msg.(PromptRequestMsg)
	if !ok {
		t.Fatalf("msg type = %T, want PromptRequestMsg", msg)
	}
	if req.Kind != "permission" {
		t.Fatalf("Kind=%q, want permission", req.Kind)
	}
	if req.Question != "准备执行高风险步骤" {
		t.Fatalf("Question=%q, want prompt message", req.Question)
	}
	if len(req.Options) != 3 {
		t.Fatalf("Options=%v, want 3 choices", req.Options)
	}
}

func TestConvertEventToolResult(t *testing.T) {
	msg := ConvertEvent(bridge.Event{
		Type:    "tool.result",
		RID:     "tool-1",
		Content: "fallback output",
		Data: map[string]any{
			"status":  "success",
			"display": "命令执行完成",
		},
	})

	res, ok := msg.(ToolResultMsg)
	if !ok {
		t.Fatalf("msg type = %T, want ToolResultMsg", msg)
	}
	if res.ID != "tool-1" {
		t.Fatalf("ID=%q, want tool-1", res.ID)
	}
	if res.Status != "success" {
		t.Fatalf("Status=%q, want success", res.Status)
	}
	if res.Output != "命令执行完成" {
		t.Fatalf("Output=%q, want display text", res.Output)
	}
}

func TestConvertEventTextFinal(t *testing.T) {
	msg := ConvertEvent(bridge.Event{
		Type: "text.final",
		RID:  "req-2",
		Data: map[string]any{
			"text": "最终回复",
		},
	})

	resp, ok := msg.(AIResponseMsg)
	if !ok {
		t.Fatalf("msg type = %T, want AIResponseMsg", msg)
	}
	if resp.Type != "final" {
		t.Fatalf("Type=%q, want final", resp.Type)
	}
	if resp.Content != "最终回复" {
		t.Fatalf("Content=%q, want 最终回复", resp.Content)
	}
}

func TestConvertEventRequestCompletedUsesInvokeDone(t *testing.T) {
	msg := ConvertEvent(bridge.Event{
		Type: string(protocol.EventTypeRequestDone),
		RID:  "req-3",
		Data: map[string]any{
			"message": "request completed",
		},
	})

	done, ok := msg.(InvokeDoneMsg)
	if !ok {
		t.Fatalf("msg type = %T, want InvokeDoneMsg", msg)
	}
	if done.Content != "" {
		t.Fatalf("Content=%q, want empty so UI can finalize from accumulated stream", done.Content)
	}
}

func TestConvertEventAgentStartedIncludesSourceRoute(t *testing.T) {
	msg := ConvertEvent(bridge.Event{
		Type: string(protocol.EventTypeAgentStarted),
		RID:  "subagent_verification_3",
		Data: map[string]any{
			"agent_id":          "subagent_verification_3",
			"agent_name":        "verification",
			"source_agent_name": "assistant",
			"task":              "验证题目输出",
		},
	})

	task, ok := msg.(AgentTaskMsg)
	if !ok {
		t.Fatalf("msg type = %T, want AgentTaskMsg", msg)
	}
	if task.Event != "dispatch" {
		t.Fatalf("Event=%q, want dispatch", task.Event)
	}
	if task.SourceAgentName != "assistant" {
		t.Fatalf("SourceAgentName=%q, want assistant", task.SourceAgentName)
	}
	if task.AgentID != "subagent_verification_3" {
		t.Fatalf("AgentID=%q, want subagent_verification_3", task.AgentID)
	}
}

func TestConvertEventAgentFailedMapsToAgentFinal(t *testing.T) {
	msg := ConvertEvent(bridge.Event{
		Type: string(protocol.EventTypeAgentFailed),
		RID:  "subagent_verification_3",
		Data: map[string]any{
			"agent_id":          "subagent_verification_3",
			"agent_name":        "verification",
			"source_agent_name": "assistant",
			"error":             "校验失败",
		},
	})

	final, ok := msg.(AgentFinalMsg)
	if !ok {
		t.Fatalf("msg type = %T, want AgentFinalMsg", msg)
	}
	if final.Event != "failed" {
		t.Fatalf("Event=%q, want failed", final.Event)
	}
	if final.Content != "校验失败" {
		t.Fatalf("Content=%q, want 校验失败", final.Content)
	}
}
