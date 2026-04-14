package bridge

import (
	"github.com/dreamSailing/vb-coding/internal/runtime"
	"github.com/dreamSailing/vb-coding/internal/toolapi"
	"sort"
	"strings"
	"sync"
)

// SecurityManager 管理权限和安全钩子
type SecurityManager struct {
	permsAllowSession map[string]bool
	pendingDiff       string
	pendingDiffPath   string
	executionMode     string
	permMu            sync.RWMutex
	hooks             runtime.SafetyGate
}

type PermissionSnapshot struct {
	ExecutionMode     string
	AllowAll          bool
	AllowedCategories []string
	HasPendingDiff    bool
	PendingDiffPath   string
}

// NewSecurityManager 创建新的安全管理器
func NewSecurityManager() *SecurityManager {
	return &SecurityManager{
		permsAllowSession: make(map[string]bool),
		executionMode:     "auto",
	}
}

func (s *SecurityManager) SetExecutionMode(mode string) {
	mode = toolapi.NormalizeExecutionMode(mode)
	s.permMu.Lock()
	s.executionMode = mode
	s.permMu.Unlock()
}

func (s *SecurityManager) ExecutionMode() string {
	s.permMu.RLock()
	v := s.executionMode
	s.permMu.RUnlock()
	return v
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
		ExecutionMode:   s.executionMode,
		HasPendingDiff:  strings.TrimSpace(s.pendingDiff) != "",
		PendingDiffPath: strings.TrimSpace(s.pendingDiffPath),
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
