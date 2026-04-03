package bridge

import (
	"github.com/dreamSailing/vb-coding/internal/runtime"
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

// NewSecurityManager 创建新的安全管理器
func NewSecurityManager() *SecurityManager {
	return &SecurityManager{
		permsAllowSession: make(map[string]bool),
		executionMode:     "auto",
	}
}

func (s *SecurityManager) SetExecutionMode(mode string) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "auto"
	}
	if mode != "manual" && mode != "plan" && mode != "auto" && mode != "bypass" {
		mode = "auto"
	}
	s.permMu.Lock()
	s.executionMode = mode
	if mode == "auto" || mode == "bypass" {
		s.permsAllowSession["auto_all"] = true
	} else {
		delete(s.permsAllowSession, "auto_all")
	}
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
	if s.permsAllowSession["auto_all"] {
		return true
	}
	return s.permsAllowSession[category]
}

// SetPendingDiff 设置待处理的差异内容
func (s *SecurityManager) SetPendingDiff(diff string) {
	s.permMu.Lock()
	defer s.permMu.Unlock()
	s.pendingDiff = diff
}
