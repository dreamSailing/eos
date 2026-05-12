package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"github.com/dreamSailing/eos/internal/pkg/settings"
	"github.com/dreamSailing/eos/internal/runtime"
	"github.com/dreamSailing/eos/internal/toolapi"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// SecurityManager 管理权限和安全钩子
type SecurityManager struct {
	permsAllowSession map[string]bool
	pendingDiff       string
	pendingDiffPath   string
	executionMode     string
	accessMode        string
	approvalMode      string
	sandboxMode       string
	previousMode      string // 进入 plan 前保存的模式
	permMu            sync.RWMutex
	hooks             runtime.SafetyGate
	onModeChange      func(oldMode, newMode string) // 模式切换回调
	deniedTools       map[string]bool               // fine-grained: denied tool names
	allowedTools      map[string]bool               // fine-grained: allowed tool names (if non-empty, whitelist)
	skipPermissions   bool                          // --dangerously-skip-permissions bypass
	rules             []settings.PermissionRule     // pattern-based permission rules
	lastAuthDecision  string
	lastAuthCategory  string
	lastAuthSummary   string
	lastAuthReason    string
	lastAuthTarget    string
	lastAuthAt        string
}

type PermissionSnapshot struct {
	ExecutionMode           string
	AccessMode              string
	ApprovalMode            string
	SandboxMode             string
	AllowAll                bool
	AllowedCategories       []string
	HasPendingDiff          bool
	PendingDiffPath         string
	LastAuthorization       string
	LastAuthorizationAt     string
	LastAuthorizationKind   string
	LastAuthorizationNote   string
	LastAuthorizationTarget string
}

// NewSecurityManager 创建新的安全管理器
func NewSecurityManager() *SecurityManager {
	return &SecurityManager{
		permsAllowSession: make(map[string]bool),
		executionMode:     "auto",
		accessMode:        "workspace-write",
		approvalMode:      "on-request",
		sandboxMode:       "workspace",
	}
}

func (s *SecurityManager) SetExecutionMode(mode string) {
	mode = toolapi.NormalizeExecutionMode(mode)
	s.permMu.Lock()
	old := s.executionMode
	s.executionMode = mode
	cb := s.onModeChange
	s.permMu.Unlock()
	if cb != nil && old != mode {
		cb(old, mode)
	}
}

// SetModeChangeCallback sets the callback for mode changes
func (s *SecurityManager) SetModeChangeCallback(cb func(oldMode, newMode string)) {
	s.permMu.Lock()
	defer s.permMu.Unlock()
	s.onModeChange = cb
}

// SwitchToPlanMode saves current mode and switches to plan
func (s *SecurityManager) SwitchToPlanMode() {
	s.permMu.Lock()
	s.previousMode = s.executionMode
	old := s.executionMode
	s.executionMode = "plan"
	cb := s.onModeChange
	s.permMu.Unlock()
	if cb != nil {
		cb(old, "plan")
	}
}

// RestorePreviousMode restores the mode saved before entering plan
func (s *SecurityManager) RestorePreviousMode() {
	s.permMu.Lock()
	prev := s.previousMode
	if prev == "" {
		prev = "auto"
	}
	old := s.executionMode
	s.executionMode = prev
	s.previousMode = ""
	cb := s.onModeChange
	s.permMu.Unlock()
	if cb != nil && old != prev {
		cb(old, prev)
	}
}

// PreviousMode returns the saved previous mode
func (s *SecurityManager) PreviousMode() string {
	s.permMu.RLock()
	defer s.permMu.RUnlock()
	if s.previousMode == "" {
		return "auto"
	}
	return s.previousMode
}

func (s *SecurityManager) ExecutionMode() string {
	s.permMu.RLock()
	v := s.executionMode
	s.permMu.RUnlock()
	return v
}

func (s *SecurityManager) SetSandboxMode(mode string) {
	mode = toolapi.NormalizeSandboxMode(mode)
	s.permMu.Lock()
	s.sandboxMode = mode
	if strings.TrimSpace(s.accessMode) == "" {
		s.accessMode = toolapi.ResolveAccessMode(toolapi.ExecSession{SandboxMode: mode})
	}
	s.permMu.Unlock()
}

func (s *SecurityManager) SandboxMode() string {
	s.permMu.RLock()
	v := s.sandboxMode
	s.permMu.RUnlock()
	return toolapi.NormalizeSandboxMode(v)
}

func (s *SecurityManager) SetAccessMode(mode string) {
	mode = toolapi.NormalizeAccessMode(mode)
	s.permMu.Lock()
	s.accessMode = mode
	s.sandboxMode = toolapi.SandboxModeFromAccessMode(mode)
	s.permMu.Unlock()
}

func (s *SecurityManager) AccessMode() string {
	s.permMu.RLock()
	v := s.accessMode
	s.permMu.RUnlock()
	return toolapi.NormalizeAccessMode(v)
}

func (s *SecurityManager) SetApprovalMode(mode string) {
	mode = toolapi.NormalizeApprovalMode(mode)
	s.permMu.Lock()
	s.approvalMode = mode
	s.permMu.Unlock()
}

func (s *SecurityManager) ApprovalMode() string {
	s.permMu.RLock()
	v := s.approvalMode
	s.permMu.RUnlock()
	return toolapi.NormalizeApprovalMode(v)
}

// GetHooks 获取安全钩子
func (s *SecurityManager) GetHooks() runtime.SafetyGate {
	s.permMu.RLock()
	defer s.permMu.RUnlock()
	return s.hooks
}

// SetHooks 设置安全钩子
func (s *SecurityManager) SetHooks(hooks runtime.SafetyGate) {
	s.permMu.Lock()
	defer s.permMu.Unlock()
	s.hooks = hooks
}

// GetPendingDiff 获取待处理的差异
func (s *SecurityManager) GetPendingDiff() string {
	s.permMu.RLock()
	defer s.permMu.RUnlock()
	return s.pendingDiff
}

// ClearPendingDiff 清除待处理的差异
func (s *SecurityManager) ClearPendingDiff() {
	s.permMu.Lock()
	defer s.permMu.Unlock()
	s.pendingDiff = ""
	s.pendingDiffPath = ""
}

// SetPendingDiffPath 设置待处理差异的路径
func (s *SecurityManager) SetPendingDiffPath(p string) {
	s.permMu.Lock()
	defer s.permMu.Unlock()
	s.pendingDiffPath = p
}

// GetPendingDiffPath 获取待处理差异的路径
func (s *SecurityManager) GetPendingDiffPath() string {
	s.permMu.RLock()
	defer s.permMu.RUnlock()
	return s.pendingDiffPath
}

// AllowSession 允许会话级别的权限
func (s *SecurityManager) AllowSession(category string) {
	s.permMu.Lock()
	defer s.permMu.Unlock()
	s.permsAllowSession[category] = true
}

// DenySession 拒绝会话级别的权限
func (s *SecurityManager) DenySession(category string) {
	s.permMu.Lock()
	defer s.permMu.Unlock()
	delete(s.permsAllowSession, category)
}

// IsAllowed 检查是否允许某类别操作
func (s *SecurityManager) IsAllowed(category string) bool {
	s.permMu.RLock()
	defer s.permMu.RUnlock()
	return s.permsAllowSession[category]
}

// SetPendingDiff 设置待处理的差异内容
func (s *SecurityManager) SetPendingDiff(diff string) {
	s.permMu.Lock()
	defer s.permMu.Unlock()
	s.pendingDiff = diff
}

func (s *SecurityManager) Snapshot() PermissionSnapshot {
	s.permMu.RLock()
	defer s.permMu.RUnlock()

	snap := PermissionSnapshot{
		ExecutionMode:           s.executionMode,
		AccessMode:              toolapi.NormalizeAccessMode(s.accessMode),
		ApprovalMode:            toolapi.NormalizeApprovalMode(s.approvalMode),
		SandboxMode:             toolapi.NormalizeSandboxMode(s.sandboxMode),
		AllowAll:                toolapi.NormalizeSandboxMode(s.sandboxMode) == "full_access",
		HasPendingDiff:          strings.TrimSpace(s.pendingDiff) != "",
		PendingDiffPath:         strings.TrimSpace(s.pendingDiffPath),
		LastAuthorization:       strings.TrimSpace(s.lastAuthDecision),
		LastAuthorizationAt:     strings.TrimSpace(s.lastAuthAt),
		LastAuthorizationKind:   strings.TrimSpace(s.lastAuthCategory),
		LastAuthorizationNote:   strings.TrimSpace(permissionNote(s.lastAuthSummary, s.lastAuthReason)),
		LastAuthorizationTarget: strings.TrimSpace(s.lastAuthTarget),
	}
	for category, allowed := range s.permsAllowSession {
		if !allowed {
			continue
		}
		snap.AllowedCategories = append(snap.AllowedCategories, category)
	}
	sort.Strings(snap.AllowedCategories)
	return snap
}

func (s *SecurityManager) RecordAuthorization(decision, category, summary, reason, target string) {
	s.permMu.Lock()
	defer s.permMu.Unlock()
	s.lastAuthDecision = strings.TrimSpace(decision)
	s.lastAuthCategory = strings.TrimSpace(category)
	s.lastAuthSummary = strings.TrimSpace(summary)
	s.lastAuthReason = strings.TrimSpace(reason)
	s.lastAuthTarget = strings.TrimSpace(target)
	s.lastAuthAt = time.Now().Format(time.RFC3339)
}

func permissionNote(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

// LoadPermissions loads fine-grained tool permissions from settings
func (s *SecurityManager) LoadPermissions(p *settings.Permissions) {
	s.permMu.Lock()
	defer s.permMu.Unlock()
	if p == nil {
		s.deniedTools = nil
		s.allowedTools = nil
		s.rules = nil
		return
	}
	s.deniedTools = make(map[string]bool, len(p.DeniedTools))
	for _, t := range p.DeniedTools {
		s.deniedTools[t] = true
	}
	s.allowedTools = make(map[string]bool, len(p.AllowedTools))
	for _, t := range p.AllowedTools {
		s.allowedTools[t] = true
	}
	s.rules = p.Rules
}

// IsToolDenied checks if a tool is denied by fine-grained permissions.
// Priority: pattern rules (first match wins) > denied list > whitelist.
func (s *SecurityManager) IsToolDenied(toolName string) bool {
	s.permMu.RLock()
	defer s.permMu.RUnlock()
	if s.skipPermissions {
		return false
	}

	// 1. Check pattern-based rules (first match wins)
	if decision, matched := s.evaluateRules(toolName); matched {
		return decision == "deny"
	}

	// 2. Check flat denied list
	if s.deniedTools != nil && s.deniedTools[toolName] {
		return true
	}

	// 3. Check whitelist
	if len(s.allowedTools) > 0 && !s.allowedTools[toolName] {
		return true
	}

	return false
}

// NeedsApproval checks if a tool requires user approval (pattern rule with "ask" decision)
func (s *SecurityManager) NeedsApproval(toolName string) bool {
	s.permMu.RLock()
	defer s.permMu.RUnlock()
	if s.skipPermissions {
		return false
	}
	if decision, matched := s.evaluateRules(toolName); matched {
		return decision == "ask"
	}
	return false
}

// evaluateRules evaluates pattern-based permission rules for a tool name.
// Returns (decision, matched) where matched is true if a rule matched.
func (s *SecurityManager) evaluateRules(toolName string) (decision string, matched bool) {
	for _, rule := range s.rules {
		if matchToolPattern(toolName, rule.Pattern) {
			return rule.Decision, true
		}
	}
	return "", false
}

// matchToolPattern matches a tool name against a glob pattern.
// Supports:
//   - "bash:*rm*" matches "bash" with any command containing "rm"
//   - "edit:*" matches all edit tool calls
//   - "bash" matches exact tool name
//   - "*" matches all tools
func matchToolPattern(toolName, pattern string) bool {
	// Exact match
	if pattern == toolName {
		return true
	}
	// Wildcard all
	if pattern == "*" {
		return true
	}
	// Glob matching using filepath.Match
	// Handle patterns like "bash:*", "edit:*", "git:*"
	if matched, _ := filepath.Match(pattern, toolName); matched {
		return true
	}
	// Handle compound patterns like "bash:*rm*" (tool:subpattern)
	if parts := strings.SplitN(pattern, ":", 2); len(parts) == 2 {
		if parts[0] == toolName || parts[0] == "*" {
			return true // If tool matches, the subpattern is informational
		}
	}
	return false
}

// SetSkipPermissions sets the bypass mode for --dangerously-skip-permissions
func (s *SecurityManager) SetSkipPermissions(skip bool) {
	s.permMu.Lock()
	defer s.permMu.Unlock()
	s.skipPermissions = skip
	if skip {
		s.accessMode = "danger-full-access"
		s.sandboxMode = "full_access"
		s.approvalMode = "never"
	} else {
		if strings.TrimSpace(s.accessMode) == "" {
			s.accessMode = "workspace-write"
		}
		if strings.TrimSpace(s.approvalMode) == "" {
			s.approvalMode = "on-request"
		}
		s.sandboxMode = toolapi.SandboxModeFromAccessMode(s.accessMode)
	}
}
