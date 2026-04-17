//go:build !without_lsp

package lsp

import (
	"context"
	"log/slog"

	"github.com/dreamSailing/eos/internal/pkg/utils"
)

// SafeLSP 安全的 LSP 包装器，优雅降级
type SafeLSP struct {
	manager *Manager
}

// NewSafeLSP 创建安全的 LSP 包装器
func NewSafeLSP(config Config) *SafeLSP {
	return &SafeLSP{
		manager: NewManager(config),
	}
}

// GetClient 安全获取客户端，失败时返回 nil
func (s *SafeLSP) GetClient(ctx context.Context, path string) (*Client, error) {
	if !s.manager.IsEnabled() {
		return nil, ErrDisabled
	}

	client, err := s.manager.GetClientForPath(ctx, path)
	if err != nil {
		// 降级：记录但不报错
		slog.Debug("lsp.safe.get_client.failed",
			"component", utils.ComponentSystem,
			"path", path,
			"error", err)
		return nil, nil
	}

	return client, nil
}

// GetDiagnostics 安全获取诊断，失败时返回空
func (s *SafeLSP) GetDiagnostics(ctx context.Context, path string) []Diagnostic {
	client, err := s.GetClient(ctx, path)
	if err != nil || client == nil {
		return nil
	}

	diagnostics, err := s.manager.GetDiagnostics(path)
	if err != nil {
		slog.Debug("lsp.safe.get_diagnostics.failed",
			"component", utils.ComponentSystem,
			"path", path,
			"error", err)
		return nil
	}

	return diagnostics
}

// DidOpen 安全打开文档，失败时静默跳过
func (s *SafeLSP) DidOpen(ctx context.Context, uri, languageID, text string) {
	client, err := s.GetClient(ctx, uri)
	if err != nil || client == nil {
		return
	}

	if err := client.DidOpen(ctx, uri, languageID, text); err != nil {
		slog.Debug("lsp.safe.did_open.failed",
			"component", utils.ComponentSystem,
			"uri", uri,
			"error", err)
	}
}

// DidChange 安全修改文档，失败时静默跳过
func (s *SafeLSP) DidChange(ctx context.Context, uri string, version int, text string) {
	client, err := s.GetClient(ctx, uri)
	if err != nil || client == nil {
		return
	}

	if err := client.DidChange(ctx, uri, version, text); err != nil {
		slog.Debug("lsp.safe.did_change.failed",
			"component", utils.ComponentSystem,
			"uri", uri,
			"error", err)
	}
}

// SetEnabled 设置启用状态
func (s *SafeLSP) SetEnabled(enabled bool) {
	s.manager.SetEnabled(enabled)
	slog.Debug("lsp.safe.enabled_changed",
		"component", utils.ComponentSystem,
		"enabled", enabled)
}

// IsEnabled 检查是否启用
func (s *SafeLSP) IsEnabled() bool {
	return s.manager.IsEnabled()
}

// Close 关闭
func (s *SafeLSP) Close() error {
	return s.manager.Close()
}

// GetManager 获取管理器（用于高级操作）
func (s *SafeLSP) GetManager() *Manager {
	return s.manager
}

// Global LSP 实例
var globalLSP *SafeLSP

// InitGlobalLSP 初始化全局 LSP
func InitGlobalLSP(config Config) {
	globalLSP = NewSafeLSP(config)
}

// GetGlobalLSP 获取全局 LSP 实例
func GetGlobalLSP() *SafeLSP {
	return globalLSP
}

// HasGlobalLSP 检查是否有全局 LSP 实例
func HasGlobalLSP() bool {
	return globalLSP != nil
}

// SafeDiagnostics 安全获取诊断信息（用于外部调用）
func SafeDiagnostics(path string) []Diagnostic {
	if !HasGlobalLSP() {
		return nil
	}
	return globalLSP.GetDiagnostics(context.Background(), path)
}

// SafeNotifyOpen 安全通知文档打开
func SafeNotifyOpen(uri, languageID, text string) {
	if !HasGlobalLSP() {
		return
	}
	globalLSP.DidOpen(context.Background(), uri, languageID, text)
}

// SafeNotifyChange 安全通知文档修改
func SafeNotifyChange(uri string, version int, text string) {
	if !HasGlobalLSP() {
		return
	}
	globalLSP.DidChange(context.Background(), uri, version, text)
}
