package jsonrpc

import (
	"encoding/json"
	"fmt"
	"strings"
)

type RPCError struct {
	Code          int
	Message       string
	Data          json.RawMessage
	Method        string
	ServiceGroup  string
	Trace         string
	ErrorCategory string
	Reason        string
}

func NewRPCError(err *Error) *RPCError {
	if err == nil {
		return nil
	}
	out := &RPCError{
		Code:    err.Code,
		Message: strings.TrimSpace(err.Message),
		Data:    append(json.RawMessage(nil), err.Data...),
	}
	if len(out.Data) == 0 {
		return out
	}
	var data map[string]any
	if json.Unmarshal(out.Data, &data) != nil {
		return out
	}
	out.Method = stringValue(data, "method")
	out.ServiceGroup = stringValue(data, "service_group")
	out.Trace = stringValue(data, "trace")
	out.ErrorCategory = stringValue(data, "error_category", "category")
	out.Reason = stringValue(data, "reason", "error", "message", "detail")
	return out
}

func (e *RPCError) Error() string {
	if e == nil {
		return ""
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "remote error"
	}
	reason := strings.TrimSpace(e.Reason)
	if reason != "" && !strings.EqualFold(reason, message) {
		message += ": " + reason
	}
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, message)
}

func stringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, ok := values[key]
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
