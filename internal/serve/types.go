package serve

import "encoding/json"

type Options struct {
	Transport             string
	DefaultWorkspacePath  string
	DefaultAllowedTools   []string
	PolicyPath            string
	RequireApprovalDigest bool
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *rpcError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

type sessionCreateParams struct {
	WorkspacePath string                 `json:"workspacePath"`
	Options       map[string]interface{} `json:"options,omitempty"`
}

type sessionCloseParams struct {
	SessionID string `json:"sessionID"`
}

type toolListParams struct {
	SessionID string `json:"sessionID"`
}

type toolExecuteParams struct {
	SessionID string      `json:"sessionID"`
	Call      toolCallDTO `json:"call"`
}

type toolPreflightParams struct {
	SessionID string      `json:"sessionID"`
	Call      toolCallDTO `json:"call"`
}

type promptResolveParams struct {
	SessionID       string `json:"sessionID"`
	RequestID       string `json:"requestID"`
	Decision        string `json:"decision"`
	ApprovalDigest  string `json:"approvalDigest"`
	Reason          string `json:"reason,omitempty"`
	PolicyID        string `json:"policyID,omitempty"`
	CorrelationID   string `json:"correlationID,omitempty"`
	ApproverTraceID string `json:"approverTraceID,omitempty"`
}

type toolCancelParams struct {
	SessionID string `json:"sessionID"`
	CallID    string `json:"callID"`
}

type taskListParams struct {
	SessionID string `json:"sessionID"`
}

type taskKillParams struct {
	SessionID string `json:"sessionID"`
	TaskID    string `json:"taskID"`
}

type toolCallDTO struct {
	ID         string                 `json:"id"`
	Tool       string                 `json:"tool"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

type eventDTO struct {
	Type          string `json:"type"`
	Ts            int64  `json:"ts"`
	SessionID     string `json:"sessionID,omitempty"`
	CorrelationID string `json:"correlationID,omitempty"`
	Message       string `json:"message,omitempty"`
	Call          any    `json:"call,omitempty"`
	Result        any    `json:"result,omitempty"`
	Request       any    `json:"request,omitempty"`
	Task          any    `json:"task,omitempty"`
}

type toolDefinitionDTO struct {
	Name        string                        `json:"name"`
	Description string                        `json:"description"`
	RiskLevel   string                        `json:"riskLevel"`
	Params      map[string]parameterInfoDTO   `json:"params,omitempty"`
	Examples    []map[string]any              `json:"examples,omitempty"`
}

type parameterInfoDTO struct {
	Type     string `json:"type,omitempty"`
	Required bool   `json:"required,omitempty"`
	Desc     string `json:"desc,omitempty"`
}
