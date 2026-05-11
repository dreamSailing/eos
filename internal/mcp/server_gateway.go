package mcp

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/toolapi"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

type MountedSSE struct {
	SSEPath     string
	MessagePath string
	server      *mcpserver.SSEServer
}

func (s *Server) NewMountedSSE(baseURL string, basePath string) *MountedSSE {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		basePath = "/mcp"
	}
	sse := mcpserver.NewSSEServer(
		s.mcp,
		mcpserver.WithBaseURL(strings.TrimRight(baseURL, "/")),
		mcpserver.WithStaticBasePath(basePath),
		mcpserver.WithUseFullURLForMessageEndpoint(false),
		mcpserver.WithKeepAlive(true),
		mcpserver.WithKeepAliveInterval(10*time.Second),
		mcpserver.WithSSEContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			return s.contextWithRequestMeta(ctx, r.Header)
		}),
	)
	return &MountedSSE{
		SSEPath:     strings.TrimRight(basePath, "/") + "/sse",
		MessagePath: strings.TrimRight(basePath, "/") + "/message",
		server:      sse,
	}
}

func (m *MountedSSE) Attach(mux *http.ServeMux) {
	if m == nil || mux == nil || m.server == nil {
		return
	}
	mux.Handle(m.SSEPath, m.server.SSEHandler())
	mux.Handle(m.MessagePath, m.server.MessageHandler())
}

func (s *Server) Services() toolapi.Services {
	return s.services
}

func (s *Server) SessionSnapshots() []map[string]any {
	items := s.listSessions()
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, item.snapshot())
	}
	return out
}

func (s *Server) SessionDetail(sessionID string) (map[string]any, bool) {
	sess := s.getSession(sessionID)
	if sess == nil {
		return nil, false
	}
	return map[string]any{
		"session":   sess.snapshot(),
		"approvals": sess.approvalList(),
		"inquiries": sess.inquiryList(),
	}, true
}

func (s *Server) PromptSnapshots() map[string][]map[string]any {
	approvals := make([]map[string]any, 0)
	inquiries := make([]map[string]any, 0)
	for _, sess := range s.listSessions() {
		sessionID := sess.ID
		for _, item := range sess.approvalList() {
			entry := cloneMap(item)
			entry["session_id"] = sessionID
			approvals = append(approvals, entry)
		}
		for _, item := range sess.inquiryList() {
			entry := cloneMap(item)
			entry["session_id"] = sessionID
			inquiries = append(inquiries, entry)
		}
	}
	sort.Slice(approvals, func(i, j int) bool {
		return stringValue(approvals[i]["request_id"]) < stringValue(approvals[j]["request_id"])
	})
	sort.Slice(inquiries, func(i, j int) bool {
		return stringValue(inquiries[i]["request_id"]) < stringValue(inquiries[j]["request_id"])
	})
	return map[string][]map[string]any{
		"approvals": approvals,
		"inquiries": inquiries,
	}
}

func (s *Server) ResolveApproval(requestID, decision, reason, policyID string) (map[string]any, error) {
	requestID = strings.TrimSpace(requestID)
	decision = strings.ToLower(strings.TrimSpace(decision))
	if requestID == "" {
		return nil, fmt.Errorf("request id required")
	}
	if decision != "allow_once" && decision != "allow_session" && decision != "deny" {
		return nil, fmt.Errorf("invalid decision")
	}
	for _, sess := range s.listSessions() {
		sess.mu.Lock()
		item := sess.Approvals[requestID]
		if item == nil {
			sess.mu.Unlock()
			continue
		}
		item.Decision = decision
		item.Reason = strings.TrimSpace(reason)
		item.PolicyID = strings.TrimSpace(policyID)
		item.ResolvedAt = time.Now()
		sess.UpdatedAt = time.Now()
		preview := fmt.Sprintf("审批已处理: %s", decision)
		if decision == "allow_session" {
			preview = "审批已放行: allow_session"
		}
		if text := normalizePreview(preview); text != "" {
			sess.Preview = text
		}
		result := pendingPromptSnapshot(item)
		result["session_id"] = sess.ID
		sess.mu.Unlock()
		return result, nil
	}
	return nil, fmt.Errorf("approval not found")
}

func (s *Server) ResolveInquiry(requestID, option, text string) (map[string]any, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, fmt.Errorf("request id required")
	}
	for _, sess := range s.listSessions() {
		sess.mu.Lock()
		item := sess.Inquiries[requestID]
		if item == nil {
			sess.mu.Unlock()
			continue
		}
		item.Decision = "resolve"
		item.Option = strings.TrimSpace(option)
		item.Text = strings.TrimSpace(text)
		item.ResolvedAt = time.Now()
		sess.UpdatedAt = time.Now()
		if preview := normalizePreview(firstNonEmpty(item.Option, item.Text, item.Question)); preview != "" {
			sess.Preview = preview
		}
		result := pendingPromptSnapshot(item)
		result["session_id"] = sess.ID
		sess.mu.Unlock()
		return result, nil
	}
	return nil, fmt.Errorf("inquiry not found")
}

func (s *Server) StatusSnapshot() map[string]any {
	sessions := s.SessionSnapshots()
	prompts := s.PromptSnapshots()
	accessMode := toolapi.ResolveAccessMode(toolapi.ExecSession{
		AccessMode:  normalizeOptionalAccessMode(s.opts.DefaultAccessMode),
		SandboxMode: s.opts.DefaultSandboxMode,
	})
	approvalMode := toolapi.ResolveApprovalMode(toolapi.ExecSession{
		ApprovalMode:          normalizeOptionalApprovalMode(s.opts.DefaultApprovalMode),
		RequireApprovalDigest: s.opts.RequireApprovalDigest,
	})
	return map[string]any{
		"transport":         s.opts.Transport,
		"workspace":         s.opts.DefaultWorkspacePath,
		"listen_addr":       s.opts.ListenAddr,
		"session_count":     len(sessions),
		"approval_count":    len(prompts["approvals"]),
		"inquiry_count":     len(prompts["inquiries"]),
		"require_approval":  s.opts.RequireApprovalDigest,
		"default_access":    accessMode,
		"default_approval":  approvalMode,
		"default_sandbox":   s.opts.DefaultSandboxMode,
		"default_allow_list": append([]string(nil), s.opts.DefaultAllowedTools...),
	}
}
