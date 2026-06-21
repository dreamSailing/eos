package adapter

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"strings"

	"github.com/dreamSailing/eos/pkg/protocol"
)

// bridge 边界说明: 本文件已脱离 bridge 依赖。
// 旧版本的 normalizeRuntimeEvent(bridge.Event) 在 Go legacy 引擎废弃后被删除。
// 事件归一化现在由 core_client.go 中的 runtimeEventFromEnvelope 负责。
// 本文件保留 protocol.Envelope 字段归一化与文本/工具提示归一化 helper。

type RuntimeEvent struct {
	Type    string
	RID     string
	Content string
	Data    map[string]any
}

type PromptResponse struct {
	Decision    string
	Option      string
	OptionIndex int
	Text        string
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

// ensureProtocolPayload 是给 protocol.Envelope 用的归一化入口。
func ensureProtocolPayload(payload map[string]any, content, eventType string) {
	if payload == nil {
		return
	}
	switch eventType {
	case string(protocol.EventTypeItemDelta),
		string(protocol.EventTypeTextFinal),
		string(protocol.EventTypeTextReasoning):
		ensurePayloadText(payload, content)
	case string(protocol.EventTypeApprovalReq):
		ensureApprovalPayload(payload, "", content)
	case string(protocol.EventTypeInquiryReq):
		ensureInquiryPayload(payload, "", content)
	}
}
