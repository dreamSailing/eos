package jsonrpc

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRPCErrorPreservesErrorDataReason(t *testing.T) {
	err := error(NewRPCError(&Error{
		Code:    CodeInvalidParams,
		Message: "invalid params",
		Data:    json.RawMessage(`{"method":"turn/start","error_category":"invalid_params","reason":"session_id is required","trace":"trace-1"}`),
	}))

	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatal("errors.As did not recover RPCError")
	}
	if rpcErr.Reason != "session_id is required" {
		t.Fatalf("Reason=%q, want session_id is required", rpcErr.Reason)
	}
	if rpcErr.Method != "turn/start" || rpcErr.ErrorCategory != "invalid_params" || rpcErr.Trace != "trace-1" {
		t.Fatalf("unexpected RPCError metadata: %+v", rpcErr)
	}
	if !strings.Contains(err.Error(), "session_id is required") {
		t.Fatalf("Error()=%q, want reason included", err.Error())
	}
}
