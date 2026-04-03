package bridge

import (
	"strings"

	"github.com/dreamSailing/vb-coding/pkg/protocol"
)

func bridgeTextDeltaEvent(content string) Event {
	content = strings.TrimSpace(content)
	return Event{
		Type:    string(protocol.EventTypeTextDelta),
		Content: content,
		Data:    protocol.TextPayloadMap(protocol.TextPayload{Text: content}),
	}
}

func bridgeReasoningEvent(content string) Event {
	content = strings.TrimSpace(content)
	return Event{
		Type:    string(protocol.EventTypeTextReasoning),
		Content: content,
		Data:    protocol.TextPayloadMap(protocol.TextPayload{Text: content}),
	}
}

func bridgeToolCallEvent(id, toolName string, args map[string]any) Event {
	payload := protocol.ToolCallPayload(protocol.ToolCall{
		ToolName:  strings.TrimSpace(toolName),
		Arguments: cloneBridgePayload(args),
	})
	if strings.TrimSpace(id) != "" {
		payload["id"] = strings.TrimSpace(id)
	}
	return Event{
		Type:    string(protocol.EventTypeToolCall),
		RID:     strings.TrimSpace(id),
		Content: strings.TrimSpace(toolName),
		Data:    payload,
	}
}

func bridgeToolResultEvent(id, toolName, status, display, errMsg string, data map[string]any) Event {
	payload := protocol.ToolResultPayload(protocol.ToolResult{
		ToolName: strings.TrimSpace(toolName),
		Status:   strings.TrimSpace(status),
		Display:  strings.TrimSpace(display),
		Data:     cloneBridgePayload(data),
	})
	if strings.TrimSpace(id) != "" {
		payload["id"] = strings.TrimSpace(id)
	}
	if strings.TrimSpace(errMsg) != "" {
		payload["error"] = strings.TrimSpace(errMsg)
	}
	content := strings.TrimSpace(display)
	if content == "" {
		content = strings.TrimSpace(errMsg)
	}
	return Event{
		Type:    string(protocol.EventTypeToolResult),
		RID:     strings.TrimSpace(id),
		Content: content,
		Data:    payload,
	}
}

func bridgeAgentProgressEvent(agentName, task string) Event {
	task = strings.TrimSpace(task)
	payload := map[string]any{
		"agent_name": strings.TrimSpace(agentName),
		"task":       task,
		"message":    task,
	}
	return Event{
		Type:    string(protocol.EventTypeAgentProgress),
		RID:     strings.TrimSpace(agentName),
		Content: task,
		Data:    payload,
	}
}

func bridgeAgentCompletedEvent(agentName, content string) Event {
	content = strings.TrimSpace(content)
	payload := protocol.TextPayloadMap(protocol.TextPayload{Text: content})
	if payload == nil {
		payload = map[string]any{}
	}
	if strings.TrimSpace(agentName) != "" {
		payload["agent_name"] = strings.TrimSpace(agentName)
	}
	if content != "" {
		payload["message"] = content
	}
	return Event{
		Type:    string(protocol.EventTypeAgentDone),
		RID:     strings.TrimSpace(agentName),
		Content: content,
		Data:    payload,
	}
}

func bridgePromptEvent(req PromptRequest) Event {
	payload := map[string]any{
		"kind":       string(req.Kind),
		"title":      req.Title,
		"question":   req.Question,
		"options":    append([]string(nil), req.Options...),
		"category":   req.Category,
		"summary":    req.Summary,
		"diff":       req.Diff,
		"diff_path":  req.DiffPath,
		"allow_text": req.AllowText,
		"text_hint":  req.TextHint,
	}
	if req.Kind == "inquiry" {
		request := protocol.InquiryRequest{
			InquiryID: strings.TrimSpace(req.ID),
			Question:  strings.TrimSpace(req.Question),
			Options:   append([]string(nil), req.Options...),
			AllowText: req.AllowText,
		}
		for k, v := range protocol.InquiryRequestPayload(request) {
			payload[k] = v
		}
		return Event{
			Type:    string(protocol.EventTypeInquiryReq),
			RID:     strings.TrimSpace(req.ID),
			Content: strings.TrimSpace(req.Question),
			Data:    payload,
		}
	}

	request := protocol.ApprovalRequest{
		ApprovalID: strings.TrimSpace(req.ID),
		Title:      strings.TrimSpace(req.Title),
		Message:    firstBridgeNonEmpty(strings.TrimSpace(req.Summary), strings.TrimSpace(req.Question)),
		Options:    append([]string(nil), req.Options...),
	}
	for k, v := range protocol.ApprovalRequestPayload(request) {
		payload[k] = v
	}
	return Event{
		Type:    string(protocol.EventTypeApprovalReq),
		RID:     strings.TrimSpace(req.ID),
		Content: firstBridgeNonEmpty(strings.TrimSpace(req.Question), strings.TrimSpace(req.Summary)),
		Data:    payload,
	}
}

func cloneBridgePayload(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func firstBridgeNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
