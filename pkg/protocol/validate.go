package protocol

// Copyright (c) 2026 EOSAIOS
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"fmt"
	"strings"
)

func ValidateEnvelope(ev Envelope) error {
	if strings.TrimSpace(string(ev.Version)) == "" {
		return fmt.Errorf("version required")
	}
	if ev.Version != VersionV1 {
		return fmt.Errorf("unsupported version: %s", ev.Version)
	}
	if strings.TrimSpace(ev.EventID) == "" {
		return fmt.Errorf("event_id required")
	}
	if strings.TrimSpace(string(ev.EventType)) == "" {
		return fmt.Errorf("event_type required")
	}
	if ev.Timestamp.IsZero() {
		return fmt.Errorf("timestamp required")
	}
	if strings.TrimSpace(string(ev.Source)) == "" {
		return fmt.Errorf("source required")
	}

	switch ev.EventType {
	case EventTypeItemStarted, EventTypeItemCompleted:
		if err := requirePayloadItem(ev.Payload, "item"); err != nil {
			return fmt.Errorf("%s: %w", ev.EventType, err)
		}
	case EventTypeItemDelta:
		if err := requirePayloadString(ev.Payload, "item_id"); err != nil {
			return fmt.Errorf("%s: %w", ev.EventType, err)
		}
		if err := requirePayloadString(ev.Payload, "delta"); err != nil {
			return fmt.Errorf("%s: %w", ev.EventType, err)
		}
	case EventTypeTextFinal, EventTypeTextReasoning:
		if err := requirePayloadString(ev.Payload, "text"); err != nil {
			return fmt.Errorf("%s: %w", ev.EventType, err)
		}
	case EventTypeApprovalReq:
		if err := requireRequestID(ev); err != nil {
			return fmt.Errorf("%s: %w", ev.EventType, err)
		}
		if err := requirePayloadString(ev.Payload, "approval_id"); err != nil {
			return fmt.Errorf("%s: %w", ev.EventType, err)
		}
	case EventTypeApprovalDone:
		if err := requireRequestID(ev); err != nil {
			return fmt.Errorf("%s: %w", ev.EventType, err)
		}
		if err := requirePayloadString(ev.Payload, "approval_id"); err != nil {
			return fmt.Errorf("%s: %w", ev.EventType, err)
		}
		if err := requirePayloadString(ev.Payload, "decision"); err != nil {
			return fmt.Errorf("%s: %w", ev.EventType, err)
		}
	case EventTypeInquiryReq, EventTypeInquiryDone:
		if err := requireRequestID(ev); err != nil {
			return fmt.Errorf("%s: %w", ev.EventType, err)
		}
		if err := requirePayloadString(ev.Payload, "inquiry_id"); err != nil {
			return fmt.Errorf("%s: %w", ev.EventType, err)
		}
	case EventTypeRequestStarted, EventTypeRequestDone, EventTypeRequestFailed:
		if err := requireRequestID(ev); err != nil {
			return fmt.Errorf("%s: %w", ev.EventType, err)
		}
		if ev.EventType == EventTypeRequestFailed {
			if eventString(ev.Payload, "error", "message", "text") == "" {
				return fmt.Errorf("%s: error payload required", ev.EventType)
			}
		}
	case EventTypeSessionUpdated:
		if strings.TrimSpace(ev.SessionID) == "" {
			return fmt.Errorf("%s: session_id required", ev.EventType)
		}
		if err := requirePayloadString(ev.Payload, "session_id"); err != nil {
			return fmt.Errorf("%s: %w", ev.EventType, err)
		}
	}

	return nil
}

func requireRequestID(ev Envelope) error {
	if strings.TrimSpace(ev.RequestID) == "" {
		return fmt.Errorf("request_id required")
	}
	return nil
}

func requirePayloadString(payload map[string]any, key string) error {
	if eventString(payload, key) == "" {
		return fmt.Errorf("payload.%s required", key)
	}
	return nil
}

func requirePayloadItem(payload map[string]any, key string) error {
	if len(payload) == 0 {
		return fmt.Errorf("payload.%s required", key)
	}
	raw, ok := payload[key]
	if !ok {
		return fmt.Errorf("payload.%s required", key)
	}
	if _, ok := raw.(map[string]any); !ok {
		return fmt.Errorf("payload.%s must be an object", key)
	}
	return nil
}

func eventString(payload map[string]any, keys ...string) string {
	if len(payload) == 0 {
		return ""
	}
	for _, key := range keys {
		raw, ok := payload[key]
		if !ok {
			continue
		}
		text, ok := raw.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text != "" {
			return text
		}
	}
	return ""
}
