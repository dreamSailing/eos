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

type sessionGetParams struct {
	SessionID string `json:"sessionID"`
}

type sessionResumeParams struct {
	SessionID string `json:"sessionID"`
}

type sessionDeleteParams struct {
	SessionID string `json:"sessionID"`
}

type requestStartParams struct {
	SessionID string      `json:"sessionID"`
	Call      toolCallDTO `json:"call"`
}

type requestCancelParams struct {
	SessionID string `json:"sessionID"`
	RequestID string `json:"requestID"`
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
	ApprovalID      string `json:"approvalID,omitempty"`
	InquiryID       string `json:"inquiryID,omitempty"`
	Decision        string `json:"decision"`
	ApprovalDigest  string `json:"approvalDigest"`
	Reason          string `json:"reason,omitempty"`
	PolicyID        string `json:"policyID,omitempty"`
	CorrelationID   string `json:"correlationID,omitempty"`
	ApproverTraceID string `json:"approverTraceID,omitempty"`
	Option          string `json:"option,omitempty"`
	Text            string `json:"text,omitempty"`
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

type toolDefinitionDTO struct {
	Name        string                      `json:"name"`
	Description string                      `json:"description"`
	RiskLevel   string                      `json:"riskLevel"`
	Params      map[string]parameterInfoDTO `json:"params,omitempty"`
	Examples    []map[string]any            `json:"examples,omitempty"`
	Source      string                      `json:"source,omitempty"`
	Category    string                      `json:"category,omitempty"`
	VisibleIn   []string                    `json:"visibleIn,omitempty"`
	ReadOnly    bool                        `json:"readOnly,omitempty"`
	Invocable   bool                        `json:"invocable"`
	Tags        []string                    `json:"tags,omitempty"`
	Metadata    map[string]any              `json:"metadata,omitempty"`
}

type parameterInfoDTO struct {
	Type     string `json:"type,omitempty"`
	Required bool   `json:"required,omitempty"`
	Desc     string `json:"desc,omitempty"`
}
