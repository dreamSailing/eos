package serve

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dreamSailing/eos/internal/toolapi"
	"github.com/dreamSailing/eos/internal/tools"
	"github.com/dreamSailing/eos/pkg/protocol"
	"github.com/google/uuid"
)

type Server struct {
	opts Options
	in   io.Reader
	out  io.Writer
	err  io.Writer

	mu               sync.Mutex
	writeMu          sync.Mutex
	initialized      bool
	sessionStorePath string
	sessions         map[string]*session
	toolDefs         []toolapi.ToolDefinition
	policy           *Policy
	tools            toolapi.Services
	catalog          toolapi.Catalog
}

type session struct {
	id                     string
	workspaceAbs           string
	title                  string
	preview                string
	allowedTools           map[string]bool
	executionMode          string
	accessMode             string
	approvalMode           string
	sandboxMode            string
	trustedWorkspace       bool
	maxConcurrentToolCalls int
	requireApprovalDigest  bool
	confirmPolicyID        string
	approvals              map[string]*approval
	allowSession           map[string]time.Time
	lastAuthorization      *authorizationStatus
	results                map[string]any
	runningCancels         map[string]context.CancelFunc
	updatedAt              time.Time
	exec                   toolapi.Executor
}

type approval struct {
	requestID        string
	kind             string
	callID           string
	tool             string
	parameters       map[string]any
	preview          map[string]any
	digest           string
	expiresAt        time.Time
	decision         string
	used             bool
	policyID         string
	reason           string
	option           string
	text             string
	triggerReason    string
	targetAccessMode string
	approvalSource   string
}

type authorizationStatus struct {
	Decision         string
	Category         string
	Tool             string
	Summary          string
	Reason           string
	TargetAccessMode string
	At               time.Time
}

func NewServer(opts Options, in io.Reader, out io.Writer, errw io.Writer, toolsSvc toolapi.Services) (*Server, error) {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	if errw == nil {
		errw = os.Stderr
	}
	if strings.TrimSpace(opts.Transport) == "" {
		opts.Transport = "stdio"
	}
	if opts.Transport != "stdio" {
		return nil, fmt.Errorf("unsupported transport: %s", opts.Transport)
	}
	p, err := LoadPolicy(opts.PolicyPath)
	if err != nil {
		return nil, err
	}
	if toolsSvc == nil {
		return nil, fmt.Errorf("tools service required")
	}

	s := &Server{
		opts:             opts,
		in:               in,
		out:              out,
		err:              errw,
		sessionStorePath: resolveSessionStorePath(opts),
		sessions:         map[string]*session{},
		policy:           p,
		tools:            toolsSvc,
		catalog:          toolsSvc.Catalog(),
	}
	s.toolDefs, _ = s.catalog.List(context.Background())
	if err := s.loadPersistedSessions(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Server) Run(ctx context.Context) error {
	sc := bufio.NewScanner(s.in)
	buf := make([]byte, 0, 1024*1024)
	sc.Buffer(buf, 10*1024*1024)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				return err
			}
			return nil
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var req rpcRequest
		if err := decodeJSONLine([]byte(line), &req); err != nil {
			s.writeStderr("invalid json: " + err.Error())
			continue
		}
		s.handleRequest(ctx, req)
	}
}

func (s *Server) handleRequest(ctx context.Context, req rpcRequest) {
	method := strings.TrimSpace(req.Method)
	if method == "" {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}

	if method != "initialize" && !s.isInitialized() {
		s.reply(req.ID, nil, &rpcError{Code: -32001, Message: "Unauthorized"})
		return
	}

	switch method {
	case "initialize":
		s.handleInitialize(req)
	case "session.create":
		s.handleSessionCreate(req)
	case "session.get":
		s.handleSessionGet(req)
	case "session.list":
		s.handleSessionList(req)
	case "session.resume":
		s.handleSessionResume(req)
	case "session.close":
		s.handleSessionClose(req)
	case "session.delete":
		s.handleSessionDelete(req)
	case "request.start":
		s.handleRequestStart(ctx, req)
	case "request.cancel":
		s.handleRequestCancel(req)
	case "tool.list":
		s.handleToolList(req)
	case "capability.list":
		s.handleCapabilityList(req)
	case "tool.preflight":
		s.handleToolPreflight(req)
	case "approval.resolve":
		s.handleApprovalResolve(req)
	case "inquiry.resolve":
		s.handleInquiryResolve(req)
	case "prompt.resolve":
		s.handlePromptResolve(req)
	case "tool.execute":
		s.handleToolExecute(ctx, req)
	case "tool.cancel":
		s.handleToolCancel(req)
	case "task.list":
		s.handleTaskList(req)
	case "task.resume":
		s.handleTaskResume(req)
	case "task.cancel":
		s.handleTaskCancel(req)
	case "task.kill":
		s.handleTaskKill(req)
	case "task.close":
		s.handleTaskClose(req)
	default:
		s.reply(req.ID, nil, &rpcError{Code: -32004, Message: "MethodNotFound"})
	}
}

func (s *Server) isInitialized() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.initialized
}

func (s *Server) handleInitialize(req rpcRequest) {
	s.mu.Lock()
	s.initialized = true
	s.mu.Unlock()
	s.reply(req.ID, map[string]any{
		"server": map[string]any{
			"name":    "eos",
			"version": "dev",
		},
		"protocolVersion": serveProtocolVersion,
		"capabilities":    serverCapabilitiesPayload(),
	}, nil)
}

func (s *Server) handleSessionCreate(req rpcRequest) {
	var p sessionCreateParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	workspace := strings.TrimSpace(p.WorkspacePath)
	if workspace == "" {
		workspace = strings.TrimSpace(s.opts.DefaultWorkspacePath)
	}
	if workspace == "" {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams", Data: map[string]any{"field": "workspacePath"}})
		return
	}
	abs, err := filepath.Abs(workspace)
	if err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams", Data: map[string]any{"field": "workspacePath"}})
		return
	}
	allowed := map[string]bool{}
	for _, t := range s.opts.DefaultAllowedTools {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		allowed[t] = true
	}
	requireDigest := s.opts.RequireApprovalDigest
	confirmPolicyID := ""
	executionMode := "auto"
	accessMode := ""
	if strings.TrimSpace(s.opts.DefaultAccessMode) != "" {
		accessMode = toolapi.NormalizeAccessMode(s.opts.DefaultAccessMode)
	}
	approvalMode := ""
	if strings.TrimSpace(s.opts.DefaultApprovalMode) != "" {
		approvalMode = toolapi.NormalizeApprovalMode(s.opts.DefaultApprovalMode)
	}
	sandboxMode := toolapi.NormalizeSandboxMode(s.opts.DefaultSandboxMode)
	trustedWorkspace := false
	maxConcurrent := 1
	title := defaultSessionTitle(abs)
	if p.Options != nil {
		if v, ok := p.Options["title"].(string); ok {
			if normalized := normalizeSessionPreview(v); normalized != "" {
				title = normalized
			}
		}
		if v, ok := p.Options["executionMode"].(string); ok {
			executionMode = toolapi.NormalizeExecutionMode(v)
		}
		if v, ok := p.Options["accessMode"].(string); ok {
			accessMode = toolapi.NormalizeAccessMode(v)
			sandboxMode = toolapi.SandboxModeFromAccessMode(accessMode)
		}
		if v, ok := p.Options["approvalMode"].(string); ok {
			approvalMode = toolapi.NormalizeApprovalMode(v)
		}
		if v, ok := p.Options["sandboxMode"].(string); ok {
			sandboxMode = toolapi.NormalizeSandboxMode(v)
		}
		if v, ok := p.Options["trustedWorkspace"].(bool); ok {
			trustedWorkspace = v
		}
		if v, ok := p.Options["maxConcurrentToolCalls"].(float64); ok {
			maxConcurrent = int(v)
		} else if v, ok := p.Options["maxConcurrentToolCalls"].(int); ok {
			maxConcurrent = v
		}
		if maxConcurrent <= 0 {
			maxConcurrent = 1
		}
		if v, ok := p.Options["requireApprovalDigest"].(bool); ok {
			requireDigest = v
		}
		if v, ok := p.Options["confirmPolicyID"].(string); ok {
			confirmPolicyID = strings.TrimSpace(v)
		}
		if v, ok := p.Options["allowedTools"].([]interface{}); ok {
			allowed = map[string]bool{}
			for _, it := range v {
				sv, ok := it.(string)
				if !ok {
					continue
				}
				sv = strings.ToLower(strings.TrimSpace(sv))
				if sv == "" {
					continue
				}
				allowed[sv] = true
			}
			if len(s.opts.DefaultAllowedTools) > 0 {
				for k := range allowed {
					if !s.isServerToolAllowed(k) {
						delete(allowed, k)
					}
				}
			}
		}
	}
	id := "s_" + uuid.New().String()[:12]
	exec := s.tools.NewExecutor(abs)
	sess := &session{
		id:                     id,
		workspaceAbs:           abs,
		title:                  title,
		allowedTools:           allowed,
		executionMode:          executionMode,
		accessMode:             accessMode,
		approvalMode:           approvalMode,
		sandboxMode:            sandboxMode,
		trustedWorkspace:       trustedWorkspace,
		maxConcurrentToolCalls: maxConcurrent,
		requireApprovalDigest:  requireDigest,
		confirmPolicyID:        confirmPolicyID,
		approvals:              map[string]*approval{},
		allowSession:           map[string]time.Time{},
		results:                map[string]any{},
		runningCancels:         map[string]context.CancelFunc{},
		updatedAt:              time.Now(),
		exec:                   exec,
	}

	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()

	s.notifySessionUpdated(sess)
	s.reply(req.ID, map[string]any{"sessionID": id}, nil)
}

func (s *Server) isServerToolAllowed(tool string) bool {
	if len(s.opts.DefaultAllowedTools) == 0 {
		return true
	}
	for _, t := range s.opts.DefaultAllowedTools {
		if strings.EqualFold(strings.TrimSpace(t), strings.TrimSpace(tool)) {
			return true
		}
	}
	return false
}

func (s *Server) handleSessionClose(req rpcRequest) {
	var p sessionCloseParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	sess := s.removeSession(strings.TrimSpace(p.SessionID))
	if sess == nil {
		s.reply(req.ID, nil, &rpcError{Code: -32002, Message: "SessionNotFound"})
		return
	}
	if err := s.persistSessions(); err != nil {
		s.writeStderr("persist sessions: " + err.Error())
	}
	s.reply(req.ID, map[string]any{"ok": true}, nil)
}

func (s *Server) handleSessionGet(req rpcRequest) {
	var p sessionGetParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	sessionID := strings.TrimSpace(p.SessionID)
	if sessionID == "" {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}

	s.mu.Lock()
	sess := s.sessions[sessionID]
	if sess == nil {
		s.mu.Unlock()
		s.reply(req.ID, nil, &rpcError{Code: -32002, Message: "SessionNotFound"})
		return
	}
	info := s.sessionInfoLocked(sess)
	s.mu.Unlock()

	s.reply(req.ID, map[string]any{"session": info}, nil)
}

func (s *Server) handleSessionList(req rpcRequest) {
	s.mu.Lock()
	items := make([]protocol.SessionInfo, 0, len(s.sessions))
	for _, sess := range s.sessions {
		items = append(items, s.sessionInfoLocked(sess))
	}
	s.mu.Unlock()

	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return strings.Compare(items[i].SessionID, items[j].SessionID) < 0
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	s.reply(req.ID, map[string]any{"sessions": items}, nil)
}

func (s *Server) handleSessionResume(req rpcRequest) {
	var p sessionResumeParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	sessionID := strings.TrimSpace(p.SessionID)
	if sessionID == "" {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}

	s.mu.Lock()
	sess := s.sessions[sessionID]
	if sess == nil {
		s.mu.Unlock()
		s.reply(req.ID, nil, &rpcError{Code: -32002, Message: "SessionNotFound"})
		return
	}
	sess.updatedAt = time.Now()
	info := s.sessionInfoLocked(sess)
	s.mu.Unlock()

	s.notifySessionUpdated(sess)
	s.reply(req.ID, map[string]any{"session": info}, nil)
}

func (s *Server) handleSessionDelete(req rpcRequest) {
	var p sessionDeleteParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	sess := s.removeSession(strings.TrimSpace(p.SessionID))
	if sess == nil {
		s.reply(req.ID, nil, &rpcError{Code: -32002, Message: "SessionNotFound"})
		return
	}
	if err := s.persistSessions(); err != nil {
		s.writeStderr("persist sessions: " + err.Error())
	}
	s.reply(req.ID, map[string]any{"ok": true}, nil)
}

func (s *Server) handleRequestStart(ctx context.Context, req rpcRequest) {
	var p requestStartParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	s.handleCallExecute(ctx, req, p.SessionID, p.Call)
}

func (s *Server) handleRequestCancel(req rpcRequest) {
	var p requestCancelParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	s.handleCallCancel(req, p.SessionID, p.RequestID)
}

func (s *Server) handleToolList(req rpcRequest) {
	var p toolListParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	sess := s.getSession(strings.TrimSpace(p.SessionID))
	if sess == nil {
		s.reply(req.ID, nil, &rpcError{Code: -32002, Message: "SessionNotFound"})
		return
	}
	execSess := sessionExecSession(sess)
	allDefs := s.currentToolDefinitions(sess.workspaceAbs)
	defs := toolapi.FilterVisibleTools(allDefs, execSess)
	catalog := defsToDTOsForSession(allDefs, execSess)
	s.reply(req.ID, map[string]any{
		"tools":          defsToDTOsForSession(defs, execSess),
		"catalog":        catalog,
		"mode":           execSess.ExecutionMode,
		"accessMode":     toolapi.ResolveAccessMode(execSess),
		"approvalMode":   toolapi.ResolveApprovalMode(execSess),
		"sandboxMode":    execSess.SandboxMode,
		"modeProfile":    modeDTO(execSess.ExecutionMode),
		"executionModes": modeDTOs(toolapi.SupportedExecutionModes()),
		"accessModes":    accessModeDTOs(toolapi.SupportedAccessModes()),
		"approvalModes":  approvalModeDTOs(toolapi.SupportedApprovalModes()),
		"summary":        buildCatalogSummary(catalog),
	}, nil)
}

func (s *Server) handleCapabilityList(req rpcRequest) {
	var p toolListParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	sess := s.getSession(strings.TrimSpace(p.SessionID))
	if sess == nil {
		s.reply(req.ID, nil, &rpcError{Code: -32002, Message: "SessionNotFound"})
		return
	}
	execSess := sessionExecSession(sess)
	allDefs := s.currentToolDefinitions(sess.workspaceAbs)
	defs := toolapi.FilterVisibleCapabilities(allDefs, execSess)
	items := defsToDTOsForSession(defs, execSess)
	catalog := defsToDTOsForSession(allDefs, execSess)
	s.reply(req.ID, map[string]any{
		"capabilities":   items,
		"tools":          items,
		"catalog":        catalog,
		"mode":           execSess.ExecutionMode,
		"accessMode":     toolapi.ResolveAccessMode(execSess),
		"approvalMode":   toolapi.ResolveApprovalMode(execSess),
		"sandboxMode":    execSess.SandboxMode,
		"modeProfile":    modeDTO(execSess.ExecutionMode),
		"executionModes": modeDTOs(toolapi.SupportedExecutionModes()),
		"accessModes":    accessModeDTOs(toolapi.SupportedAccessModes()),
		"approvalModes":  approvalModeDTOs(toolapi.SupportedApprovalModes()),
		"summary":        buildCatalogSummary(catalog),
	}, nil)
}

func (s *Server) handleToolPreflight(req rpcRequest) {
	var p toolPreflightParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	sess := s.getSession(strings.TrimSpace(p.SessionID))
	if sess == nil {
		s.reply(req.ID, nil, &rpcError{Code: -32002, Message: "SessionNotFound"})
		return
	}
	call := p.Call
	call.Tool = strings.TrimSpace(call.Tool)
	if call.ID == "" || call.Tool == "" {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	var err error
	call.Parameters, err = normalizePathsInParams(sess.workspaceAbs, call.Parameters)
	if err != nil {
		s.reply(req.ID, nil, errToRPC(err))
		return
	}
	defs := s.currentToolDefinitions(sess.workspaceAbs)
	def, ok := toolapi.FindToolDefinition(defs, call.Tool)
	if !ok {
		s.reply(req.ID, nil, &rpcError{Code: -32003, Message: "ToolNotAllowed"})
		return
	}
	access := toolapi.EvaluateToolAccess(def, sessionExecSession(sess))
	suggestedAccessMode := suggestedUpgradeAccessMode(access)
	if !access.Visible || !access.Executable {
		s.reply(req.ID, nil, &rpcError{
			Code:    -32003,
			Message: "ToolNotAllowed",
			Data: preflightContextData(sess, access, map[string]any{
				"triggerReason":       firstNonEmpty(access.Reason, "tool_not_allowed"),
				"suggestedAccessMode": suggestedAccessMode,
			}),
		})
		return
	}

	risk := string(def.RiskLevel)
	preview, err := s.buildPreview(sess, call)
	if err != nil {
		rpcErr := errToRPC(err)
		rpcErr.Data = preflightContextData(sess, access, mergeAnyMap(asMap(rpcErr.Data), map[string]any{
			"triggerReason":       preflightTriggerReasonFromError(rpcErr),
			"suggestedAccessMode": suggestedAccessModeForError(access, rpcErr),
		}))
		s.reply(req.ID, nil, rpcErr)
		return
	}

	payload := map[string]any{
		"sessionID":     sess.id,
		"relatedCallID": call.ID,
		"tool":          call.Tool,
		"parameters":    call.Parameters,
		"preview":       preview,
	}
	digest, _, err := approvalDigest(payload)
	if err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32012, Message: "Internal"})
		return
	}
	ttl := int64(60)
	if risk == "high" {
		ttl = 30
	}
	expiresAt := time.Now().Add(time.Duration(ttl) * time.Second)

	out := map[string]any{
		"riskLevel":           risk,
		"accessMode":          access.AccessMode,
		"approvalMode":        access.ApprovalMode,
		"approvalSource":      access.ApprovalSource,
		"sandboxMode":         access.SandboxMode,
		"preview":             preview,
		"approvalDigest":      digest,
		"ttlSeconds":          ttl,
		"triggerReason":       "",
		"suggestedAccessMode": suggestedAccessMode,
		"workspaceBoundary": map[string]any{
			"root":       strings.TrimSpace(sess.workspaceAbs),
			"tempDirs":   toolsAllowedTempDirs(),
			"enforced":   access.AccessMode == "workspace-write",
			"accessMode": access.AccessMode,
		},
	}

	if access.NeedsApproval {
		requestID := "r_" + uuid.New().String()[:12]
		a := &approval{
			requestID:        requestID,
			kind:             "approval",
			callID:           call.ID,
			tool:             call.Tool,
			parameters:       cloneMap(call.Parameters),
			preview:          cloneMap(preview),
			digest:           digest,
			expiresAt:        expiresAt,
			triggerReason:    "approval_required",
			targetAccessMode: suggestedAccessMode,
			approvalSource:   access.ApprovalSource,
		}
		s.mu.Lock()
		sess.approvals[requestID] = a
		sess.updatedAt = time.Now()
		s.mu.Unlock()

		out["requestID"] = requestID
		out["triggerReason"] = "approval_required"

		s.notifyProtocol(s.newApprovalRequiredEvent(sess, requestID, call, preview, risk, digest, ttl))
		s.notifySessionUpdated(sess)
	}

	s.reply(req.ID, out, nil)
}

func (s *Server) handlePromptResolve(req rpcRequest) {
	var p promptResolveParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	s.resolvePrompt(req, p, "")
}

func (s *Server) handleApprovalResolve(req rpcRequest) {
	var p promptResolveParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	s.resolvePrompt(req, p, "approval")
}

func (s *Server) handleInquiryResolve(req rpcRequest) {
	var p promptResolveParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	if strings.TrimSpace(p.Decision) == "" {
		p.Decision = "resolve"
	}
	s.resolvePrompt(req, p, "inquiry")
}

func (s *Server) resolvePrompt(req rpcRequest, p promptResolveParams, expectedKind string) {
	sess := s.getSession(strings.TrimSpace(p.SessionID))
	if sess == nil {
		s.reply(req.ID, nil, &rpcError{Code: -32002, Message: "SessionNotFound"})
		return
	}
	requestID := strings.TrimSpace(firstNonEmpty(p.RequestID, p.ApprovalID, p.InquiryID))
	if requestID == "" {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}

	s.mu.Lock()
	a := sess.approvals[requestID]
	s.mu.Unlock()
	if a == nil {
		s.reply(req.ID, nil, &rpcError{Code: -32006, Message: "ConfirmationRequired", Data: map[string]any{"requestID": requestID}})
		return
	}
	if expectedKind != "" && !strings.EqualFold(strings.TrimSpace(a.kind), expectedKind) {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams", Data: map[string]any{"requestID": requestID}})
		return
	}
	if time.Now().After(a.expiresAt) {
		s.reply(req.ID, nil, &rpcError{Code: -32007, Message: "ConfirmationExpired", Data: map[string]any{"requestID": requestID}})
		return
	}
	requireDigest := !strings.EqualFold(strings.TrimSpace(a.kind), "inquiry")
	if requireDigest && (strings.TrimSpace(p.ApprovalDigest) == "" || strings.TrimSpace(p.ApprovalDigest) != a.digest) {
		s.reply(req.ID, nil, &rpcError{Code: -32008, Message: "ConfirmationDigestMismatch", Data: map[string]any{"requestID": requestID}})
		return
	}

	decision := strings.ToLower(strings.TrimSpace(p.Decision))
	if decision != "deny" && decision != "allow_once" && decision != "allow_session" && decision != "resolve" {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams", Data: map[string]any{"field": "decision"}})
		return
	}

	s.mu.Lock()
	a.decision = decision
	a.policyID = strings.TrimSpace(p.PolicyID)
	a.reason = strings.TrimSpace(p.Reason)
	a.option = p.Option
	a.text = p.Text
	if decision == "allow_session" && strings.TrimSpace(a.digest) != "" {
		sess.allowSession[a.digest] = time.Now().Add(10 * time.Minute)
	}
	if preview := sessionPreviewFromResolution(*a); preview != "" {
		sess.preview = preview
	}
	sess.lastAuthorization = &authorizationStatus{
		Decision:         decision,
		Category:         strings.TrimSpace(a.kind),
		Tool:             strings.TrimSpace(a.tool),
		Summary:          normalizeSessionPreview(sessionPreviewFromApproval(a)),
		Reason:           strings.TrimSpace(a.reason),
		TargetAccessMode: strings.TrimSpace(a.targetAccessMode),
		At:               time.Now(),
	}
	aCopy := *a
	sess.updatedAt = time.Now()
	s.mu.Unlock()

	s.notifyProtocol(s.newPromptResolvedEvent(sess, aCopy))
	s.notifySessionUpdated(sess)
	s.reply(req.ID, map[string]any{"ok": true}, nil)
}

func (s *Server) handleToolExecute(ctx context.Context, req rpcRequest) {
	var p toolExecuteParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	s.handleCallExecute(ctx, req, p.SessionID, p.Call)
}

func (s *Server) handleCallExecute(ctx context.Context, req rpcRequest, sessionID string, call toolCallDTO) {
	sess := s.getSession(strings.TrimSpace(sessionID))
	if sess == nil {
		s.reply(req.ID, nil, &rpcError{Code: -32002, Message: "SessionNotFound"})
		return
	}

	call.Tool = strings.TrimSpace(call.Tool)
	if call.ID == "" || call.Tool == "" {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	var err error
	call.Parameters, err = normalizePathsInParams(sess.workspaceAbs, call.Parameters)
	if err != nil {
		s.reply(req.ID, nil, errToRPC(err))
		return
	}
	s.mu.Lock()
	if r, ok := sess.results[call.ID]; ok {
		s.mu.Unlock()
		s.reply(req.ID, r, nil)
		return
	}
	if _, ok := sess.runningCancels[call.ID]; ok {
		s.mu.Unlock()
		s.reply(req.ID, nil, &rpcError{Code: -32010, Message: "Conflict"})
		return
	}
	if sess.maxConcurrentToolCalls > 0 && len(sess.runningCancels) >= sess.maxConcurrentToolCalls {
		max := sess.maxConcurrentToolCalls
		running := len(sess.runningCancels)
		s.mu.Unlock()
		s.reply(req.ID, nil, &rpcError{Code: -32010, Message: "TooManyConcurrentCalls", Data: map[string]any{"max": max, "running": running}})
		return
	}
	execCtx, cancel := context.WithCancel(ctx)
	sess.runningCancels[call.ID] = cancel
	if preview := sessionPreviewForCall(call); preview != "" {
		sess.preview = preview
	}
	sess.updatedAt = time.Now()
	s.mu.Unlock()

	s.notifyProtocol(s.newRequestEvent(sess, protocol.EventTypeRequestStarted, call.ID, call, map[string]any{
		"status": "running",
	}))
	s.notifySessionUpdated(sess)

	respID := append(json.RawMessage(nil), req.ID...)
	callCopy := call
	callCopy.Parameters = cloneMap(callCopy.Parameters)
	go func() {
		result, rpcErr := s.executeTool(execCtx, sess, callCopy)
		s.mu.Lock()
		delete(sess.runningCancels, call.ID)
		if rpcErr == nil {
			sess.results[call.ID] = result
			if preview := sessionPreviewFromResult(result); preview != "" {
				sess.preview = preview
			}
		} else if rpcErr.Code != -32006 && rpcErr.Code != -32009 {
			if preview := normalizeSessionPreview(rpcErr.Message); preview != "" {
				sess.preview = preview
			}
		}
		sess.updatedAt = time.Now()
		s.mu.Unlock()
		if rpcErr != nil {
			s.enrichRequestError(sess, rpcErr)
			if rpcErr.Code != -32006 && rpcErr.Code != -32009 {
				s.notifyProtocol(s.newRequestEvent(sess, protocol.EventTypeRequestFailed, call.ID, callCopy, map[string]any{
					"status":  "failed",
					"error":   rpcErr.Message,
					"code":    rpcErr.Code,
					"summary": normalizeSessionPreview(rpcErr.Message),
				}))
			}
			s.notifySessionUpdated(sess)
			s.reply(respID, nil, rpcErr)
			return
		}
		if errText, failed := requestFailureFromResult(result); failed {
			s.mu.Lock()
			if preview := normalizeSessionPreview(errText); preview != "" {
				sess.preview = preview
			}
			sess.updatedAt = time.Now()
			s.mu.Unlock()
			s.notifyProtocol(s.newRequestEvent(sess, protocol.EventTypeRequestFailed, call.ID, callCopy, map[string]any{
				"status":  "failed",
				"error":   errText,
				"result":  result,
				"summary": normalizeSessionPreview(errText),
			}))
			s.notifySessionUpdated(sess)
			s.reply(respID, result, nil)
			return
		}
		s.notifyProtocol(s.newRequestEvent(sess, protocol.EventTypeRequestDone, call.ID, callCopy, map[string]any{
			"status":  "success",
			"result":  result,
			"summary": sessionPreviewFromResult(result),
		}))
		s.notifySessionUpdated(sess)
		s.reply(respID, result, nil)
	}()
}

func (s *Server) executeTool(ctx context.Context, sess *session, call toolCallDTO) (any, *rpcError) {
	defs := s.currentToolDefinitions(sess.workspaceAbs)
	def, ok := toolapi.FindToolDefinition(defs, call.Tool)
	if !ok {
		return nil, &rpcError{Code: -32003, Message: "ToolNotAllowed"}
	}
	access := toolapi.EvaluateToolAccess(def, sessionExecSession(sess))
	suggestedAccessMode := suggestedUpgradeAccessMode(access)
	if !access.Visible || !access.Executable {
		return nil, &rpcError{Code: -32003, Message: "ToolNotAllowed", Data: map[string]any{
			"reason":              access.Reason,
			"mode":                access.Mode,
			"riskLevel":           access.RiskLevel,
			"accessMode":          access.AccessMode,
			"approvalMode":        access.ApprovalMode,
			"approvalSource":      access.ApprovalSource,
			"suggestedAccessMode": suggestedAccessMode,
		}}
	}
	risk := string(access.RiskLevel)
	effectiveAccessMode := access.AccessMode
	if escalated := s.consumeApprovedEscalationAccessMode(sess, call); escalated != "" {
		effectiveAccessMode = escalated
	}
	preview, err := s.buildPreviewForAccess(sess, effectiveAccessMode, call)
	if err != nil {
		rpcErr := errToRPC(err)
		rpcErr.Data = mergeAnyMap(asMap(rpcErr.Data), preflightContextData(sess, access, map[string]any{
			"triggerReason":       preflightTriggerReasonFromError(rpcErr),
			"suggestedAccessMode": suggestedAccessModeForError(access, rpcErr),
		}))
		if shouldEscalateOnFailure(access, rpcErr.Message) {
			requestID := s.ensurePendingApproval(sess, call, preview, "", risk, preflightTriggerReasonFromError(rpcErr), suggestedAccessModeForError(access, rpcErr), access.ApprovalSource)
			rpcErr = &rpcError{
				Code:    -32006,
				Message: "ConfirmationRequired",
				Data: mergeAnyMap(asMap(rpcErr.Data), map[string]any{
					"requestID":           requestID,
					"triggerReason":       preflightTriggerReasonFromError(rpcErr),
					"suggestedAccessMode": suggestedAccessModeForError(access, rpcErr),
				}),
			}
		}
		return nil, rpcErr
	}

	payload := map[string]any{
		"sessionID":     sess.id,
		"relatedCallID": call.ID,
		"tool":          call.Tool,
		"parameters":    call.Parameters,
		"preview":       preview,
	}
	digest, _, err := approvalDigest(payload)
	if err != nil {
		return nil, &rpcError{Code: -32012, Message: "Internal"}
	}

	if access.NeedsApproval {
		if !s.isApproved(sess, call, digest) {
			requestID := s.ensurePendingApproval(sess, call, preview, digest, risk, "approval_required", suggestedAccessMode, access.ApprovalSource)
			return nil, &rpcError{Code: -32006, Message: "ConfirmationRequired", Data: map[string]any{
				"requestID":           requestID,
				"approvalDigest":      digest,
				"triggerReason":       "approval_required",
				"suggestedAccessMode": suggestedAccessMode,
				"approvalSource":      access.ApprovalSource,
			}}
		}
	}

	if call.Tool == "ask_user_question" {
		if a, ok := s.getResolvedInquiry(sess, call, digest); ok {
			s.notifyProtocol(s.newToolCallEvent(sess, call))
			res := toolapi.ToolResult{
				ID:     call.ID,
				Type:   "tool_result",
				Tool:   call.Tool,
				Status: "success",
				Data: map[string]any{
					"option": a.option,
					"text":   a.text,
				},
				Display: "User answered",
				Ts:      time.Now().Unix(),
			}
			s.notifyProtocol(s.newToolResultEvent(sess, call.ID, res))
			return res, nil
		}
		requestID := s.ensurePendingInquiry(sess, call, digest)
		return nil, &rpcError{Code: -32009, Message: "InquiryRequired", Data: map[string]any{"requestID": requestID}}
	}

	s.notifyProtocol(s.newToolCallEvent(sess, call))
	res, err := sess.exec.Execute(ctx, toolapi.ExecSession{
		WorkspaceRoot:         sess.workspaceAbs,
		AllowedTools:          sess.allowedTools,
		TraceID:               call.ID,
		ExecutionMode:         sess.executionMode,
		AccessMode:            effectiveAccessMode,
		ApprovalMode:          sess.approvalMode,
		SandboxMode:           toolapi.SandboxModeFromAccessMode(effectiveAccessMode),
		RequireApprovalDigest: sess.requireApprovalDigest,
	}, []toolapi.ToolCall{{
		ID:     call.ID,
		Name:   call.Tool,
		Params: call.Parameters,
	}})
	if err != nil {
		return nil, &rpcError{Code: -32012, Message: "Internal"}
	}
	if len(res) == 0 {
		return nil, &rpcError{Code: -32012, Message: "Internal"}
	}
	if errText, failed := requestFailureFromResult(res[0]); failed && shouldEscalateOnFailure(access, errText) {
		requestID := s.ensurePendingApproval(sess, call, preview, digest, risk, "sandbox_failure", suggestedAccessMode, access.ApprovalSource)
		return nil, &rpcError{Code: -32006, Message: "ConfirmationRequired", Data: preflightContextData(sess, access, map[string]any{
			"requestID":           requestID,
			"approvalDigest":      digest,
			"triggerReason":       "sandbox_failure",
			"suggestedAccessMode": suggestedAccessMode,
			"failureReason":       errText,
		})}
	}

	s.notifyProtocol(s.newToolResultEvent(sess, call.ID, res[0]))

	return res[0], nil
}

func (s *Server) isApproved(sess *session, call toolCallDTO, digest string) bool {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if until, ok := sess.allowSession[digest]; ok && now.Before(until) {
		return true
	}
	for _, a := range sess.approvals {
		if a.callID != call.ID {
			continue
		}
		if a.digest != digest {
			continue
		}
		if now.After(a.expiresAt) {
			continue
		}
		if a.decision == "allow_once" && !a.used {
			a.used = true
			return true
		}
		if a.decision == "allow_session" {
			return true
		}
	}
	return false
}

func (s *Server) ensurePendingApproval(sess *session, call toolCallDTO, preview map[string]any, digest string, risk string, triggerReason string, targetAccessMode string, approvalSource string) string {
	ttl := int64(60)
	if risk == "high" {
		ttl = 30
	}
	expiresAt := time.Now().Add(time.Duration(ttl) * time.Second)
	requestID := "r_" + uuid.New().String()[:12]

	s.mu.Lock()
	sess.approvals[requestID] = &approval{
		requestID:        requestID,
		kind:             "approval",
		callID:           call.ID,
		tool:             call.Tool,
		parameters:       cloneMap(call.Parameters),
		preview:          cloneMap(preview),
		digest:           digest,
		expiresAt:        expiresAt,
		triggerReason:    strings.TrimSpace(triggerReason),
		targetAccessMode: strings.TrimSpace(targetAccessMode),
		approvalSource:   strings.TrimSpace(approvalSource),
	}
	if previewText := normalizeSessionPreview(s.confirmSummary(call, preview)); previewText != "" {
		sess.preview = previewText
	}
	sess.updatedAt = time.Now()
	s.mu.Unlock()

	s.notifyProtocol(s.newApprovalRequiredEvent(sess, requestID, call, preview, risk, digest, ttl))
	s.notifySessionUpdated(sess)
	return requestID
}

func (s *Server) consumeApprovedEscalationAccessMode(sess *session, call toolCallDTO) string {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range sess.approvals {
		if item == nil || item.callID != call.ID || strings.TrimSpace(item.targetAccessMode) == "" {
			continue
		}
		if strings.TrimSpace(item.digest) != "" {
			continue
		}
		if now.After(item.expiresAt) {
			continue
		}
		switch item.decision {
		case "allow_session":
			return toolapi.NormalizeAccessMode(item.targetAccessMode)
		case "allow_once":
			if item.used {
				continue
			}
			item.used = true
			return toolapi.NormalizeAccessMode(item.targetAccessMode)
		}
	}
	return ""
}

func (s *Server) getResolvedInquiry(sess *session, call toolCallDTO, digest string) (*approval, bool) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range sess.approvals {
		if a.callID != call.ID {
			continue
		}
		if a.digest != digest {
			continue
		}
		if now.After(a.expiresAt) {
			continue
		}
		if a.decision == "resolve" && !a.used {
			a.used = true
			return a, true
		}
	}
	return nil, false
}

func (s *Server) ensurePendingInquiry(sess *session, call toolCallDTO, digest string) string {
	ttl := int64(3600) // 1 hour
	expiresAt := time.Now().Add(time.Duration(ttl) * time.Second)
	requestID := "i_" + uuid.New().String()[:12]

	var question string
	if q, ok := call.Parameters["question"].(string); ok {
		question = q
	}

	s.mu.Lock()
	sess.approvals[requestID] = &approval{
		requestID:  requestID,
		kind:       "inquiry",
		callID:     call.ID,
		tool:       call.Tool,
		parameters: cloneMap(call.Parameters),
		digest:     digest,
		expiresAt:  expiresAt,
	}
	if previewText := normalizeSessionPreview(question); previewText != "" {
		sess.preview = previewText
	}
	sess.updatedAt = time.Now()
	s.mu.Unlock()

	var options []string
	if opts, ok := call.Parameters["options"].([]interface{}); ok {
		for _, opt := range opts {
			if str, ok := opt.(string); ok {
				options = append(options, str)
			}
		}
	} else if opts, ok := call.Parameters["options"].([]string); ok {
		options = opts
	}

	s.notifyProtocol(s.newInquiryRequiredEvent(sess, requestID, call, question, options, digest, ttl))
	s.notifySessionUpdated(sess)
	return requestID
}

func (s *Server) handleToolCancel(req rpcRequest) {
	var p toolCancelParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	s.handleCallCancel(req, p.SessionID, p.CallID)
}

func (s *Server) handleCallCancel(req rpcRequest, sessionID, callID string) {
	sess := s.getSession(strings.TrimSpace(sessionID))
	if sess == nil {
		s.reply(req.ID, nil, &rpcError{Code: -32002, Message: "SessionNotFound"})
		return
	}
	callID = strings.TrimSpace(callID)
	if callID == "" {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	s.mu.Lock()
	cancel := sess.runningCancels[callID]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.reply(req.ID, map[string]any{"ok": cancel != nil}, nil)
}

func (s *Server) handleTaskList(req rpcRequest) {
	var p taskListParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	if s.getSession(strings.TrimSpace(p.SessionID)) == nil {
		s.reply(req.ID, nil, &rpcError{Code: -32002, Message: "SessionNotFound"})
		return
	}
	items, err := s.tools.Tasks().List(context.Background())
	if err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32012, Message: "Internal"})
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"id":        it.ID,
			"kind":      it.Kind,
			"status":    it.Status,
			"startedAt": unixOrZero(it.StartedAt),
			"updatedAt": unixOrZero(it.UpdatedAt),
			"endedAt":   unixOrZero(it.EndedAt),
			"label":     it.Label,
			"summary":   it.Summary,
			"canKill":   it.CanKill,
			"canResume": it.CanResume,
			"canClose":  it.CanClose,
			"metadata":  it.Metadata,
		})
	}
	s.reply(req.ID, map[string]any{"tasks": out}, nil)
}

func unixOrZero(ts time.Time) int64 {
	if ts.IsZero() {
		return 0
	}
	return ts.Unix()
}

func (s *Server) handleTaskCancel(req rpcRequest) {
	s.handleTaskKill(req)
}

func (s *Server) handleTaskResume(req rpcRequest) {
	s.handleTaskAction(req, func(taskID string) error {
		return s.tools.Tasks().Resume(context.Background(), taskID)
	})
}

func (s *Server) handleTaskKill(req rpcRequest) {
	var p taskKillParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	if s.getSession(strings.TrimSpace(p.SessionID)) == nil {
		s.reply(req.ID, nil, &rpcError{Code: -32002, Message: "SessionNotFound"})
		return
	}
	taskID := strings.TrimSpace(p.TaskID)
	if taskID == "" {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	err := s.tools.Tasks().Kill(context.Background(), taskID)
	if err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32012, Message: "Internal"})
		return
	}
	s.reply(req.ID, map[string]any{"ok": true}, nil)
}

func (s *Server) handleTaskClose(req rpcRequest) {
	s.handleTaskAction(req, func(taskID string) error {
		return s.tools.Tasks().Close(context.Background(), taskID)
	})
}

func (s *Server) handleTaskAction(req rpcRequest, action func(taskID string) error) {
	var p taskResumeParams
	if err := decodeParams(req.Params, &p); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	if s.getSession(strings.TrimSpace(p.SessionID)) == nil {
		s.reply(req.ID, nil, &rpcError{Code: -32002, Message: "SessionNotFound"})
		return
	}
	taskID := strings.TrimSpace(p.TaskID)
	if taskID == "" {
		s.reply(req.ID, nil, &rpcError{Code: -32005, Message: "InvalidParams"})
		return
	}
	if err := action(taskID); err != nil {
		s.reply(req.ID, nil, &rpcError{Code: -32012, Message: "Internal", Data: map[string]any{"taskID": taskID}})
		return
	}
	s.reply(req.ID, map[string]any{"ok": true}, nil)
}

func (s *Server) removeSession(id string) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.sessions[strings.TrimSpace(id)]
	if sess == nil {
		return nil
	}
	for _, cancel := range sess.runningCancels {
		cancel()
	}
	delete(s.sessions, sess.id)
	return sess
}

func (s *Server) getSession(id string) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[id]
}

func (s *Server) currentToolDefinitions(workspaceRoot string) []toolapi.ToolDefinition {
	if s == nil {
		return nil
	}
	if s.catalog == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		return append([]toolapi.ToolDefinition(nil), s.toolDefs...)
	}
	ctx := context.Background()
	if strings.TrimSpace(workspaceRoot) != "" {
		ctx = tools.WithWorkspaceRoot(ctx, workspaceRoot)
	}
	defs, err := s.catalog.List(ctx)
	if err != nil || len(defs) == 0 {
		s.mu.Lock()
		defer s.mu.Unlock()
		return append([]toolapi.ToolDefinition(nil), s.toolDefs...)
	}
	s.mu.Lock()
	s.toolDefs = append([]toolapi.ToolDefinition(nil), defs...)
	s.mu.Unlock()
	return defs
}

func (s *Server) reply(id json.RawMessage, result any, err *rpcError) {
	if len(id) == 0 {
		return
	}
	resp := rpcResponse{JSONRPC: "2.0", ID: id, Result: result, Error: err}
	b, _ := json.Marshal(resp)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, _ = s.out.Write(append(b, '\n'))
}

func (s *Server) notifyProtocol(ev protocol.Envelope) {
	nt := rpcNotification{JSONRPC: "2.0", Method: "event", Params: ev}
	b, _ := json.Marshal(nt)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, _ = s.out.Write(append(b, '\n'))
}

func requestFailureFromResult(result any) (string, bool) {
	switch v := result.(type) {
	case toolapi.ToolResult:
		status := strings.ToLower(strings.TrimSpace(v.Status))
		if status == "" || status == "success" {
			return "", false
		}
		if text := strings.TrimSpace(v.Error); text != "" {
			return text, true
		}
		if text := strings.TrimSpace(v.Display); text != "" {
			return text, true
		}
		return v.Status, true
	case map[string]any:
		status := strings.ToLower(strings.TrimSpace(anyString(v["status"])))
		if status == "" || status == "success" {
			return "", false
		}
		if text := strings.TrimSpace(anyString(v["error"])); text != "" {
			return text, true
		}
		if text := strings.TrimSpace(anyString(v["display"])); text != "" {
			return text, true
		}
		return status, true
	default:
		return "", false
	}
}

func anyString(v any) string {
	text, _ := v.(string)
	return text
}

func (s *Server) newRequestEvent(sess *session, eventType protocol.EventType, requestID string, call toolCallDTO, extra map[string]any) protocol.Envelope {
	payload := map[string]any{
		"request_id":    strings.TrimSpace(requestID),
		"tool":          strings.TrimSpace(call.Tool),
		"mode":          toolapi.NormalizeExecutionMode(sess.executionMode),
		"access_mode":   toolapi.ResolveAccessMode(sessionExecSession(sess)),
		"approval_mode": toolapi.ResolveApprovalMode(sessionExecSession(sess)),
		"input_kind":    "tool",
		"call": map[string]any{
			"id":         strings.TrimSpace(call.ID),
			"tool":       strings.TrimSpace(call.Tool),
			"parameters": cloneMap(call.Parameters),
		},
	}
	maps.Copy(payload, extra)
	return protocol.NewEvent(eventType, protocol.EventOptions{
		SessionID:     sessionIDOf(sess),
		ThreadID:      sessionIDOf(sess),
		RequestID:     strings.TrimSpace(requestID),
		CorrelationID: strings.TrimSpace(call.ID),
		Source:        protocol.SourceServe,
		Payload:       payload,
	})
}

func (s *Server) newToolCallEvent(sess *session, call toolCallDTO) protocol.Envelope {
	payload := protocol.ToolCallPayload(protocol.ToolCall{
		ToolName:  call.Tool,
		Arguments: cloneMap(call.Parameters),
	})
	payload["id"] = call.ID
	return protocol.NewEvent(protocol.EventTypeToolCall, protocol.EventOptions{
		SessionID:     sessionIDOf(sess),
		ThreadID:      sessionIDOf(sess),
		RequestID:     strings.TrimSpace(call.ID),
		CorrelationID: strings.TrimSpace(call.ID),
		Source:        protocol.SourceServe,
		Payload:       payload,
	})
}

func (s *Server) newToolResultEvent(sess *session, correlationID string, res toolapi.ToolResult) protocol.Envelope {
	payload := protocol.ToolResultPayload(protocol.ToolResult{
		ToolName: res.Tool,
		Status:   res.Status,
		Display:  res.Display,
		Data:     cloneMap(res.Data),
	})
	payload["id"] = res.ID
	if res.Type != "" {
		payload["result_type"] = res.Type
	}
	if res.Error != "" {
		payload["error"] = res.Error
	}
	if res.Ts != 0 {
		payload["ts"] = res.Ts
	}
	return protocol.NewEvent(protocol.EventTypeToolResult, protocol.EventOptions{
		SessionID:     sessionIDOf(sess),
		ThreadID:      sessionIDOf(sess),
		RequestID:     strings.TrimSpace(res.ID),
		CorrelationID: strings.TrimSpace(correlationID),
		Source:        protocol.SourceServe,
		Payload:       payload,
	})
}

func (s *Server) newApprovalRequiredEvent(sess *session, requestID string, call toolCallDTO, preview map[string]any, risk, digest string, ttl int64) protocol.Envelope {
	payload := protocol.ApprovalRequestPayload(protocol.ApprovalRequest{
		ApprovalID: strings.TrimSpace(requestID),
		Title:      "Execution confirmation",
		Message:    s.confirmSummary(call, preview),
		RiskLevel:  strings.TrimSpace(risk),
		Options:    []string{"allow_once", "allow_session", "deny"},
	})
	payload["related_call_id"] = call.ID
	payload["approval_digest"] = digest
	payload["ttl_seconds"] = ttl
	payload["call"] = call
	payload["preview"] = cloneMap(preview)
	return protocol.NewEvent(protocol.EventTypeApprovalReq, protocol.EventOptions{
		SessionID:     sessionIDOf(sess),
		ThreadID:      sessionIDOf(sess),
		RequestID:     strings.TrimSpace(requestID),
		CorrelationID: strings.TrimSpace(call.ID),
		Source:        protocol.SourceServe,
		Payload:       payload,
	})
}

func (s *Server) newInquiryRequiredEvent(sess *session, requestID string, call toolCallDTO, question string, options []string, digest string, ttl int64) protocol.Envelope {
	payload := protocol.InquiryRequestPayload(protocol.InquiryRequest{
		InquiryID: strings.TrimSpace(requestID),
		Question:  strings.TrimSpace(question),
		Options:   append([]string(nil), options...),
		AllowText: true,
	})
	payload["related_call_id"] = call.ID
	payload["approval_digest"] = digest
	payload["ttl_seconds"] = ttl
	payload["call"] = call
	return protocol.NewEvent(protocol.EventTypeInquiryReq, protocol.EventOptions{
		SessionID:     sessionIDOf(sess),
		ThreadID:      sessionIDOf(sess),
		RequestID:     strings.TrimSpace(requestID),
		CorrelationID: strings.TrimSpace(call.ID),
		Source:        protocol.SourceServe,
		Payload:       payload,
	})
}

func (s *Server) newPromptResolvedEvent(sess *session, item approval) protocol.Envelope {
	if strings.EqualFold(strings.TrimSpace(item.kind), "inquiry") {
		payload := protocol.InquiryResolutionPayload(protocol.InquiryResolution{
			InquiryID: strings.TrimSpace(item.requestID),
			Option:    strings.TrimSpace(item.option),
			Text:      strings.TrimSpace(item.text),
		})
		if strings.TrimSpace(item.callID) != "" {
			payload["related_call_id"] = strings.TrimSpace(item.callID)
		}
		return protocol.NewEvent(protocol.EventTypeInquiryDone, protocol.EventOptions{
			SessionID:     sessionIDOf(sess),
			ThreadID:      sessionIDOf(sess),
			RequestID:     strings.TrimSpace(item.requestID),
			CorrelationID: strings.TrimSpace(item.callID),
			Source:        protocol.SourceServe,
			Payload:       payload,
		})
	}

	payload := protocol.ApprovalResolutionPayload(protocol.ApprovalResolution{
		ApprovalID: strings.TrimSpace(item.requestID),
		Decision:   strings.TrimSpace(item.decision),
		Reason:     strings.TrimSpace(item.reason),
	})
	if strings.TrimSpace(item.callID) != "" {
		payload["related_call_id"] = strings.TrimSpace(item.callID)
	}
	if strings.TrimSpace(item.policyID) != "" {
		payload["policy_id"] = strings.TrimSpace(item.policyID)
	}
	return protocol.NewEvent(protocol.EventTypeApprovalDone, protocol.EventOptions{
		SessionID:     sessionIDOf(sess),
		ThreadID:      sessionIDOf(sess),
		RequestID:     strings.TrimSpace(item.requestID),
		CorrelationID: strings.TrimSpace(item.callID),
		Source:        protocol.SourceServe,
		Payload:       payload,
	})
}

func (s *Server) notifySessionUpdated(sess *session) {
	if sess == nil {
		return
	}
	s.notifyProtocol(s.newSessionUpdatedEvent(sess))
	if err := s.persistSessions(); err != nil {
		s.writeStderr("persist sessions: " + err.Error())
	}
}

func (s *Server) newSessionUpdatedEvent(sess *session) protocol.Envelope {
	info := s.sessionInfo(sess)
	return protocol.NewEvent(protocol.EventTypeSessionUpdated, protocol.EventOptions{
		SessionID:     info.SessionID,
		ThreadID:      info.ThreadID,
		CorrelationID: info.CurrentRequestID,
		Timestamp:     info.UpdatedAt,
		Source:        protocol.SourceServe,
		Payload:       protocol.SessionPayload(info),
	})
}

func (s *Server) sessionInfo(sess *session) protocol.SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionInfoLocked(sess)
}

func (s *Server) sessionInfoLocked(sess *session) protocol.SessionInfo {
	if sess == nil {
		return protocol.SessionInfo{}
	}

	now := time.Now()
	pendingApprovals := make([]string, 0, len(sess.approvals))
	pendingInquiries := make([]string, 0, len(sess.approvals))
	for id, item := range sess.approvals {
		if item == nil || now.After(item.expiresAt) || strings.TrimSpace(item.decision) != "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.kind), "inquiry") {
			pendingInquiries = append(pendingInquiries, id)
			continue
		}
		pendingApprovals = append(pendingApprovals, id)
	}
	sort.Strings(pendingApprovals)
	sort.Strings(pendingInquiries)

	runningTasks := make([]string, 0, len(sess.runningCancels))
	for id := range sess.runningCancels {
		runningTasks = append(runningTasks, id)
	}
	sort.Strings(runningTasks)

	status := "idle"
	currentRequestID := ""
	switch {
	case len(runningTasks) > 0:
		status = "running"
		currentRequestID = runningTasks[0]
	case len(pendingApprovals) > 0:
		status = "waiting_input"
		currentRequestID = pendingApprovals[0]
	case len(pendingInquiries) > 0:
		status = "waiting_input"
		currentRequestID = pendingInquiries[0]
	}

	updatedAt := sess.updatedAt
	if updatedAt.IsZero() {
		updatedAt = now
	}
	preview := normalizeSessionPreview(sess.preview)
	if preview == "" {
		for _, id := range pendingApprovals {
			if item := sess.approvals[id]; item != nil {
				preview = sessionPreviewFromApproval(item)
				if preview != "" {
					break
				}
			}
		}
	}
	if preview == "" {
		for _, id := range pendingInquiries {
			if item := sess.approvals[id]; item != nil {
				preview = sessionPreviewFromApproval(item)
				if preview != "" {
					break
				}
			}
		}
	}

	return protocol.SessionInfo{
		SessionID:        strings.TrimSpace(sess.id),
		ThreadID:         strings.TrimSpace(sess.id),
		Workspace:        strings.TrimSpace(sess.workspaceAbs),
		Title:            strings.TrimSpace(sess.title),
		Preview:          preview,
		Mode:             strings.TrimSpace(sess.executionMode),
		Status:           status,
		CurrentRequestID: currentRequestID,
		PendingApprovals: pendingApprovals,
		PendingInquiries: pendingInquiries,
		RunningTasks:     runningTasks,
		UpdatedAt:        updatedAt,
		Metadata: map[string]any{
			"trusted_workspace":         sess.trustedWorkspace,
			"require_approval_digest":   sess.requireApprovalDigest,
			"max_concurrent_tool_calls": sess.maxConcurrentToolCalls,
			"access_mode":               toolapi.ResolveAccessMode(sessionExecSession(sess)),
			"approval_mode":             toolapi.ResolveApprovalMode(sessionExecSession(sess)),
			"sandbox_mode":              strings.TrimSpace(sess.sandboxMode),
			"last_authorization":        sessionLastAuthorization(sess),
		},
	}
}

func sessionIDOf(sess *session) string {
	if sess == nil {
		return ""
	}
	return strings.TrimSpace(sess.id)
}

func sessionLastAuthorization(sess *session) map[string]any {
	if sess == nil || sess.lastAuthorization == nil || sess.lastAuthorization.At.IsZero() {
		return nil
	}
	item := sess.lastAuthorization
	return map[string]any{
		"decision":           strings.TrimSpace(item.Decision),
		"category":           strings.TrimSpace(item.Category),
		"tool":               strings.TrimSpace(item.Tool),
		"summary":            strings.TrimSpace(item.Summary),
		"reason":             strings.TrimSpace(item.Reason),
		"target_access_mode": strings.TrimSpace(item.TargetAccessMode),
		"at":                 item.At.Format(time.RFC3339),
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

func defaultSessionTitle(workspaceAbs string) string {
	title := filepath.Base(strings.TrimSpace(workspaceAbs))
	if title == "." {
		return ""
	}
	return normalizeSessionPreview(title)
}

func sessionPreviewForCall(call toolCallDTO) string {
	toolName := strings.TrimSpace(call.Tool)
	switch strings.ToLower(toolName) {
	case "bash":
		if command := normalizeSessionPreview(anyString(call.Parameters["command"])); command != "" {
			return "bash: " + command
		}
	case "ask_user_question":
		if question := normalizeSessionPreview(anyString(call.Parameters["question"])); question != "" {
			return question
		}
	}
	for _, key := range []string{"path", "file", "source", "destination"} {
		if value := normalizeSessionPreview(anyString(call.Parameters[key])); value != "" {
			if toolName == "" {
				return value
			}
			return toolName + ": " + value
		}
	}
	if toolName == "" {
		return ""
	}
	return "调用工具: " + toolName
}

func sessionPreviewFromResult(result any) string {
	switch v := result.(type) {
	case toolapi.ToolResult:
		if text := normalizeSessionPreview(firstNonEmpty(v.Display, v.Error, anyString(v.Data["text"]), anyString(v.Data["option"]))); text != "" {
			return text
		}
		if toolName := strings.TrimSpace(v.Tool); toolName != "" {
			return "完成工具: " + toolName
		}
	case map[string]any:
		if text := normalizeSessionPreview(firstNonEmpty(anyString(v["display"]), anyString(v["error"]), anyString(v["text"]), anyString(v["message"]))); text != "" {
			return text
		}
		if toolName := strings.TrimSpace(anyString(v["tool"])); toolName != "" {
			return "完成工具: " + toolName
		}
	}
	return ""
}

func sessionPreviewFromResolution(item approval) string {
	toolName := strings.TrimSpace(item.tool)
	switch strings.ToLower(strings.TrimSpace(item.kind)) {
	case "inquiry":
		if answer := normalizeSessionPreview(firstNonEmpty(item.option, item.text)); answer != "" {
			return "已回答: " + answer
		}
		if toolName != "" {
			return "已回答: " + toolName
		}
	default:
		switch strings.ToLower(strings.TrimSpace(item.decision)) {
		case "allow_once":
			if toolName != "" {
				return "已确认执行: " + toolName
			}
			return "已确认执行"
		case "allow_session":
			if toolName != "" {
				return "本会话已放行: " + toolName
			}
			return "本会话已放行"
		case "deny":
			if toolName != "" {
				return "已拒绝: " + toolName
			}
			return "已拒绝执行"
		}
	}
	return ""
}

func sessionPreviewFromApproval(item *approval) string {
	if item == nil {
		return ""
	}
	if preview := sessionPreviewFromResolution(*item); preview != "" {
		return preview
	}
	if strings.EqualFold(strings.TrimSpace(item.kind), "inquiry") {
		if question := normalizeSessionPreview(anyString(item.parameters["question"])); question != "" {
			return question
		}
	}
	if command := normalizeSessionPreview(anyString(item.preview["command"])); command != "" {
		return "bash: " + command
	}
	if command := normalizeSessionPreview(anyString(item.parameters["command"])); command != "" {
		return "bash: " + command
	}
	if path := normalizeSessionPreview(firstNonEmpty(anyString(item.parameters["path"]), anyString(item.parameters["file"]), anyString(item.parameters["source"]), anyString(item.parameters["destination"]))); path != "" {
		if toolName := strings.TrimSpace(item.tool); toolName != "" {
			return toolName + ": " + path
		}
		return path
	}
	if toolName := strings.TrimSpace(item.tool); toolName != "" {
		return "调用工具: " + toolName
	}
	return ""
}

func normalizeSessionPreview(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) <= 96 {
		return text
	}
	return string(runes[:96]) + "..."
}

func (s *Server) writeStderr(msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, _ = fmt.Fprintln(s.err, msg)
}

func (s *Server) enrichRequestError(sess *session, rpcErr *rpcError) {
	if rpcErr == nil || rpcErr.Code != -32006 {
		return
	}
	data, ok := rpcErr.Data.(map[string]any)
	if !ok {
		return
	}
	requestID, _ := data["requestID"].(string)
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return
	}

	s.mu.Lock()
	a := sess.approvals[requestID]
	s.mu.Unlock()
	if a == nil {
		return
	}
	if _, exists := data["approvalDigest"]; !exists && strings.TrimSpace(a.digest) != "" {
		data["approvalDigest"] = strings.TrimSpace(a.digest)
	}
	if _, exists := data["decisionOptions"]; !exists {
		data["decisionOptions"] = []string{"allow_once", "allow_session", "deny"}
	}
	if _, exists := data["triggerReason"]; !exists && strings.TrimSpace(a.triggerReason) != "" {
		data["triggerReason"] = strings.TrimSpace(a.triggerReason)
	}
	if _, exists := data["suggestedAccessMode"]; !exists && strings.TrimSpace(a.targetAccessMode) != "" {
		data["suggestedAccessMode"] = strings.TrimSpace(a.targetAccessMode)
	}
	if _, exists := data["approvalSource"]; !exists && strings.TrimSpace(a.approvalSource) != "" {
		data["approvalSource"] = strings.TrimSpace(a.approvalSource)
	}
}

func errToRPC(err error) *rpcError {
	var re *rpcError
	if errors.As(err, &re) {
		return re
	}
	return &rpcError{Code: -32012, Message: "Internal"}
}

func (s *Server) buildPreview(sess *session, call toolCallDTO) (map[string]any, error) {
	return s.buildPreviewForAccess(sess, toolapi.ResolveAccessMode(sessionExecSession(sess)), call)
}

func (s *Server) buildPreviewForAccess(sess *session, accessMode string, call toolCallDTO) (map[string]any, error) {
	if err := s.checkWorkspaceConstraints(sess, accessMode, call); err != nil {
		return nil, err
	}
	switch strings.ToLower(call.Tool) {
	case "bash":
		cmd, _ := call.Parameters["command"].(string)
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			return map[string]any{}, nil
		}
		if s.policy != nil {
			rule := s.policy.FindRule("bash", "high")
			if rule != nil {
				allowed := rule.AllowedCommands()
				if len(allowed) > 0 && !containsExact(allowed, cmd) {
					return nil, &rpcError{Code: -32003, Message: "ToolNotAllowed"}
				}
			}
		}
		return map[string]any{"command": cmd, "safetyFindings": []any{}}, nil
	case "edit":
		mode, _ := call.Parameters["mode"].(string)
		file, _ := call.Parameters["file"].(string)
		return map[string]any{"mode": strings.TrimSpace(mode), "file": strings.TrimSpace(file)}, nil
	case "fs":
		mode, _ := call.Parameters["mode"].(string)
		path, _ := call.Parameters["path"].(string)
		src, _ := call.Parameters["source"].(string)
		dst, _ := call.Parameters["destination"].(string)
		return map[string]any{"mode": strings.TrimSpace(mode), "path": strings.TrimSpace(path), "source": strings.TrimSpace(src), "destination": strings.TrimSpace(dst)}, nil
	default:
		return map[string]any{}, nil
	}
}

func (s *Server) checkWorkspaceConstraints(sess *session, accessMode string, call toolCallDTO) error {
	if sess == nil {
		return &rpcError{Code: -32002, Message: "SessionNotFound"}
	}
	if toolapi.NormalizeAccessMode(accessMode) == "danger-full-access" {
		return nil
	}
	if strings.TrimSpace(sess.workspaceAbs) == "" {
		return nil
	}
	for _, field := range []string{"path", "file", "source", "destination", "working_dir", "root"} {
		raw, ok := call.Parameters[field].(string)
		if !ok {
			continue
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		abs, ok, err := resolveInWorkspace(sess.workspaceAbs, raw)
		if err != nil {
			return &rpcError{Code: -32005, Message: "InvalidParams", Data: map[string]any{"field": field}}
		}
		if !ok {
			return &rpcError{Code: -32009, Message: "WorkspaceViolation", Data: map[string]any{"field": field, "path": raw}}
		}
		if s.policy != nil {
			rule := s.policy.FindRule(strings.ToLower(call.Tool), string(s.catalog.RiskLevel(call.Tool)))
			if rule != nil {
				for _, pat := range rule.DenyPathGlobs() {
					if matchDenyGlob(pat, filepath.ToSlash(abs)) {
						return &rpcError{Code: -32003, Message: "ToolNotAllowed", Data: map[string]any{"path": raw}}
					}
				}
			}
		}
	}
	return nil
}

func resolveInWorkspace(workspaceAbs, p string) (string, bool, error) {
	workspaceAbs = filepath.Clean(workspaceAbs)
	if filepath.IsAbs(p) {
		abs := filepath.Clean(p)
		ok := isSubpath(workspaceAbs, abs)
		return abs, ok, nil
	}
	joined := filepath.Join(workspaceAbs, p)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", false, err
	}
	abs = filepath.Clean(abs)
	return abs, isSubpath(workspaceAbs, abs), nil
}

func isSubpath(base, p string) bool {
	base = filepath.Clean(base)
	p = filepath.Clean(p)
	rel, err := filepath.Rel(base, p)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

func matchDenyGlob(pattern, target string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	target = filepath.ToSlash(strings.TrimSpace(target))
	if pattern == "" || target == "" {
		return false
	}
	if strings.Contains(pattern, "**") {
		needle := strings.ReplaceAll(pattern, "**", "")
		needle = strings.Trim(needle, "/")
		return needle != "" && strings.Contains(target, needle)
	}
	ok, _ := filepath.Match(pattern, target)
	return ok
}

func containsExact(items []string, s string) bool {
	for _, it := range items {
		if strings.TrimSpace(it) == s {
			return true
		}
	}
	return false
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	cp := make(map[string]any, len(m))
	maps.Copy(cp, m)
	return cp
}

func asMap(v any) map[string]any {
	out, _ := v.(map[string]any)
	if out == nil {
		return map[string]any{}
	}
	return out
}

func mergeAnyMap(a, b map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func suggestedUpgradeAccessMode(access toolapi.ToolAccess) string {
	if access.AccessMode == "danger-full-access" {
		return ""
	}
	if access.Reason == "sandbox_mode" || access.Reason == "access_mode" {
		return "danger-full-access"
	}
	return ""
}

func suggestedAccessModeForError(access toolapi.ToolAccess, rpcErr *rpcError) string {
	if rpcErr == nil {
		return suggestedUpgradeAccessMode(access)
	}
	if rpcErr.Code == -32009 || strings.EqualFold(strings.TrimSpace(rpcErr.Message), "WorkspaceViolation") {
		return "danger-full-access"
	}
	return suggestedUpgradeAccessMode(access)
}

func shouldEscalateOnFailure(access toolapi.ToolAccess, message string) bool {
	if toolapi.NormalizeApprovalMode(access.ApprovalMode) != "on-failure" {
		return false
	}
	if access.AccessMode == "danger-full-access" {
		return false
	}
	return tools.IsSandboxPolicyError(message) || strings.EqualFold(strings.TrimSpace(message), "WorkspaceViolation")
}

func preflightTriggerReasonFromError(rpcErr *rpcError) string {
	if rpcErr == nil {
		return ""
	}
	if rpcErr.Code == -32009 || strings.EqualFold(strings.TrimSpace(rpcErr.Message), "WorkspaceViolation") {
		return "workspace_violation"
	}
	return "sandbox_failure"
}

func preflightContextData(sess *session, access toolapi.ToolAccess, extra map[string]any) map[string]any {
	out := map[string]any{
		"accessMode":          access.AccessMode,
		"approvalMode":        access.ApprovalMode,
		"approvalSource":      access.ApprovalSource,
		"sandboxMode":         access.SandboxMode,
		"suggestedAccessMode": suggestedUpgradeAccessMode(access),
		"workspaceBoundary": map[string]any{
			"root":     strings.TrimSpace(sess.workspaceAbs),
			"tempDirs": toolsAllowedTempDirs(),
		},
	}
	if extra != nil {
		for k, v := range extra {
			out[k] = v
		}
	}
	return out
}

func toolsAllowedTempDirs() []string {
	dirs := []string{
		os.TempDir(),
		os.Getenv("TMPDIR"),
		os.Getenv("TMP"),
		os.Getenv("TEMP"),
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		abs, err := filepath.Abs(dir)
		if err == nil {
			dir = abs
		}
		dir = filepath.Clean(dir)
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		out = append(out, dir)
	}
	return out
}

func (s *Server) confirmSummary(call toolCallDTO, preview map[string]any) string {
	switch strings.ToLower(call.Tool) {
	case "bash":
		cmd, _ := preview["command"].(string)
		cmd = strings.TrimSpace(cmd)
		if cmd != "" {
			return "即将执行 Shell 命令：" + cmd
		}
	}
	return "即将执行工具：" + strings.TrimSpace(call.Tool)
}

func buildToolDefinitions(defs []toolapi.ToolDefinition, sess *toolapi.ExecSession) []toolDefinitionDTO {
	if len(defs) == 0 {
		return nil
	}
	out := make([]toolDefinitionDTO, 0, len(defs))
	for _, d := range defs {
		params := map[string]parameterInfoDTO{}
		for k, v := range d.Params {
			params[k] = parameterInfoDTO{
				Type:     v.Type,
				Required: v.Required,
				Desc:     v.Desc,
			}
		}
		examples := make([]map[string]any, 0, len(d.Examples))
		for _, ex := range d.Examples {
			examples = append(examples, map[string]any{
				"description": ex.Description,
				"input":       ex.Input,
			})
		}
		item := toolDefinitionDTO{
			Name:               d.Name,
			Description:        d.Description,
			RiskLevel:          string(d.RiskLevel),
			Params:             params,
			Examples:           examples,
			Source:             string(d.Source),
			Category:           d.Category,
			VisibleIn:          append([]string(nil), d.VisibleIn...),
			ReadOnly:           d.ReadOnly,
			Invocable:          d.Invocable,
			RequiresFullAccess: d.RequiresFullAccess,
			Tags:               append([]string(nil), d.Tags...),
			Metadata:           cloneMap(d.Metadata),
		}
		if sess != nil {
			access := toolapi.EvaluateToolAccess(d, *sess)
			item.Access = &toolAccessDTO{
				Mode:           access.Mode,
				AccessMode:     access.AccessMode,
				ApprovalMode:   access.ApprovalMode,
				ApprovalSource: access.ApprovalSource,
				SandboxMode:    toolapi.NormalizeSandboxMode(sess.SandboxMode),
				Visible:        access.Visible,
				Executable:     access.Executable,
				NeedsApproval:  access.NeedsApproval,
				Reason:         access.Reason,
			}
		}
		out = append(out, item)
	}
	return out
}

func defsToDTOsForSession(defs []toolapi.ToolDefinition, sess toolapi.ExecSession) []toolDefinitionDTO {
	return buildToolDefinitions(defs, &sess)
}

func sessionExecSession(sess *session) toolapi.ExecSession {
	if sess == nil {
		return toolapi.ExecSession{
			ExecutionMode: toolapi.NormalizeExecutionMode(""),
			AccessMode:    "",
			ApprovalMode:  "",
			SandboxMode:   toolapi.SandboxModeFromAccessMode(toolapi.NormalizeAccessMode("")),
		}
	}
	return toolapi.ExecSession{
		AllowedTools:          sess.allowedTools,
		ExecutionMode:         toolapi.NormalizeExecutionMode(sess.executionMode),
		AccessMode:            normalizeOptionalAccessMode(sess.accessMode),
		ApprovalMode:          normalizeOptionalApprovalMode(sess.approvalMode),
		SandboxMode:           toolapi.NormalizeSandboxMode(sess.sandboxMode),
		RequireApprovalDigest: sess.requireApprovalDigest,
		WorkspaceRoot:         sess.workspaceAbs,
	}
}

func normalizeOptionalAccessMode(mode string) string {
	if strings.TrimSpace(mode) == "" {
		return ""
	}
	return toolapi.NormalizeAccessMode(mode)
}

func normalizeOptionalApprovalMode(mode string) string {
	if strings.TrimSpace(mode) == "" {
		return ""
	}
	return toolapi.NormalizeApprovalMode(mode)
}

func buildCatalogSummary(items []toolDefinitionDTO) map[string]any {
	summary := map[string]any{
		"total":            len(items),
		"visible":          0,
		"executable":       0,
		"needsApproval":    0,
		"hidden":           0,
		"capabilityOnly":   0,
		"blockedByAccess":  0,
		"blockedByMode":    0,
		"blockedBySandbox": 0,
		"blockedByAllow":   0,
		"blockedByDontAsk": 0,
	}
	for _, item := range items {
		if item.Access == nil {
			continue
		}
		if item.Access.Visible {
			summary["visible"] = summary["visible"].(int) + 1
		} else {
			summary["hidden"] = summary["hidden"].(int) + 1
		}
		if item.Access.Executable {
			summary["executable"] = summary["executable"].(int) + 1
		}
		if item.Access.NeedsApproval {
			summary["needsApproval"] = summary["needsApproval"].(int) + 1
		}
		switch item.Access.Reason {
		case "non_invocable":
			summary["capabilityOnly"] = summary["capabilityOnly"].(int) + 1
		case "access_mode":
			summary["blockedByAccess"] = summary["blockedByAccess"].(int) + 1
		case "execution_mode":
			summary["blockedByMode"] = summary["blockedByMode"].(int) + 1
		case "sandbox_mode":
			summary["blockedBySandbox"] = summary["blockedBySandbox"].(int) + 1
		case "allowed_tools":
			summary["blockedByAllow"] = summary["blockedByAllow"].(int) + 1
		case "dont_ask":
			summary["blockedByDontAsk"] = summary["blockedByDontAsk"].(int) + 1
		}
	}
	return summary
}

func modeDTO(mode string) executionModeDTO {
	desc := toolapi.ExecutionModeDescriptorFor(mode)
	return executionModeDTO{
		Name:             desc.Name,
		Aliases:          append([]string(nil), desc.Aliases...),
		Description:      desc.Description,
		ApprovalBehavior: desc.ApprovalBehavior,
	}
}

func modeDTOs(items []toolapi.ExecutionModeDescriptor) []executionModeDTO {
	out := make([]executionModeDTO, 0, len(items))
	for _, item := range items {
		out = append(out, executionModeDTO{
			Name:             item.Name,
			Aliases:          append([]string(nil), item.Aliases...),
			Description:      item.Description,
			ApprovalBehavior: item.ApprovalBehavior,
		})
	}
	return out
}

func accessModeDTOs(items []toolapi.AccessModeDescriptor) []accessModeDTO {
	out := make([]accessModeDTO, 0, len(items))
	for _, item := range items {
		out = append(out, accessModeDTO{
			Name:        item.Name,
			Aliases:     append([]string(nil), item.Aliases...),
			Description: item.Description,
		})
	}
	return out
}

func approvalModeDTOs(items []toolapi.ApprovalModeDescriptor) []approvalModeDTO {
	out := make([]approvalModeDTO, 0, len(items))
	for _, item := range items {
		out = append(out, approvalModeDTO{
			Name:        item.Name,
			Aliases:     append([]string(nil), item.Aliases...),
			Description: item.Description,
		})
	}
	return out
}
