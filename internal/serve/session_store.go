package serve

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const serveSessionStoreVersion = "v1"

type persistedSessionStore struct {
	Version  string             `json:"version"`
	Sessions []persistedSession `json:"sessions,omitempty"`
}

type persistedSession struct {
	SessionID              string               `json:"session_id"`
	Workspace              string               `json:"workspace"`
	Title                  string               `json:"title,omitempty"`
	Preview                string               `json:"preview,omitempty"`
	AllowedTools           []string             `json:"allowed_tools,omitempty"`
	ExecutionMode          string               `json:"execution_mode,omitempty"`
	TrustedWorkspace       bool                 `json:"trusted_workspace,omitempty"`
	MaxConcurrentToolCalls int                  `json:"max_concurrent_tool_calls,omitempty"`
	RequireApprovalDigest  bool                 `json:"require_approval_digest,omitempty"`
	ConfirmPolicyID        string               `json:"confirm_policy_id,omitempty"`
	Approvals              []persistedApproval  `json:"approvals,omitempty"`
	AllowSession           map[string]time.Time `json:"allow_session,omitempty"`
	UpdatedAt              time.Time            `json:"updated_at,omitempty"`
}

type persistedApproval struct {
	RequestID  string                 `json:"request_id"`
	Kind       string                 `json:"kind,omitempty"`
	CallID     string                 `json:"call_id,omitempty"`
	Tool       string                 `json:"tool,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Preview    map[string]interface{} `json:"preview,omitempty"`
	Digest     string                 `json:"digest,omitempty"`
	ExpiresAt  time.Time              `json:"expires_at,omitempty"`
	Decision   string                 `json:"decision,omitempty"`
	Used       bool                   `json:"used,omitempty"`
	PolicyID   string                 `json:"policy_id,omitempty"`
	Reason     string                 `json:"reason,omitempty"`
	Option     string                 `json:"option,omitempty"`
	Text       string                 `json:"text,omitempty"`
}

func resolveSessionStorePath(opts Options) string {
	if path := strings.TrimSpace(opts.SessionStorePath); path != "" {
		return filepath.Clean(path)
	}
	workspace := strings.TrimSpace(opts.DefaultWorkspacePath)
	if workspace == "" {
		return ""
	}
	abs, err := filepath.Abs(workspace)
	if err != nil {
		return ""
	}
	return filepath.Join(abs, ".vb", "serve", "sessions.json")
}

func (s *Server) loadPersistedSessions() error {
	path := strings.TrimSpace(s.sessionStorePath)
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	var store persistedSessionStore
	if err := json.Unmarshal(raw, &store); err != nil {
		return err
	}
	if version := strings.TrimSpace(store.Version); version != "" && version != serveSessionStoreVersion {
		return errors.New("unsupported session store version")
	}

	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range store.Sessions {
		sessionID := strings.TrimSpace(item.SessionID)
		if sessionID == "" {
			continue
		}
		workspace := strings.TrimSpace(item.Workspace)
		if workspace == "" {
			continue
		}
		abs, err := filepath.Abs(workspace)
		if err != nil {
			continue
		}
		sess := &session{
			id:                     sessionID,
			workspaceAbs:           abs,
			title:                  firstNonEmpty(strings.TrimSpace(item.Title), defaultSessionTitle(abs)),
			preview:                normalizeSessionPreview(item.Preview),
			allowedTools:           sessionAllowedTools(item.AllowedTools),
			executionMode:          strings.TrimSpace(item.ExecutionMode),
			trustedWorkspace:       item.TrustedWorkspace,
			maxConcurrentToolCalls: item.MaxConcurrentToolCalls,
			requireApprovalDigest:  item.RequireApprovalDigest,
			confirmPolicyID:        strings.TrimSpace(item.ConfirmPolicyID),
			approvals:              map[string]*approval{},
			allowSession:           map[string]time.Time{},
			results:                map[string]any{},
			runningCancels:         map[string]context.CancelFunc{},
			updatedAt:              item.UpdatedAt,
			exec:                   s.tools.NewExecutor(abs),
		}
		if sess.maxConcurrentToolCalls <= 0 {
			sess.maxConcurrentToolCalls = 1
		}
		if strings.TrimSpace(sess.executionMode) == "" {
			sess.executionMode = "default"
		}
		if sess.updatedAt.IsZero() {
			sess.updatedAt = now
		}
		for requestID, until := range item.AllowSession {
			requestID = strings.TrimSpace(requestID)
			if requestID == "" || now.After(until) {
				continue
			}
			sess.allowSession[requestID] = until
		}
		for _, saved := range item.Approvals {
			if restored := restoreApproval(saved, now); restored != nil {
				sess.approvals[restored.requestID] = restored
			}
		}
		s.sessions[sess.id] = sess
	}
	return nil
}

func (s *Server) persistSessions() error {
	path := strings.TrimSpace(s.sessionStorePath)
	if path == "" {
		return nil
	}

	store := s.snapshotPersistedSessionStore()
	if len(store.Sessions) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func (s *Server) snapshotPersistedSessionStore() persistedSessionStore {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]persistedSession, 0, len(s.sessions))
	for _, sess := range s.sessions {
		if item := snapshotSession(sess, now); item != nil {
			items = append(items, *item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return strings.Compare(items[i].SessionID, items[j].SessionID) < 0
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return persistedSessionStore{
		Version:  serveSessionStoreVersion,
		Sessions: items,
	}
}

func snapshotSession(sess *session, now time.Time) *persistedSession {
	if sess == nil {
		return nil
	}
	allowSession := make(map[string]time.Time, len(sess.allowSession))
	for digest, until := range sess.allowSession {
		digest = strings.TrimSpace(digest)
		if digest == "" || now.After(until) {
			continue
		}
		allowSession[digest] = until
	}

	approvals := make([]persistedApproval, 0, len(sess.approvals))
	for _, item := range sess.approvals {
		if saved := snapshotApproval(item, now); saved != nil {
			approvals = append(approvals, *saved)
		}
	}
	sort.Slice(approvals, func(i, j int) bool {
		if approvals[i].ExpiresAt.Equal(approvals[j].ExpiresAt) {
			return strings.Compare(approvals[i].RequestID, approvals[j].RequestID) < 0
		}
		return approvals[i].ExpiresAt.Before(approvals[j].ExpiresAt)
	})

	updatedAt := sess.updatedAt
	if updatedAt.IsZero() {
		updatedAt = now
	}
	return &persistedSession{
		SessionID:              strings.TrimSpace(sess.id),
		Workspace:              strings.TrimSpace(sess.workspaceAbs),
		Title:                  firstNonEmpty(strings.TrimSpace(sess.title), defaultSessionTitle(sess.workspaceAbs)),
		Preview:                normalizeSessionPreview(sess.preview),
		AllowedTools:           sessionAllowedToolList(sess.allowedTools),
		ExecutionMode:          strings.TrimSpace(sess.executionMode),
		TrustedWorkspace:       sess.trustedWorkspace,
		MaxConcurrentToolCalls: sess.maxConcurrentToolCalls,
		RequireApprovalDigest:  sess.requireApprovalDigest,
		ConfirmPolicyID:        strings.TrimSpace(sess.confirmPolicyID),
		Approvals:              approvals,
		AllowSession:           allowSession,
		UpdatedAt:              updatedAt,
	}
}

func snapshotApproval(item *approval, now time.Time) *persistedApproval {
	if item == nil || strings.TrimSpace(item.requestID) == "" {
		return nil
	}
	if !item.expiresAt.IsZero() && now.After(item.expiresAt) {
		return nil
	}
	if item.used && !strings.EqualFold(strings.TrimSpace(item.decision), "allow_session") {
		return nil
	}
	return &persistedApproval{
		RequestID:  strings.TrimSpace(item.requestID),
		Kind:       strings.TrimSpace(item.kind),
		CallID:     strings.TrimSpace(item.callID),
		Tool:       strings.TrimSpace(item.tool),
		Parameters: cloneMap(item.parameters),
		Preview:    cloneMap(item.preview),
		Digest:     strings.TrimSpace(item.digest),
		ExpiresAt:  item.expiresAt,
		Decision:   strings.TrimSpace(item.decision),
		Used:       item.used,
		PolicyID:   strings.TrimSpace(item.policyID),
		Reason:     strings.TrimSpace(item.reason),
		Option:     strings.TrimSpace(item.option),
		Text:       strings.TrimSpace(item.text),
	}
}

func restoreApproval(item persistedApproval, now time.Time) *approval {
	requestID := strings.TrimSpace(item.RequestID)
	if requestID == "" {
		return nil
	}
	if !item.ExpiresAt.IsZero() && now.After(item.ExpiresAt) {
		return nil
	}
	if item.Used && !strings.EqualFold(strings.TrimSpace(item.Decision), "allow_session") {
		return nil
	}
	return &approval{
		requestID:  requestID,
		kind:       strings.TrimSpace(item.Kind),
		callID:     strings.TrimSpace(item.CallID),
		tool:       strings.TrimSpace(item.Tool),
		parameters: cloneMap(item.Parameters),
		preview:    cloneMap(item.Preview),
		digest:     strings.TrimSpace(item.Digest),
		expiresAt:  item.ExpiresAt,
		decision:   strings.TrimSpace(item.Decision),
		used:       item.Used,
		policyID:   strings.TrimSpace(item.PolicyID),
		reason:     strings.TrimSpace(item.Reason),
		option:     strings.TrimSpace(item.Option),
		text:       strings.TrimSpace(item.Text),
	}
}

func sessionAllowedTools(items []string) map[string]bool {
	out := map[string]bool{}
	for _, item := range items {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		out[item] = true
	}
	return out
}

func sessionAllowedToolList(items map[string]bool) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for item, allowed := range items {
		if !allowed {
			continue
		}
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}
