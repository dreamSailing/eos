//go:build !without_lsp

package lsp

// Copyright (c) 2026 EOSAIOS
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/eosaios/eos/internal/pkg/events"
	"github.com/eosaios/eos/internal/pkg/utils"
)

// Manager LSP 管理器
type Manager struct {
	mu          sync.RWMutex
	clients     map[string]*Client // key: rootURI
	detector    *Detector
	diagnostics *DiagnosticStore
	config      Config
}

// Config 配置
type Config struct {
	Enabled    bool          // 总开关
	AutoDetect bool          // 自动检测
	Timeout    time.Duration // 启动超时
}

// DefaultConfig 默认配置
var DefaultConfig = Config{
	Enabled:    true,
	AutoDetect: true,
	Timeout:    10 * time.Second,
}

// NewManager 创建管理器
func NewManager(config Config) *Manager {
	if config.Timeout == 0 {
		config.Timeout = DefaultConfig.Timeout
	}

	return &Manager{
		clients:     make(map[string]*Client),
		detector:    NewDetector(),
		diagnostics: NewDiagnosticStore(),
		config:      config,
	}
}

// GetClientForPath 为路径获取或创建客户端
func (m *Manager) GetClientForPath(ctx context.Context, path string) (*Client, error) {
	if !m.config.Enabled {
		return nil, ErrDisabled
	}

	// 转换为绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	// 如果是文件，获取目录
	if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
		absPath = filepath.Dir(absPath)
	}

	// 检查是否已有客户端
	m.mu.RLock()
	client := m.clients[absPath]
	m.mu.RUnlock()

	if client != nil && client.IsRunning() {
		return client, nil
	}

	// 创建新客户端
	return m.createClient(ctx, absPath)
}

func (m *Manager) GetRunningClientForPath(path string) (*Client, bool) {
	if m == nil || !m.config.Enabled {
		return nil, false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, false
	}
	if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
		absPath = filepath.Dir(absPath)
	}
	m.mu.RLock()
	client := m.clients[absPath]
	m.mu.RUnlock()
	if client != nil && client.IsRunning() {
		return client, true
	}
	return nil, false
}

// createClient 创建新客户端
func (m *Manager) createClient(ctx context.Context, rootPath string) (*Client, error) {
	// 检测语言
	lang := m.detector.DetectLanguage(rootPath)
	if lang == "" {
		return nil, ErrNoLanguageDetected
	}

	// 检查是否支持
	if !m.detector.IsLanguageSupported(lang) {
		return nil, ErrUnsupportedLanguage
	}

	// 查找服务器
	serverInfo, err := m.detector.FindServer(lang)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrServerNotFound, err)
	}

	// 创建客户端
	client := NewClient(ClientConfig{
		Command: serverInfo.Command,
		Args:    serverInfo.Args,
		RootURI: fileURI(rootPath),
	})

	// 设置诊断回调
	client.SetDiagnosticsCallback(func(params PublishDiagnosticsParams) {
		m.handleDiagnostics(params)
	})

	// 启动客户端（带超时）
	startCtx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	if err := client.Start(startCtx); err != nil {
		return nil, fmt.Errorf("failed to start client: %w", err)
	}

	// 初始化
	if err := client.Initialize(startCtx); err != nil {
		_ = client.Stop()
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}

	// 保存客户端
	m.mu.Lock()
	m.clients[rootPath] = client
	m.mu.Unlock()

	slog.Debug("lsp.manager.client_created",
		"component", utils.ComponentSystem,
		"root_path", rootPath,
		"language", lang,
		"command", serverInfo.Command)

	return client, nil
}

// handleDiagnostics 处理诊断信息
func (m *Manager) handleDiagnostics(params PublishDiagnosticsParams) {
	m.diagnostics.Set(params.URI, params.Version, params.Diagnostics)

	// 发布事件到总线
	events.Publish(events.TopicLSPDiagnostics, params)

	slog.Debug("lsp.manager.diagnostics",
		"component", utils.ComponentSystem,
		"uri", params.URI,
		"count", len(params.Diagnostics))
}

// GetDiagnostics 获取文件的诊断信息
func (m *Manager) GetDiagnostics(filePath string) ([]Diagnostic, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}

	uri := DocumentURI(fileURI(absPath))
	diag, ok := m.diagnostics.Get(uri)
	if !ok {
		return nil, nil
	}

	return diag.Diagnostics, nil
}

// Close 关闭管理器
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var firstErr error
	for root, client := range m.clients {
		if err := client.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(m.clients, root)
	}

	return firstErr
}

// IsEnabled 检查是否启用
func (m *Manager) IsEnabled() bool {
	return m.config.Enabled
}

// GetAllDiagnostics 获取所有诊断信息
func (m *Manager) GetAllDiagnostics() map[string][]Diagnostic {
	m.mu.RLock()
	defer m.mu.RUnlock()

	allItems := m.diagnostics.GetAll()
	result := make(map[string][]Diagnostic, len(allItems))

	for uri, fileDiag := range allItems {
		result[uri] = fileDiag.Diagnostics
	}

	return result
}

// SetEnabled 设置启用状态
func (m *Manager) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Enabled = enabled

	if !enabled {
		// 关闭所有客户端
		for _, client := range m.clients {
			_ = client.Stop()
		}
		m.clients = make(map[string]*Client)
	}
}

// fileURI 将路径转换为 file:// URI
func fileURI(path string) string {
	absPath, err := filepath.Abs(path)
	if err == nil {
		path = absPath
	}

	// Windows 路径转换
	if len(path) >= 2 && path[1] == ':' {
		path = "/" + string(path[0]) + ":" + path[2:]
		// 替换反斜杠
		path = filepath.ToSlash(path)
	}

	return "file://" + path
}

// 错误定义
var (
	// ErrDisabled LSP 已禁用
	ErrDisabled = fmt.Errorf("lsp is disabled")
	// ErrNoLanguageDetected 未检测到语言
	ErrNoLanguageDetected = fmt.Errorf("no language detected")
	// ErrUnsupportedLanguage 不支持的语言
	ErrUnsupportedLanguage = fmt.Errorf("unsupported language")
	// ErrServerNotFound 未找到 LSP 服务器
	ErrServerNotFound = fmt.Errorf("lsp server not found")
)
