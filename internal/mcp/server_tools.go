package mcp

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/toolapi"
	"github.com/google/uuid"
	mcpmodel "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

type controlToolDef struct {
	Name        string
	Description string
	Schema      json.RawMessage
	Handler     func(context.Context, mcpmodel.CallToolRequest) (*mcpmodel.CallToolResult, error)
}

func (s *Server) registerTools() {
	for _, item := range s.controlTools() {
		s.mcp.AddTool(mcpmodel.NewToolWithRawSchema(item.Name, item.Description, item.Schema), item.Handler)
	}
	for _, item := range s.runtimeTools() {
		s.mcp.AddTool(item.Tool, item.Handler)
	}
}

func (s *Server) runtimeTools() []mcpserver.ServerTool {
	defs := s.currentToolDefinitions(s.opts.DefaultWorkspacePath)
	sess := toolapi.ExecSession{
		WorkspaceRoot:         s.opts.DefaultWorkspacePath,
		AllowedTools:          allowedToolsMap(s.opts.DefaultAllowedTools),
		ExecutionMode:         "auto",
		AccessMode:            normalizeOptionalAccessMode(s.opts.DefaultAccessMode),
		ApprovalMode:          normalizeOptionalApprovalMode(s.opts.DefaultApprovalMode),
		SandboxMode:           s.opts.DefaultSandboxMode,
		RequireApprovalDigest: s.opts.RequireApprovalDigest,
	}
	visible := toolapi.FilterVisibleTools(defs, sess)
	sort.Slice(visible, func(i, j int) bool {
		return strings.ToLower(visible[i].Name) < strings.ToLower(visible[j].Name)
	})
	out := make([]mcpserver.ServerTool, 0, len(visible))
	for _, def := range visible {
		toolDef := def
		out = append(out, mcpserver.ServerTool{
			Tool:    buildMCPTool(toolDef),
			Handler: s.handleRuntimeTool(toolDef),
		})
	}
	return out
}

func buildMCPTool(def toolapi.ToolDefinition) mcpmodel.Tool {
	desc := strings.TrimSpace(def.Description)
	if desc == "" {
		desc = "EOS tool"
	}
	schema := buildToolSchema(def)
	tool := mcpmodel.NewToolWithRawSchema(def.Name, desc, schema)
	tool.Annotations = mcpmodel.ToolAnnotation{
		Title:           def.Name,
		ReadOnlyHint:    mcpmodel.ToBoolPtr(def.ReadOnly),
		DestructiveHint: mcpmodel.ToBoolPtr(!def.ReadOnly),
		IdempotentHint:  mcpmodel.ToBoolPtr(def.ReadOnly),
		OpenWorldHint:   mcpmodel.ToBoolPtr(def.RequiresFullAccess),
	}
	return tool
}

func buildToolSchema(def toolapi.ToolDefinition) json.RawMessage {
	properties := map[string]any{}
	required := make([]string, 0, len(def.Params))
	keys := make([]string, 0, len(def.Params))
	for key := range def.Params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		param := def.Params[key]
		properties[key] = map[string]any{
			"type":        schemaType(param.Type),
			"description": strings.TrimSpace(param.Desc),
		}
		if param.Required {
			required = append(required, key)
		}
	}
	properties["session_id"] = map[string]any{
		"type":        "string",
		"description": "可选：显式指定 EOS session ID；省略时使用当前连接的默认 session。",
	}
	properties["workspace_root"] = map[string]any{
		"type":        "string",
		"description": "可选：覆盖默认工作区，仅控制工具类接口使用；普通工具执行以 session 绑定工作区为准。",
	}
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	b, _ := json.Marshal(schema)
	return b
}

func schemaType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "string", "integer", "number", "boolean", "array", "object":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "string"
	}
}

func (s *Server) currentToolDefinitions(workspaceRoot string) []toolapi.ToolDefinition {
	ctx := context.Background()
	defs, err := s.services.Catalog().List(ctx)
	if err != nil {
		return nil
	}
	return defs
}

func (s *Server) handleRuntimeTool(def toolapi.ToolDefinition) func(context.Context, mcpmodel.CallToolRequest) (*mcpmodel.CallToolResult, error) {
	return func(ctx context.Context, request mcpmodel.CallToolRequest) (*mcpmodel.CallToolResult, error) {
		args := request.GetArguments()
		sess, err := s.ensureSession(ctx, args)
		if err != nil {
			return toolErrorResult(err.Error(), map[string]any{"tool": def.Name}), nil
		}
		cleanArgs := cloneMap(args)
		delete(cleanArgs, "session_id")
		delete(cleanArgs, "workspace_root")

		callID := stringArg(cleanArgs, "call_id")
		if callID == "" {
			callID = "call_" + strings.ReplaceAll(time.Now().Format("150405.000000000"), ".", "")
		}
		preview, err := s.buildPreview(sess, def, cleanArgs)
		if err != nil {
			return toolErrorResult(err.Error(), map[string]any{"tool": def.Name, "session_id": sess.ID}), nil
		}
		payload := map[string]any{
			"sessionID":  sess.ID,
			"tool":       def.Name,
			"parameters": cleanArgs,
			"preview":    preview,
		}
		digest, _, err := approvalDigest(payload)
		if err != nil {
			return toolErrorResult("approval digest failed", map[string]any{"tool": def.Name}), nil
		}

		if def.Name == "ask_user_question" {
			if inquiry, ok := s.getResolvedInquiry(sess, callID, digest); ok {
				return toolSuccessResult(map[string]any{
					"tool":       def.Name,
					"session_id": sess.ID,
					"option":     inquiry.Option,
					"text":       inquiry.Text,
				}, "User answered"), nil
			}
			pending := s.ensurePendingInquiry(sess, callID, def.Name, cleanArgs, digest)
			return toolErrorResult("inquiry required", map[string]any{
				"tool":        def.Name,
				"session_id":  sess.ID,
				"request_id":  pending.RequestID,
				"digest":      pending.Digest,
				"question":    pending.Question,
				"options":     append([]string(nil), pending.Options...),
				"expires_at":  pending.ExpiresAt.Format(time.RFC3339),
				"status":      "pending_inquiry",
				"related_uri": "eos://sessions/" + sess.ID + "/inquiries",
			}), nil
		}

		access := toolapi.EvaluateToolAccess(def, toolapi.ExecSession{
			WorkspaceRoot:         sess.WorkspaceAbs,
			AllowedTools:          sess.AllowedTools,
			ExecutionMode:         sess.ExecutionMode,
			AccessMode:            sess.AccessMode,
			ApprovalMode:          sess.ApprovalMode,
			SandboxMode:           sess.SandboxMode,
			RequireApprovalDigest: sess.RequireApprovalDigest,
		})
		if !access.Executable {
			return toolErrorResult("tool not allowed", map[string]any{
				"tool":       def.Name,
				"reason":     access.Reason,
				"session_id": sess.ID,
			}), nil
		}
		if access.NeedsApproval && !s.isApproved(sess, callID, digest) {
			pending := s.ensurePendingApproval(sess, callID, def.Name, cleanArgs, preview, digest)
			return toolErrorResult("approval required", map[string]any{
				"tool":                  def.Name,
				"session_id":            sess.ID,
				"request_id":            pending.RequestID,
				"digest":                pending.Digest,
				"preview":               cloneMap(pending.Preview),
				"expires_at":            pending.ExpiresAt.Format(time.RFC3339),
				"status":                "pending_approval",
				"approval_mode":         access.ApprovalMode,
				"approval_source":       access.ApprovalSource,
				"suggested_access_mode": suggestedUpgradeAccessMode(access),
				"related_uri":           "eos://sessions/" + sess.ID + "/approvals",
			}), nil
		}

		executor := s.services.NewExecutor(sess.WorkspaceAbs)
		results, err := executor.Execute(ctx, toolapi.ExecSession{
			WorkspaceRoot:         sess.WorkspaceAbs,
			AllowedTools:          sess.AllowedTools,
			TraceID:               callID,
			ExecutionMode:         sess.ExecutionMode,
			AccessMode:            sess.AccessMode,
			ApprovalMode:          sess.ApprovalMode,
			SandboxMode:           sess.SandboxMode,
			RequireApprovalDigest: sess.RequireApprovalDigest,
		}, []toolapi.ToolCall{{
			ID:     callID,
			Name:   def.Name,
			Params: cleanArgs,
		}})
		if err != nil {
			return toolErrorResult(err.Error(), map[string]any{"tool": def.Name, "session_id": sess.ID}), nil
		}
		if len(results) == 0 {
			return toolErrorResult("empty tool result", map[string]any{"tool": def.Name, "session_id": sess.ID}), nil
		}
		res := results[0]
		sess.touch(firstNonEmpty(res.Display, res.Error))
		return toolResultFromExecution(res, sess.ID), nil
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func suggestedUpgradeAccessMode(access toolapi.ToolAccess) string {
	if access.AccessMode == "danger-full-access" {
		return ""
	}
	if access.Reason == "sandbox_mode" || access.Reason == "access_mode" || access.AccessMode == "workspace-write" {
		return "danger-full-access"
	}
	return ""
}

func toolResultFromExecution(res toolapi.ToolResult, sessionID string) *mcpmodel.CallToolResult {
	payload := map[string]any{
		"id":         res.ID,
		"type":       res.Type,
		"tool":       res.Tool,
		"status":     res.Status,
		"data":       res.Data,
		"error":      res.Error,
		"display":    res.Display,
		"ts":         res.Ts,
		"session_id": sessionID,
	}
	text := firstNonEmpty(res.Display, res.Error)
	if text == "" {
		b, _ := json.Marshal(payload)
		text = string(b)
	}
	return &mcpmodel.CallToolResult{
		Content:           []mcpmodel.Content{mcpmodel.NewTextContent(text)},
		StructuredContent: payload,
		IsError:           strings.TrimSpace(strings.ToLower(res.Status)) != "" && !strings.EqualFold(res.Status, "success"),
	}
}

func toolErrorResult(message string, payload map[string]any) *mcpmodel.CallToolResult {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["error"] = strings.TrimSpace(message)
	return &mcpmodel.CallToolResult{
		Content:           []mcpmodel.Content{mcpmodel.NewTextContent(strings.TrimSpace(message))},
		StructuredContent: payload,
		IsError:           true,
	}
}

func toolSuccessResult(payload map[string]any, text string) *mcpmodel.CallToolResult {
	if payload == nil {
		payload = map[string]any{}
	}
	return &mcpmodel.CallToolResult{
		Content:           []mcpmodel.Content{mcpmodel.NewTextContent(strings.TrimSpace(text))},
		StructuredContent: payload,
	}
}

func (s *Server) controlTools() []controlToolDef {
	return []controlToolDef{
		{
			Name:        "eos_session_create",
			Description: "创建 EOS session，可选设置为当前连接默认 session。",
			Schema: mustJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"workspace_root":          map[string]any{"type": "string"},
					"execution_mode":          map[string]any{"type": "string"},
					"access_mode":             map[string]any{"type": "string"},
					"approval_mode":           map[string]any{"type": "string"},
					"sandbox_mode":            map[string]any{"type": "string"},
					"require_approval_digest": map[string]any{"type": "boolean"},
					"allowed_tools":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"set_default":             map[string]any{"type": "boolean"},
				},
			}),
			Handler: func(ctx context.Context, request mcpmodel.CallToolRequest) (*mcpmodel.CallToolResult, error) {
				sess, err := s.createSession(ctx, request.GetArguments())
				if err != nil {
					return toolErrorResult(err.Error(), nil), nil
				}
				return toolSuccessResult(map[string]any{"session": sess.snapshot()}, "Session created"), nil
			},
		},
		{
			Name:        "eos_session_list",
			Description: "列出当前 MCP server 中的 EOS sessions。",
			Schema:      mustJSON(map[string]any{"type": "object", "properties": map[string]any{}}),
			Handler: func(ctx context.Context, request mcpmodel.CallToolRequest) (*mcpmodel.CallToolResult, error) {
				items := s.listSessions()
				out := make([]map[string]any, 0, len(items))
				for _, item := range items {
					out = append(out, item.snapshot())
				}
				return toolSuccessResult(map[string]any{"sessions": out}, "Sessions listed"), nil
			},
		},
		{
			Name:        "eos_session_get",
			Description: "获取指定 EOS session 的详情；省略 `session_id` 时返回当前连接默认 session。",
			Schema: mustJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"session_id": map[string]any{"type": "string"},
				},
			}),
			Handler: func(ctx context.Context, request mcpmodel.CallToolRequest) (*mcpmodel.CallToolResult, error) {
				sess, err := s.ensureSession(ctx, request.GetArguments())
				if err != nil {
					return toolErrorResult(err.Error(), nil), nil
				}
				return toolSuccessResult(map[string]any{"session": sess.snapshot()}, "Session fetched"), nil
			},
		},
		{
			Name:        "eos_session_close",
			Description: "关闭一个 EOS session。",
			Schema: mustJSON(map[string]any{
				"type":     "object",
				"required": []string{"session_id"},
				"properties": map[string]any{
					"session_id": map[string]any{"type": "string"},
				},
			}),
			Handler: func(ctx context.Context, request mcpmodel.CallToolRequest) (*mcpmodel.CallToolResult, error) {
				sessionID := request.GetString("session_id", "")
				if !s.closeSession(sessionID) {
					return toolErrorResult("session not found", map[string]any{"session_id": sessionID}), nil
				}
				return toolSuccessResult(map[string]any{"ok": true, "session_id": sessionID}, "Session closed"), nil
			},
		},
		{
			Name:        "eos_approval_resolve",
			Description: "解决待审批请求。",
			Schema: mustJSON(map[string]any{
				"type":     "object",
				"required": []string{"request_id", "decision"},
				"properties": map[string]any{
					"session_id": map[string]any{"type": "string"},
					"request_id": map[string]any{"type": "string"},
					"decision":   map[string]any{"type": "string", "enum": []string{"allow_once", "allow_session", "deny"}},
					"policy_id":  map[string]any{"type": "string"},
					"reason":     map[string]any{"type": "string"},
				},
			}),
			Handler: func(ctx context.Context, request mcpmodel.CallToolRequest) (*mcpmodel.CallToolResult, error) {
				sess, err := s.ensureSession(ctx, request.GetArguments())
				if err != nil {
					return toolErrorResult(err.Error(), nil), nil
				}
				item, err := s.resolveApproval(sess, request.GetString("request_id", ""), request.GetString("decision", ""), request.GetString("policy_id", ""), request.GetString("reason", ""))
				if err != nil {
					return toolErrorResult(err.Error(), map[string]any{"session_id": sess.ID}), nil
				}
				return toolSuccessResult(map[string]any{"approval": pendingPromptSnapshot(item), "session_id": sess.ID}, "Approval resolved"), nil
			},
		},
		{
			Name:        "eos_inquiry_resolve",
			Description: "回答待用户确认/提问请求。",
			Schema: mustJSON(map[string]any{
				"type":     "object",
				"required": []string{"request_id"},
				"properties": map[string]any{
					"session_id": map[string]any{"type": "string"},
					"request_id": map[string]any{"type": "string"},
					"option":     map[string]any{"type": "string"},
					"text":       map[string]any{"type": "string"},
				},
			}),
			Handler: func(ctx context.Context, request mcpmodel.CallToolRequest) (*mcpmodel.CallToolResult, error) {
				sess, err := s.ensureSession(ctx, request.GetArguments())
				if err != nil {
					return toolErrorResult(err.Error(), nil), nil
				}
				item, err := s.resolveInquiry(sess, request.GetString("request_id", ""), request.GetString("option", ""), request.GetString("text", ""))
				if err != nil {
					return toolErrorResult(err.Error(), map[string]any{"session_id": sess.ID}), nil
				}
				return toolSuccessResult(map[string]any{"inquiry": pendingPromptSnapshot(item), "session_id": sess.ID}, "Inquiry resolved"), nil
			},
		},
		{
			Name:        "eos_task_list",
			Description: "列出 EOS 当前后台任务。",
			Schema:      mustJSON(map[string]any{"type": "object", "properties": map[string]any{}}),
			Handler: func(ctx context.Context, request mcpmodel.CallToolRequest) (*mcpmodel.CallToolResult, error) {
				items, err := s.services.Tasks().List(ctx)
				if err != nil {
					return toolErrorResult(err.Error(), nil), nil
				}
				return toolSuccessResult(map[string]any{"tasks": taskInfoList(items)}, "Tasks listed"), nil
			},
		},
		{
			Name:        "eos_task_kill",
			Description: "终止一个 EOS 后台任务。",
			Schema: mustJSON(map[string]any{
				"type":     "object",
				"required": []string{"task_id"},
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string"},
				},
			}),
			Handler: func(ctx context.Context, request mcpmodel.CallToolRequest) (*mcpmodel.CallToolResult, error) {
				taskID := request.GetString("task_id", "")
				if err := s.services.Tasks().Kill(ctx, taskID); err != nil {
					return toolErrorResult(err.Error(), map[string]any{"task_id": taskID}), nil
				}
				return toolSuccessResult(map[string]any{"ok": true, "task_id": taskID}, "Task killed"), nil
			},
		},
		{
			Name:        "eos_task_resume",
			Description: "恢复一个 EOS 任务。",
			Schema: mustJSON(map[string]any{
				"type":     "object",
				"required": []string{"task_id"},
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string"},
				},
			}),
			Handler: func(ctx context.Context, request mcpmodel.CallToolRequest) (*mcpmodel.CallToolResult, error) {
				taskID := request.GetString("task_id", "")
				if err := s.services.Tasks().Resume(ctx, taskID); err != nil {
					return toolErrorResult(err.Error(), map[string]any{"task_id": taskID}), nil
				}
				return toolSuccessResult(map[string]any{"ok": true, "task_id": taskID}, "Task resumed"), nil
			},
		},
		{
			Name:        "eos_task_close",
			Description: "关闭一个 EOS 任务。",
			Schema: mustJSON(map[string]any{
				"type":     "object",
				"required": []string{"task_id"},
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string"},
				},
			}),
			Handler: func(ctx context.Context, request mcpmodel.CallToolRequest) (*mcpmodel.CallToolResult, error) {
				taskID := request.GetString("task_id", "")
				if err := s.services.Tasks().Close(ctx, taskID); err != nil {
					return toolErrorResult(err.Error(), map[string]any{"task_id": taskID}), nil
				}
				return toolSuccessResult(map[string]any{"ok": true, "task_id": taskID}, "Task closed"), nil
			},
		},
	}
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func (s *Server) ensurePendingApproval(sess *runtimeSession, callID, tool string, params map[string]any, preview map[string]any, digest string) *pendingPrompt {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	for _, item := range sess.Approvals {
		if item.Digest == digest && time.Now().Before(item.ExpiresAt) && item.Decision == "" {
			return item
		}
	}
	item := &pendingPrompt{
		RequestID:  "apr_" + uuid.NewString()[:12],
		Kind:       "approval",
		Tool:       tool,
		CallID:     callID,
		Digest:     digest,
		Preview:    cloneMap(preview),
		Parameters: cloneMap(params),
		ExpiresAt:  time.Now().Add(60 * time.Second),
		CreatedAt:  time.Now(),
	}
	sess.Approvals[item.RequestID] = item
	sess.UpdatedAt = time.Now()
	return item
}

func (s *Server) ensurePendingInquiry(sess *runtimeSession, callID, tool string, params map[string]any, digest string) *pendingPrompt {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	for _, item := range sess.Inquiries {
		if item.Digest == digest && time.Now().Before(item.ExpiresAt) && item.Decision == "" {
			return item
		}
	}
	item := &pendingPrompt{
		RequestID:  "inq_" + uuid.NewString()[:12],
		Kind:       "inquiry",
		Tool:       tool,
		CallID:     callID,
		Digest:     digest,
		Parameters: cloneMap(params),
		Question:   stringArg(params, "question"),
		Options:    parseStringSliceArg(params["options"]),
		ExpiresAt:  time.Now().Add(time.Hour),
		CreatedAt:  time.Now(),
	}
	sess.Inquiries[item.RequestID] = item
	sess.UpdatedAt = time.Now()
	return item
}

func (s *Server) isApproved(sess *runtimeSession, callID, digest string) bool {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	now := time.Now()
	for _, item := range sess.Approvals {
		if item.Digest != digest || now.After(item.ExpiresAt) {
			continue
		}
		switch item.Decision {
		case "allow_session":
			return true
		case "allow_once":
			if !item.Used {
				item.Used = true
				item.ResolvedAt = now
				return true
			}
		}
	}
	return false
}

func (s *Server) getResolvedInquiry(sess *runtimeSession, callID, digest string) (*pendingPrompt, bool) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	now := time.Now()
	for _, item := range sess.Inquiries {
		if item.Digest != digest || now.After(item.ExpiresAt) {
			continue
		}
		if item.Decision == "resolve" && !item.Used {
			item.Used = true
			item.ResolvedAt = now
			return item, true
		}
	}
	return nil, false
}

func (s *Server) resolveApproval(sess *runtimeSession, requestID, decision, policyID, reason string) (*pendingPrompt, error) {
	requestID = strings.TrimSpace(requestID)
	decision = strings.ToLower(strings.TrimSpace(decision))
	if requestID == "" {
		return nil, fmt.Errorf("request_id required")
	}
	if decision != "allow_once" && decision != "allow_session" && decision != "deny" {
		return nil, fmt.Errorf("invalid decision")
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	item := sess.Approvals[requestID]
	if item == nil {
		return nil, fmt.Errorf("approval not found")
	}
	item.Decision = decision
	item.PolicyID = strings.TrimSpace(policyID)
	item.Reason = strings.TrimSpace(reason)
	item.ResolvedAt = time.Now()
	sess.UpdatedAt = time.Now()
	sess.LastAuthorization = map[string]any{
		"decision":           item.Decision,
		"tool":               item.Tool,
		"reason":             item.Reason,
		"target_access_mode": item.TargetAccessMode,
		"at":                 item.ResolvedAt.Format(time.RFC3339),
	}
	return item, nil
}

func (s *Server) resolveInquiry(sess *runtimeSession, requestID, option, text string) (*pendingPrompt, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, fmt.Errorf("request_id required")
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	item := sess.Inquiries[requestID]
	if item == nil {
		return nil, fmt.Errorf("inquiry not found")
	}
	item.Decision = "resolve"
	item.Option = strings.TrimSpace(option)
	item.Text = strings.TrimSpace(text)
	item.ResolvedAt = time.Now()
	sess.UpdatedAt = time.Now()
	return item, nil
}

func taskInfoList(items []toolapi.TaskInfo) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"id":         item.ID,
			"kind":       item.Kind,
			"status":     item.Status,
			"started_at": timeString(item.StartedAt),
			"updated_at": timeString(item.UpdatedAt),
			"ended_at":   timeString(item.EndedAt),
			"label":      item.Label,
			"summary":    item.Summary,
			"can_kill":   item.CanKill,
			"can_resume": item.CanResume,
			"can_close":  item.CanClose,
			"metadata":   item.Metadata,
		})
	}
	return out
}

func (s *Server) buildPreview(sess *runtimeSession, def toolapi.ToolDefinition, params map[string]any) (map[string]any, error) {
	if err := s.checkWorkspaceConstraints(sess, def.Name, params); err != nil {
		return nil, err
	}
	switch strings.ToLower(def.Name) {
	case "bash":
		cmd := strings.TrimSpace(stringArg(params, "command"))
		if cmd == "" {
			return map[string]any{}, nil
		}
		if rule := s.policy.findRule("bash", string(def.RiskLevel)); rule != nil {
			allowed := rule.allowedCommands()
			if len(allowed) > 0 && !containsExact(allowed, cmd) {
				return nil, fmt.Errorf("tool not allowed")
			}
		}
		return map[string]any{"command": cmd, "safetyFindings": []any{}}, nil
	case "edit":
		return map[string]any{
			"mode": strings.TrimSpace(stringArg(params, "mode")),
			"file": strings.TrimSpace(stringArg(params, "file")),
		}, nil
	case "fs":
		return map[string]any{
			"mode":        strings.TrimSpace(stringArg(params, "mode")),
			"path":        strings.TrimSpace(stringArg(params, "path")),
			"source":      strings.TrimSpace(stringArg(params, "source")),
			"destination": strings.TrimSpace(stringArg(params, "destination")),
		}, nil
	default:
		return map[string]any{}, nil
	}
}

func (s *Server) checkWorkspaceConstraints(sess *runtimeSession, toolName string, params map[string]any) error {
	if sess == nil || strings.TrimSpace(sess.WorkspaceAbs) == "" {
		return nil
	}
	risk := ""
	defs := s.currentToolDefinitions(sess.WorkspaceAbs)
	if def, ok := toolapi.FindToolDefinition(defs, toolName); ok {
		risk = string(def.RiskLevel)
	}
	for _, field := range []string{"path", "file", "source", "destination", "working_dir", "root"} {
		raw := strings.TrimSpace(stringArg(params, field))
		if raw == "" {
			continue
		}
		abs, ok, err := resolveInWorkspace(sess.WorkspaceAbs, raw)
		if err != nil {
			return fmt.Errorf("invalid path parameter: %s", field)
		}
		if !ok && sess.SandboxMode != "full_access" {
			return fmt.Errorf("workspace violation: %s", raw)
		}
		if rule := s.policy.findRule(toolName, risk); rule != nil {
			for _, pat := range rule.denyPathGlobs() {
				if matchDenyGlob(pat, filepath.ToSlash(abs)) {
					return fmt.Errorf("tool not allowed")
				}
			}
		}
	}
	return nil
}
