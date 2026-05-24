package jsonrpc

import (
	"bytes"
	"encoding/json"
	"errors"
)

type MessageKind string

const (
	KindRequest      MessageKind = "request"
	KindNotification MessageKind = "notification"
	KindResponse     MessageKind = "response"
)

type DecodedMessage struct {
	Kind         MessageKind
	Request      *Request
	Notification *Notification
	Response     *Response
}

func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func Decode(data []byte) (DecodedMessage, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return DecodedMessage{}, errors.New("empty jsonrpc message")
	}

	var probe struct {
		ID     *RequestID       `json:"id"`
		Method string           `json:"method"`
		Result *json.RawMessage `json:"result"`
		Error  *Error           `json:"error"`
	}
	if err := json.Unmarshal(trimmed, &probe); err != nil {
		return DecodedMessage{}, err
	}

	if probe.Method != "" {
		if probe.ID == nil || probe.ID.IsZero() {
			var notification Notification
			if err := json.Unmarshal(trimmed, &notification); err != nil {
				return DecodedMessage{}, err
			}
			return DecodedMessage{Kind: KindNotification, Notification: &notification}, notification.Validate()
		}
		var request Request
		if err := json.Unmarshal(trimmed, &request); err != nil {
			return DecodedMessage{}, err
		}
		return DecodedMessage{Kind: KindRequest, Request: &request}, request.Validate()
	}

	var response Response
	if err := json.Unmarshal(trimmed, &response); err != nil {
		return DecodedMessage{}, err
	}
	return DecodedMessage{Kind: KindResponse, Response: &response}, response.Validate()
}
