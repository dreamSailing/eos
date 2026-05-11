package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"strings"

	"github.com/dreamSailing/eos/pkg/protocol"
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

func bridgeLoopBlockedEvent(toolName, level, reason, message string, data map[string]any) Event {
	payload := cloneBridgePayload(data)
	if payload == nil {
		payload = map[string]any{}
	}
	if toolName = strings.TrimSpace(toolName); toolName != "" {
		payload["tool_name"] = toolName
	}
	if level = strings.TrimSpace(level); level != "" {
		payload["level"] = level
	}
	if reason = strings.TrimSpace(reason); reason != "" {
		payload["reason"] = reason
	}
	if message = strings.TrimSpace(message); message != "" {
		payload["message"] = message
	}
	return Event{
		Type:    string(protocol.EventTypeLoopBlocked),
		RID:     firstBridgeNonEmpty(toolName, reason),
		Content: message,
		Data:    payload,
	}
}

func bridgeTurnWrapUpEvent(toolName, reason, message string, data map[string]any) Event {
	payload := cloneBridgePayload(data)
	if payload == nil {
		payload = map[string]any{}
	}
	if toolName = strings.TrimSpace(toolName); toolName != "" {
		payload["tool_name"] = toolName
	}
	if reason = strings.TrimSpace(reason); reason != "" {
		payload["reason"] = reason
	}
	if message = strings.TrimSpace(message); message != "" {
		payload["message"] = message
	}
	return Event{
		Type:    string(protocol.EventTypeTurnWrapUp),
		RID:     firstBridgeNonEmpty(toolName, reason),
		Content: message,
		Data:    payload,
	}
}

func bridgeAgentStartedEvent(agentID, agentName, task string, data map[string]any) Event {
	task = strings.TrimSpace(task)
	payload := cloneBridgePayload(data)
	if payload == nil {
		payload = map[string]any{}
	}
	if strings.TrimSpace(agentID) != "" {
		payload["agent_id"] = strings.TrimSpace(agentID)
	}
	if strings.TrimSpace(agentName) != "" {
		payload["agent_name"] = strings.TrimSpace(agentName)
	}
	if task != "" {
		payload["task"] = task
		payload["message"] = task
	}
	return Event{
		Type:    string(protocol.EventTypeAgentStarted),
		RID:     firstBridgeNonEmpty(strings.TrimSpace(agentID), strings.TrimSpace(agentName)),
		Content: task,
		Data:    payload,
	}
}

func bridgeAgentProgressEvent(agentID, agentName, task string, data map[string]any) Event {
	task = strings.TrimSpace(task)
	payload := cloneBridgePayload(data)
	if payload == nil {
		payload = map[string]any{}
	}
	if strings.TrimSpace(agentID) != "" {
		payload["agent_id"] = strings.TrimSpace(agentID)
	}
	if strings.TrimSpace(agentName) != "" {
		payload["agent_name"] = strings.TrimSpace(agentName)
	}
	if task != "" {
		payload["task"] = task
		payload["message"] = task
	}
	return Event{
		Type:    string(protocol.EventTypeAgentProgress),
		RID:     firstBridgeNonEmpty(strings.TrimSpace(agentID), strings.TrimSpace(agentName)),
		Content: task,
		Data:    payload,
	}
}

func bridgeAgentCompletedEvent(agentID, agentName, content string, data map[string]any) Event {
	content = strings.TrimSpace(content)
	payload := cloneBridgePayload(data)
	if payload == nil {
		payload = map[string]any{}
	}
	if content != "" {
		payload["text"] = content
	}
	if strings.TrimSpace(agentID) != "" {
		payload["agent_id"] = strings.TrimSpace(agentID)
	}
	if strings.TrimSpace(agentName) != "" {
		payload["agent_name"] = strings.TrimSpace(agentName)
	}
	if content != "" {
		payload["message"] = content
	}
	return Event{
		Type:    string(protocol.EventTypeAgentDone),
		RID:     firstBridgeNonEmpty(strings.TrimSpace(agentID), strings.TrimSpace(agentName)),
		Content: content,
		Data:    payload,
	}
}

func bridgeAgentFailedEvent(agentID, agentName, errMsg string, data map[string]any) Event {
	errMsg = strings.TrimSpace(errMsg)
	payload := cloneBridgePayload(data)
	if payload == nil {
		payload = map[string]any{}
	}
	if strings.TrimSpace(agentID) != "" {
		payload["agent_id"] = strings.TrimSpace(agentID)
	}
	if strings.TrimSpace(agentName) != "" {
		payload["agent_name"] = strings.TrimSpace(agentName)
	}
	if errMsg != "" {
		payload["error"] = errMsg
		payload["message"] = errMsg
	}
	return Event{
		Type:    string(protocol.EventTypeAgentFailed),
		RID:     firstBridgeNonEmpty(strings.TrimSpace(agentID), strings.TrimSpace(agentName)),
		Content: errMsg,
		Data:    payload,
	}
}

func bridgeAgentCancelledEvent(agentID, agentName, reason string, data map[string]any) Event {
	reason = strings.TrimSpace(reason)
	payload := cloneBridgePayload(data)
	if payload == nil {
		payload = map[string]any{}
	}
	if strings.TrimSpace(agentID) != "" {
		payload["agent_id"] = strings.TrimSpace(agentID)
	}
	if strings.TrimSpace(agentName) != "" {
		payload["agent_name"] = strings.TrimSpace(agentName)
	}
	if reason != "" {
		payload["reason"] = reason
		payload["message"] = reason
	}
	return Event{
		Type:    string(protocol.EventTypeAgentCancelled),
		RID:     firstBridgeNonEmpty(strings.TrimSpace(agentID), strings.TrimSpace(agentName)),
		Content: reason,
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

func mergeBridgeEventData(base map[string]any, extras map[string]any) map[string]any {
	out := cloneBridgePayload(base)
	if out == nil {
		out = map[string]any{}
	}
	for key, value := range extras {
		if key == "type" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
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
