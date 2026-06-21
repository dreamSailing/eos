package ui

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"testing"

	uiadapter "github.com/dreamSailing/eos/internal/ui/adapter"
	"github.com/dreamSailing/eos/pkg/protocol"
)

func TestConvertEventApprovalRequired(t *testing.T) {
	msg := ConvertEvent(uiadapter.RuntimeEvent{
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
	msg := ConvertEvent(uiadapter.RuntimeEvent{
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

func TestConvertEventTurnToolCallDoneCarriesArguments(t *testing.T) {
	// turn.tool_call_done 是唯一携带完整 arguments 的事件，必须产出 ToolCallMsg
	// 且把 arguments 解析为参数，而不是丢弃。
	msg := ConvertEvent(uiadapter.RuntimeEvent{
		Type: "tool.call",
		RID:  "turn-1",
		Data: map[string]any{
			"original_event_type": string(protocol.EventTypeTurnItemCompleted),
			"id":                  "tc_pwd",
			"name":                "read_file",
			"arguments":           `{"file_path":"/tmp/a.txt","offset":10,"limit":5}`,
		},
	})
	res, ok := msg.(ToolCallMsg)
	if !ok {
		t.Fatalf("msg type = %T, want ToolCallMsg (done must carry arguments)", msg)
	}
	if res.ID != "tc_pwd" {
		t.Fatalf("ID=%q, want tc_pwd", res.ID)
	}
	if res.Name != "read_file" {
		t.Fatalf("Name=%q, want read_file", res.Name)
	}
	if p, _ := res.Params["file_path"].(string); p != "/tmp/a.txt" {
		t.Fatalf("Params[file_path]=%v, want /tmp/a.txt", res.Params["file_path"])
	}
	if _, ok := res.Params["offset"]; !ok {
		t.Fatalf("Params missing offset, got %v", res.Params)
	}
}

func TestConvertEventToolCallStartDoesNotLeakEnvelope(t *testing.T) {
	// turn.tool_call_start 只有 id/name，没有 arguments。
	// 必须返回 nil 参数，绝不能把信封字段当成参数返回（这正是导致
	// 卡片渲染出 event_id/session_id/turn_id 的根因）。
	msg := ConvertEvent(uiadapter.RuntimeEvent{
		Type: "tool.call",
		RID:  "turn-1",
		Data: map[string]any{
			"original_event_type": string(protocol.EventTypeTurnItemStarted),
			"event_id":            "evt_xxx",
			"id":                  "tc_1",
			"name":                "list_directory",
			"session_id":          "sess_21",
			"turn_id":             "tui_turn_xxx",
		},
	})
	res, ok := msg.(ToolCallMsg)
	if !ok {
		t.Fatalf("msg type = %T, want ToolCallMsg", msg)
	}
	if len(res.Params) != 0 {
		t.Fatalf("Params=%v, want empty (start has no arguments)", res.Params)
	}
}

func TestConvertEventToolCallDoesNotFallBackToEnvelope(t *testing.T) {
	// 兼容旧测试：即使没有任何可识别的参数键，也不应把整个 Data
	// （信封元数据）当参数返回。
	msg := ConvertEvent(uiadapter.RuntimeEvent{
		Type: "tool.call",
		RID:  "turn-1",
		Data: map[string]any{
			"id":         "tc_3",
			"name":       "grep",
			"event_id":   "evt_yyy",
			"session_id": "sess_9",
			"turn_id":    "tui_turn_zzz",
		},
	})
	res, ok := msg.(ToolCallMsg)
	if !ok {
		t.Fatalf("msg type = %T, want ToolCallMsg", msg)
	}
	if len(res.Params) != 0 {
		t.Fatalf("Params=%v, want empty when no real arguments present", res.Params)
	}
}

func TestConvertEventToolCallAcceptsInputAlias(t *testing.T) {
	// 部分适配器用 input/parameters 而非 arguments 承载参数。
	msg := ConvertEvent(uiadapter.RuntimeEvent{
		Type: "tool.call",
		RID:  "turn-1",
		Data: map[string]any{
			"id":    "tc_4",
			"name":  "read_file",
			"input": `{"file_path":"/tmp/b.txt"}`,
		},
	})
	res, ok := msg.(ToolCallMsg)
	if !ok {
		t.Fatalf("msg type = %T, want ToolCallMsg", msg)
	}
	if p, _ := res.Params["file_path"].(string); p != "/tmp/b.txt" {
		t.Fatalf("Params[file_path]=%v, want /tmp/b.txt", res.Params["file_path"])
	}
}

func TestConvertEventTurnToolObservationUsesRequestID(t *testing.T) {
	msg := ConvertEvent(uiadapter.RuntimeEvent{
		Type: "tool.result",
		RID:  "turn-1",
		Data: map[string]any{
			"original_event_type": string(protocol.EventTypeTurnItemCompleted),
			"request_id":          "tc_pwd",
			"tool":                "shell_pwd",
			"status":              "success",
		},
	})

	res, ok := msg.(ToolResultMsg)
	if !ok {
		t.Fatalf("msg type = %T, want ToolResultMsg", msg)
	}
	if res.ID != "tc_pwd" {
		t.Fatalf("ID=%q, want request_id tc_pwd", res.ID)
	}
	if res.Status != "success" {
		t.Fatalf("Status=%q, want success", res.Status)
	}
}

func TestConvertEventToolCallWithStringArgumentsJSON(t *testing.T) {
	msg := ConvertEvent(uiadapter.RuntimeEvent{
		Type: "tool.call",
		RID:  "tc_pwd",
		Data: map[string]any{
			"id":        "tc_pwd",
			"name":      "shell_pwd",
			"arguments": "{}",
		},
	})

	res, ok := msg.(ToolCallMsg)
	if !ok {
		t.Fatalf("msg type = %T, want ToolCallMsg", msg)
	}
	if res.ID != "tc_pwd" {
		t.Fatalf("ID=%q, want tc_pwd", res.ID)
	}
	if res.Name != "shell_pwd" {
		t.Fatalf("Name=%q, want shell_pwd", res.Name)
	}
}

func TestConvertEventTextFinal(t *testing.T) {
	msg := ConvertEvent(uiadapter.RuntimeEvent{
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
	msg := ConvertEvent(uiadapter.RuntimeEvent{
		Type: string(protocol.EventTypeRequestDone),
		RID:  "req-3",
		Data: map[string]any{
			"text":    "completed turn text",
			"message": "request completed",
		},
	})

	done, ok := msg.(InvokeDoneMsg)
	if !ok {
		t.Fatalf("msg type = %T, want InvokeDoneMsg", msg)
	}
	if done.Content != "completed turn text" {
		t.Fatalf("Content=%q, want completed turn text", done.Content)
	}
}

func TestConvertEventRequestCompletedEmptyTextFallsBackToStream(t *testing.T) {
	// turn.completed 在 text 为空时，Go 侧依赖 aiLive 缓冲区回退。
	// 此测试验证即使 "text" 键缺失或为空，InvokeDoneMsg 仍正常返回。
	msg := ConvertEvent(uiadapter.RuntimeEvent{
		Type: string(protocol.EventTypeRequestDone),
		RID:  "req-4",
		Data: map[string]any{
			"message": "request completed without text",
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
	msg := ConvertEvent(uiadapter.RuntimeEvent{
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
	msg := ConvertEvent(uiadapter.RuntimeEvent{
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
