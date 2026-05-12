package mcp

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dreamSailing/eos/internal/toolapi"
	"github.com/google/uuid"
	mcpmodel "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

type serverHeaderKey struct{}

type runtimeSession struct {
	ID                    string
	ConnectionID          string
	WorkspaceAbs          string
	AllowedTools          map[string]bool
	ExecutionMode         string
	AccessMode            string
	ApprovalMode          string
	SandboxMode           string
	RequireApprovalDigest bool
	UpdatedAt             time.Time
	Preview               string
	Approvals             map[string]*pendingPrompt
	Inquiries             map[string]*pendingPrompt
	LastAuthorization     map[string]any
	mu                    sync.RWMutex
}

type pendingPrompt struct {
	RequestID        string         `json:"request_id"`
	Kind             string         `json:"kind"`
	Tool             string         `json:"tool"`
	CallID           string         `json:"call_id"`
	Digest           string         `json:"digest"`
	Preview          map[string]any `json:"preview,omitempty"`
	Parameters       map[string]any `json:"parameters,omitempty"`
	ExpiresAt        time.Time      `json:"expires_at"`
	Decision         string         `json:"decision,omitempty"`
	PolicyID         string         `json:"policy_id,omitempty"`
	Reason           string         `json:"reason,omitempty"`
	TriggerReason    string         `json:"trigger_reason,omitempty"`
	TargetAccessMode string         `json:"target_access_mode,omitempty"`
	ApprovalSource   string         `json:"approval_source,omitempty"`
	Option           string         `json:"option,omitempty"`
	Text             string         `json:"text,omitempty"`
	Used             bool           `json:"used,omitempty"`
	Question         string         `json:"question,omitempty"`
	Options          []string       `json:"options,omitempty"`
	ResolvedAt       time.Time      `json:"resolved_at,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	RelatedCall      string         `json:"related_call,omitempty"`
}

type serverPolicy struct {
	ID    string             `json:"id"`
	Rules []serverPolicyRule `json:"rules"`
}

type serverPolicyRule struct {
	ID          string                 `json:"id"`
	Tool        string                 `json:"tool"`
	RiskLevel   string                 `json:"riskLevel"`
	Decision    string                 `json:"decision"`
	Constraints map[string]interface{} `json:"constraints"`
}

func loadServerPolicy(path string) (*serverPolicy, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p serverPolicy
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *serverPolicy) findRule(tool string, riskLevel string) *serverPolicyRule {
	if p == nil {
		return nil
	}
	tool = strings.ToLower(strings.TrimSpace(tool))
	riskLevel = strings.ToLower(strings.TrimSpace(riskLevel))
	for i := range p.Rules {
		r := &p.Rules[i]
		if strings.ToLower(strings.TrimSpace(r.Tool)) != tool {
			continue
		}
		if level := strings.ToLower(strings.TrimSpace(r.RiskLevel)); level != "" && level != riskLevel {
			continue
		}
		return r
	}
	return nil
}

func (r *serverPolicyRule) allowedCommands() []string {
	if r == nil {
		return nil
	}
	raw, ok := r.Constraints["allowedCommands"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, it := range arr {
		s, ok := it.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (r *serverPolicyRule) denyPathGlobs() []string {
	if r == nil {
		return nil
	}
	raw, ok := r.Constraints["denyPathGlobs"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, it := range arr {
		s, ok := it.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, filepath.ToSlash(s))
		}
	}
	return out
}

func (s *Server) ensureSession(ctx context.Context, args map[string]any) (*runtimeSession, error) {
	sessionID := strings.TrimSpace(stringArg(args, "session_id"))
	if sessionID != "" {
		s.mu.RLock()
		sess := s.sessions[sessionID]
		s.mu.RUnlock()
		if sess == nil {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return sess, nil
	}
	return s.ensureDefaultSession(ctx)
}

func (s *Server) ensureDefaultSession(ctx context.Context) (*runtimeSession, error) {
	connID := connectionIDFromContext(ctx)
	if connID == "" {
		connID = "default"
	}
	s.mu.RLock()
	if sessionID := s.defaultSessionByConn[connID]; sessionID != "" {
		if sess := s.sessions[sessionID]; sess != nil {
			s.mu.RUnlock()
			return sess, nil
		}
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if sessionID := s.defaultSessionByConn[connID]; sessionID != "" {
		if sess := s.sessions[sessionID]; sess != nil {
			return sess, nil
		}
	}
	sess := newRuntimeSession(
		connID,
		s.opts.DefaultWorkspacePath,
		allowedToolsMap(s.opts.DefaultAllowedTools),
		"auto",
		s.opts.DefaultAccessMode,
		s.opts.DefaultApprovalMode,
		s.opts.DefaultSandboxMode,
		s.opts.RequireApprovalDigest,
	)
	s.sessions[sess.ID] = sess
	s.defaultSessionByConn[connID] = sess.ID
	return sess, nil
}

func (s *Server) createSession(ctx context.Context, args map[string]any) (*runtimeSession, error) {
	connID := connectionIDFromContext(ctx)
	if connID == "" {
		connID = "default"
	}
	workspace := strings.TrimSpace(stringArg(args, "workspace_root"))
	if workspace == "" {
		workspace = s.opts.DefaultWorkspacePath
	}
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	executionMode := toolapi.NormalizeExecutionMode(stringArg(args, "execution_mode"))
	accessMode := strings.TrimSpace(stringArg(args, "access_mode"))
	approvalMode := strings.TrimSpace(stringArg(args, "approval_mode"))
	sandboxMode := toolapi.NormalizeSandboxMode(stringArg(args, "sandbox_mode"))
	if accessMode != "" {
		accessMode = toolapi.NormalizeAccessMode(accessMode)
		sandboxMode = toolapi.SandboxModeFromAccessMode(accessMode)
	}
	if sandboxMode == "" {
		sandboxMode = s.opts.DefaultSandboxMode
	}
	requireDigest := boolArg(args, "require_approval_digest", s.opts.RequireApprovalDigest)
	if accessMode == "" && strings.TrimSpace(s.opts.DefaultAccessMode) != "" {
		accessMode = toolapi.NormalizeAccessMode(s.opts.DefaultAccessMode)
	}
	if approvalMode != "" {
		approvalMode = toolapi.NormalizeApprovalMode(approvalMode)
	} else if strings.TrimSpace(s.opts.DefaultApprovalMode) != "" {
		approvalMode = toolapi.NormalizeApprovalMode(s.opts.DefaultApprovalMode)
	}
	allowed := parseStringSliceArg(args["allowed_tools"])
	if len(allowed) == 0 {
		allowed = s.opts.DefaultAllowedTools
	}
	sess := newRuntimeSession(connID, workspaceAbs, allowedToolsMap(allowed), executionMode, accessMode, approvalMode, sandboxMode, requireDigest)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID] = sess
	if boolArg(args, "set_default", false) || s.defaultSessionByConn[connID] == "" {
		s.defaultSessionByConn[connID] = sess.ID
	}
	return sess, nil
}

func newRuntimeSession(connID, workspace string, allowed map[string]bool, executionMode, accessMode, approvalMode, sandboxMode string, requireDigest bool) *runtimeSession {
	if executionMode == "" {
		executionMode = "auto"
	}
	if sandboxMode == "" {
		sandboxMode = "workspace"
	}
	var allowedCopy map[string]bool
	if allowed != nil {
		allowedCopy = make(map[string]bool, len(allowed))
		for k, v := range allowed {
			allowedCopy[k] = v
		}
	}
	return &runtimeSession{
		ID:                    "sess_" + uuid.NewString()[:12],
		ConnectionID:          connID,
		WorkspaceAbs:          workspace,
		AllowedTools:          allowedCopy,
		ExecutionMode:         toolapi.NormalizeExecutionMode(executionMode),
		AccessMode:            normalizeOptionalAccessMode(accessMode),
		ApprovalMode:          normalizeOptionalApprovalMode(approvalMode),
		SandboxMode:           toolapi.NormalizeSandboxMode(sandboxMode),
		RequireApprovalDigest: requireDigest,
		UpdatedAt:             time.Now(),
		Approvals:             map[string]*pendingPrompt{},
		Inquiries:             map[string]*pendingPrompt{},
	}
}

func (s *Server) getSession(id string) *runtimeSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[strings.TrimSpace(id)]
}

func (s *Server) listSessions() []*runtimeSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*runtimeSession, 0, len(s.sessions))
	for _, sess := range s.sessions {
		out = append(out, sess)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

func (s *Server) closeSession(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.sessions[id]
	if sess == nil {
		return false
	}
	delete(s.sessions, id)
	for connID, sessionID := range s.defaultSessionByConn {
		if sessionID == id {
			delete(s.defaultSessionByConn, connID)
		}
	}
	return true
}

func (s *runtimeSession) touch(preview string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UpdatedAt = time.Now()
	if text := normalizePreview(preview); text != "" {
		s.Preview = text
	}
}

func (s *runtimeSession) snapshot() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	execSess := toolapi.ExecSession{
		ExecutionMode:         s.ExecutionMode,
		AccessMode:            s.AccessMode,
		ApprovalMode:          s.ApprovalMode,
		SandboxMode:           s.SandboxMode,
		RequireApprovalDigest: s.RequireApprovalDigest,
	}
	return map[string]any{
		"id":                      s.ID,
		"connection_id":           s.ConnectionID,
		"workspace_root":          s.WorkspaceAbs,
		"execution_mode":          s.ExecutionMode,
		"access_mode":             toolapi.ResolveAccessMode(execSess),
		"approval_mode":           toolapi.ResolveApprovalMode(execSess),
		"sandbox_mode":            toolapi.NormalizeSandboxMode(s.SandboxMode),
		"require_approval_digest": s.RequireApprovalDigest,
		"updated_at":              s.UpdatedAt.Format(time.RFC3339),
		"preview":                 s.Preview,
		"pending_approvals":       s.pendingIDsLocked(s.Approvals),
		"pending_inquiries":       s.pendingIDsLocked(s.Inquiries),
		"allowed_tools":           allowedToolsSlice(s.AllowedTools),
		"last_authorization":      cloneMap(s.LastAuthorization),
	}
}

func (s *runtimeSession) pendingIDsLocked(items map[string]*pendingPrompt) []string {
	out := make([]string, 0, len(items))
	now := time.Now()
	for id, item := range items {
		if item == nil || now.After(item.ExpiresAt) || item.Decision != "" {
			continue
		}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func (s *runtimeSession) approvalList() []map[string]any {
	return pendingPromptList(s.Approvals)
}

func (s *runtimeSession) inquiryList() []map[string]any {
	return pendingPromptList(s.Inquiries)
}

func pendingPromptList(items map[string]*pendingPrompt) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, pendingPromptSnapshot(item))
	}
	sort.Slice(out, func(i, j int) bool {
		return stringValue(out[i]["request_id"]) < stringValue(out[j]["request_id"])
	})
	return out
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

func pendingPromptSnapshot(item *pendingPrompt) map[string]any {
	return map[string]any{
		"request_id":         item.RequestID,
		"kind":               item.Kind,
		"tool":               item.Tool,
		"call_id":            item.CallID,
		"digest":             item.Digest,
		"preview":            cloneMap(item.Preview),
		"parameters":         cloneMap(item.Parameters),
		"expires_at":         item.ExpiresAt.Format(time.RFC3339),
		"decision":           item.Decision,
		"policy_id":          item.PolicyID,
		"reason":             item.Reason,
		"trigger_reason":     item.TriggerReason,
		"target_access_mode": item.TargetAccessMode,
		"approval_source":    item.ApprovalSource,
		"option":             item.Option,
		"text":               item.Text,
		"used":               item.Used,
		"question":           item.Question,
		"options":            append([]string(nil), item.Options...),
		"resolved_at":        timeString(item.ResolvedAt),
		"created_at":         item.CreatedAt.Format(time.RFC3339),
	}
}

func timeString(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.Format(time.RFC3339)
}

func connectionIDFromContext(ctx context.Context) string {
	if session := mcpserver.ClientSessionFromContext(ctx); session != nil {
		return strings.TrimSpace(session.SessionID())
	}
	return ""
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func allowedToolsSlice(in map[string]bool) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for k, v := range in {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func normalizePreview(text string) string {
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

func parseStringSliceArg(raw any) []string {
	switch v := raw.(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if text, ok := item.(string); ok {
				text = strings.TrimSpace(text)
				if text != "" {
					out = append(out, text)
				}
			}
		}
		return out
	default:
		return nil
	}
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if value, ok := args[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func stringValue(v any) string {
	text, _ := v.(string)
	return text
}

func boolArg(args map[string]any, key string, defaultValue bool) bool {
	if args == nil {
		return defaultValue
	}
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err == nil {
			return parsed
		}
	}
	return defaultValue
}

func approvalDigest(payload any) (string, []byte, error) {
	b, err := canonicalJSON(payload)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), b, nil
}

func canonicalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeCanonical(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeCanonical(buf *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		buf.WriteString("null")
		return nil
	case bool:
		if x {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return nil
	case string:
		b, _ := json.Marshal(x)
		buf.Write(b)
		return nil
	case json.Number:
		buf.WriteString(x.String())
		return nil
	case float64:
		buf.WriteString(strconv.FormatFloat(x, 'g', -1, 64))
		return nil
	case float32:
		buf.WriteString(strconv.FormatFloat(float64(x), 'g', -1, 32))
		return nil
	case int:
		buf.WriteString(strconv.FormatInt(int64(x), 10))
		return nil
	case int64:
		buf.WriteString(strconv.FormatInt(x, 10))
		return nil
	case int32:
		buf.WriteString(strconv.FormatInt(int64(x), 10))
		return nil
	case uint64:
		buf.WriteString(strconv.FormatUint(x, 10))
		return nil
	case uint32:
		buf.WriteString(strconv.FormatUint(uint64(x), 10))
		return nil
	case []any:
		buf.WriteByte('[')
		for i, it := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, it); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	case map[string]any:
		return writeCanonicalMap(buf, x)
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Errorf("unsupported type: %T", v)
		}
		var anyv any
		if err := json.Unmarshal(b, &anyv); err != nil {
			return err
		}
		return writeCanonical(buf, anyv)
	}
}

func writeCanonicalMap(buf *bytes.Buffer, m map[string]any) error {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		buf.Write(kb)
		buf.WriteByte(':')
		if err := writeCanonical(buf, m[k]); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
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
		if strings.TrimSpace(it) == strings.TrimSpace(s) {
			return true
		}
	}
	return false
}

func makeTextResource(uri string, payload any) ([]mcpmodel.ResourceContents, error) {
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	return []mcpmodel.ResourceContents{
		mcpmodel.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(b),
		},
	}, nil
}
