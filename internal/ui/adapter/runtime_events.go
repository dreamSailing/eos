package adapter

import (
	"strings"

	"github.com/dreamSailing/vb-coding/internal/bridge"
	"github.com/dreamSailing/vb-coding/pkg/protocol"
)

func normalizeRuntimeEvent(ev bridge.Event) bridge.Event {
	out := bridge.Event{
		Type:    strings.TrimSpace(ev.Type),
		RID:     strings.TrimSpace(ev.RID),
		Content: strings.TrimSpace(ev.Content),
		Data:    cloneDataMap(ev.Data),
	}
	if out.Data == nil {
		out.Data = map[string]any{}
	}

	switch out.Type {
	case "meta", "delta":
		out.Type = string(protocol.EventTypeTextDelta)
		ensurePayloadText(out.Data, out.Content)
	case "final":
		out.Type = string(protocol.EventTypeTextFinal)
		ensurePayloadText(out.Data, out.Content)
	case "reasoning", "phase.note":
		out.Type = string(protocol.EventTypeTextReasoning)
		ensurePayloadText(out.Data, out.Content)
	case "tool_call":
		out.Type = string(protocol.EventTypeToolCall)
		ensureToolName(out.Data, out.Content)
	case "tool_result":
		out.Type = string(protocol.EventTypeToolResult)
		ensureToolResult(out.Data, out.Content)
	case "prompt.request":
		kind := strings.ToLower(strings.TrimSpace(stringValue(out.Data, "kind")))
		if kind == "inquiry" {
			out.Type = string(protocol.EventTypeInquiryReq)
			ensureInquiryPayload(out.Data, out.RID, out.Content)
		} else {
			out.Type = string(protocol.EventTypeApprovalReq)
			ensureApprovalPayload(out.Data, out.RID, out.Content)
		}
	case "agent.task":
		out.Type = string(protocol.EventTypeAgentProgress)
		if _, ok := out.Data["task"]; !ok && out.Content != "" {
			out.Data["task"] = out.Content
		}
		if _, ok := out.Data["message"]; !ok && out.Content != "" {
			out.Data["message"] = out.Content
		}
	case "agent.final":
		out.Type = string(protocol.EventTypeAgentDone)
		ensurePayloadText(out.Data, out.Content)
	case "error":
		out.Type = string(protocol.EventTypeRequestFailed)
		if _, ok := out.Data["error"]; !ok && out.Content != "" {
			out.Data["error"] = out.Content
		}
	case string(protocol.EventTypeTextDelta),
		string(protocol.EventTypeTextFinal),
		string(protocol.EventTypeTextReasoning):
		ensurePayloadText(out.Data, out.Content)
	case string(protocol.EventTypeToolCall):
		ensureToolName(out.Data, out.Content)
	case string(protocol.EventTypeToolResult):
		ensureToolResult(out.Data, out.Content)
	case string(protocol.EventTypeApprovalReq):
		ensureApprovalPayload(out.Data, out.RID, out.Content)
	case string(protocol.EventTypeInquiryReq):
		ensureInquiryPayload(out.Data, out.RID, out.Content)
	case string(protocol.EventTypeRequestFailed):
		if _, ok := out.Data["error"]; !ok && out.Content != "" {
			out.Data["error"] = out.Content
		}
	case string(protocol.EventTypeAgentStarted),
		string(protocol.EventTypeAgentProgress),
		string(protocol.EventTypeAgentDone),
		string(protocol.EventTypeAgentFailed),
		string(protocol.EventTypeTaskStarted),
		string(protocol.EventTypeTaskUpdated),
		string(protocol.EventTypeTaskDone),
		string(protocol.EventTypeTaskFailed):
		if _, ok := out.Data["message"]; !ok && out.Content != "" {
			out.Data["message"] = out.Content
		}
	}

	return out
}

func ensurePayloadText(payload map[string]any, content string) {
	if _, ok := payload["text"]; !ok && strings.TrimSpace(content) != "" {
		payload["text"] = content
	}
}

func ensureToolName(payload map[string]any, content string) {
	if _, ok := payload["tool_name"]; !ok && strings.TrimSpace(content) != "" {
		payload["tool_name"] = content
	}
}

func ensureToolResult(payload map[string]any, content string) {
	ensureToolName(payload, stringValue(payload, "tool_name"))
	if status := strings.TrimSpace(stringValue(payload, "status")); status == "" {
		payload["status"] = "success"
	}
	if _, ok := payload["display"]; !ok && strings.TrimSpace(content) != "" {
		payload["display"] = content
	}
}

func ensureApprovalPayload(payload map[string]any, requestID, content string) {
	if _, ok := payload["approval_id"]; !ok && strings.TrimSpace(requestID) != "" {
		payload["approval_id"] = requestID
	}
	message := stringValue(payload, "message", "summary", "question")
	if message == "" {
		message = strings.TrimSpace(content)
	}
	if _, ok := payload["message"]; !ok && message != "" {
		payload["message"] = message
	}
	if _, ok := payload["options"]; !ok {
		payload["options"] = []string{"allow_once", "allow_session", "deny"}
	}
}

func ensureInquiryPayload(payload map[string]any, requestID, content string) {
	if _, ok := payload["inquiry_id"]; !ok && strings.TrimSpace(requestID) != "" {
		payload["inquiry_id"] = requestID
	}
	question := stringValue(payload, "question", "message")
	if question == "" {
		question = strings.TrimSpace(content)
	}
	if _, ok := payload["question"]; !ok && question != "" {
		payload["question"] = question
	}
}

func cloneDataMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func stringValue(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, ok := payload[key]
		if !ok {
			continue
		}
		if text, ok := raw.(string); ok {
			text = strings.TrimSpace(text)
			if text != "" {
				return text
			}
		}
	}
	return ""
}
