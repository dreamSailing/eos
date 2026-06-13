package jsonrpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
	CodeBackpressure   = -32001
)

type RequestID struct {
	raw string
}

func StringID(id string) RequestID {
	data, _ := json.Marshal(id)
	return RequestID{raw: string(data)}
}

func NumberID(id int64) RequestID {
	return RequestID{raw: strconv.FormatInt(id, 10)}
}

func (id RequestID) IsZero() bool {
	return strings.TrimSpace(id.raw) == ""
}

func (id RequestID) String() string {
	if id.IsZero() {
		return ""
	}
	var s string
	if err := json.Unmarshal([]byte(id.raw), &s); err == nil {
		return s
	}
	return id.raw
}

func (id RequestID) MarshalJSON() ([]byte, error) {
	if id.IsZero() {
		return []byte("null"), nil
	}
	return []byte(id.raw), nil
}

func (id *RequestID) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*id = RequestID{}
		return nil
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return err
		}
		if strings.TrimSpace(s) == "" {
			return errors.New("jsonrpc id must not be blank")
		}
		*id = RequestID{raw: string(trimmed)}
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(trimmed, &n); err != nil {
		return fmt.Errorf("jsonrpc id must be a string or number: %w", err)
	}
	if _, err := strconv.ParseFloat(n.String(), 64); err != nil {
		return fmt.Errorf("jsonrpc id must be numeric: %w", err)
	}
	*id = RequestID{raw: n.String()}
	return nil
}

type Request struct {
	ID     RequestID       `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
	Trace  string          `json:"trace,omitempty"`
}

type Notification struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
	Trace  string          `json:"trace,omitempty"`
}

type Response struct {
	ID     RequestID       `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
	Trace  string          `json:"trace,omitempty"`
}

type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func NewRequest(id RequestID, method string, params any) (Request, error) {
	raw, err := marshalParams(params)
	if err != nil {
		return Request{}, err
	}
	return Request{ID: id, Method: strings.TrimSpace(method), Params: raw, Trace: GenerateTraceID()}, nil
}

var traceCounter atomic.Uint64

func GenerateTraceID() string {
	ts := time.Now().Unix()
	n := traceCounter.Add(1)
	return fmt.Sprintf("eos-%08x%08x", ts, n)
}

func NewNotification(method string, params any) (Notification, error) {
	raw, err := marshalParams(params)
	if err != nil {
		return Notification{}, err
	}
	return Notification{Method: strings.TrimSpace(method), Params: raw}, nil
}

func NewResultResponse(id RequestID, result any) (Response, error) {
	if result == nil {
		return Response{ID: id, Result: json.RawMessage("null")}, nil
	}
	raw, err := marshalParams(result)
	if err != nil {
		return Response{}, err
	}
	return Response{ID: id, Result: raw}, nil
}

func NewErrorResponse(id RequestID, code int, message string, data any) (Response, error) {
	raw, err := marshalParams(data)
	if err != nil {
		return Response{}, err
	}
	return Response{
		ID: id,
		Error: &Error{
			Code:    code,
			Message: strings.TrimSpace(message),
			Data:    raw,
		},
	}, nil
}

func (r Request) Validate() error {
	if r.ID.IsZero() {
		return errors.New("jsonrpc request id is required")
	}
	if strings.TrimSpace(r.Method) == "" {
		return errors.New("jsonrpc method is required")
	}
	return nil
}

func (n Notification) Validate() error {
	if strings.TrimSpace(n.Method) == "" {
		return errors.New("jsonrpc method is required")
	}
	return nil
}

func (r Response) Validate() error {
	if r.ID.IsZero() {
		return errors.New("jsonrpc response id is required")
	}
	if len(r.Result) > 0 && r.Error != nil {
		return errors.New("jsonrpc response cannot include both result and error")
	}
	if len(r.Result) == 0 && r.Error == nil {
		return errors.New("jsonrpc response needs result or error")
	}
	return nil
}

func marshalParams(v any) (json.RawMessage, error) {
	if v == nil {
		return nil, nil
	}
	if raw, ok := v.(json.RawMessage); ok {
		if len(raw) == 0 {
			return nil, nil
		}
		return append(json.RawMessage(nil), raw...), nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}
