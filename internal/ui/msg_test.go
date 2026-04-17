package ui

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
