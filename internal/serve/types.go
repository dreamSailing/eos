package serve

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"encoding/json"

	"github.com/dreamSailing/eos/pkg/coreapi"
)

type Options struct {
	Transport             string
	DefaultWorkspacePath  string
	DefaultAllowedTools   []string
	DefaultAccessMode     string
	DefaultApprovalMode   string
	DefaultSandboxMode    string
	PolicyPath            string
	SessionStorePath      string
	RequireApprovalDigest bool
	Engine                coreapi.Engine
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

type taskResumeParams struct {
	SessionID string `json:"sessionID"`
	TaskID    string `json:"taskID"`
	Task      string `json:"task,omitempty"`
}

type toolCallDTO struct {
	ID         string                 `json:"id"`
	Tool       string                 `json:"tool"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

type toolDefinitionDTO struct {
	Name               string                      `json:"name"`
	Description        string                      `json:"description"`
	RiskLevel          string                      `json:"riskLevel"`
	Params             map[string]parameterInfoDTO `json:"params,omitempty"`
	Examples           []map[string]any            `json:"examples,omitempty"`
	Source             string                      `json:"source,omitempty"`
	Category           string                      `json:"category,omitempty"`
	VisibleIn          []string                    `json:"visibleIn,omitempty"`
	ReadOnly           bool                        `json:"readOnly,omitempty"`
	Invocable          bool                        `json:"invocable"`
	RequiresFullAccess bool                        `json:"requiresFullAccess,omitempty"`
	Tags               []string                    `json:"tags,omitempty"`
	Metadata           map[string]any              `json:"metadata,omitempty"`
	Access             *toolAccessDTO              `json:"access,omitempty"`
}

type parameterInfoDTO struct {
	Type     string `json:"type,omitempty"`
	Required bool   `json:"required,omitempty"`
	Desc     string `json:"desc,omitempty"`
}

type toolAccessDTO struct {
	Mode           string `json:"mode"`
	AccessMode     string `json:"accessMode,omitempty"`
	ApprovalMode   string `json:"approvalMode,omitempty"`
	ApprovalSource string `json:"approvalSource,omitempty"`
	SandboxMode    string `json:"sandboxMode,omitempty"`
	Visible        bool   `json:"visible"`
	Executable     bool   `json:"executable"`
	NeedsApproval  bool   `json:"needsApproval"`
	Reason         string `json:"reason,omitempty"`
}

type executionModeDTO struct {
	Name             string   `json:"name"`
	Aliases          []string `json:"aliases,omitempty"`
	Description      string   `json:"description,omitempty"`
	ApprovalBehavior string   `json:"approvalBehavior,omitempty"`
}

type accessModeDTO struct {
	Name        string   `json:"name"`
	Aliases     []string `json:"aliases,omitempty"`
	Description string   `json:"description,omitempty"`
}

type approvalModeDTO struct {
	Name        string   `json:"name"`
	Aliases     []string `json:"aliases,omitempty"`
	Description string   `json:"description,omitempty"`
}
