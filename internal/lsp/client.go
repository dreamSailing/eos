//go:build !without_lsp

package lsp

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dreamSailing/eos/internal/pkg/utils"
)

// Client LSP 客户端
type Client struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.Reader
	stderr  io.Reader
	cmdPath string
	cmdArgs []string
	rootURI string

	// 通信
	nextID atomic.Int64
	notify chan Notification
	mu     sync.RWMutex

	// 状态
	started atomic.Bool
	stopped atomic.Bool

	// 诊断回调
	onDiagnostics func(params PublishDiagnosticsParams)
}

// Notification 服务器通知
type Notification struct {
	Method string
	Params json.RawMessage
}

// ClientConfig 客户端配置
type ClientConfig struct {
	Command string
	Args    []string
	RootURI string
}

// NewClient 创建 LSP 客户端
func NewClient(config ClientConfig) *Client {
	return &Client{
		cmdPath: config.Command,
		cmdArgs: config.Args,
		rootURI: config.RootURI,
		notify:  make(chan Notification, 100),
	}
}

// SetDiagnosticsCallback 设置诊断回调
func (c *Client) SetDiagnosticsCallback(fn func(params PublishDiagnosticsParams)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onDiagnostics = fn
}

func (c *Client) CommandLine() string {
	if c == nil {
		return ""
	}
	cmd := strings.TrimSpace(c.cmdPath)
	if len(c.cmdArgs) > 0 {
		cmd = strings.TrimSpace(cmd + " " + strings.Join(c.cmdArgs, " "))
	}
	return cmd
}

// Start 启动 LSP 服务器
func (c *Client) Start(ctx context.Context) error {
	if !c.started.CompareAndSwap(false, true) {
		return fmt.Errorf("client already started")
	}

	slog.Debug("lsp.client.starting",
		"component", utils.ComponentSystem,
		"command", c.cmdPath,
		"args", c.cmdArgs)

	// 启动服务器进程
	c.cmd = utils.CommandContext(ctx, c.cmdPath, c.cmdArgs...)

	// 创建管道
	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	c.stdin = stdin

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	c.stdout = stdout

	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	c.stderr = stderr

	// 启动进程
	if err := c.cmd.Start(); err != nil {
		c.started.Store(false)
		return fmt.Errorf("failed to start server: %w", err)
	}

	// 启动消息读取协程
	go c.readMessages()
	go c.handleNotifications()

	slog.Debug("lsp.client.started",
		"component", utils.ComponentSystem,
		"command", c.cmdPath)

	return nil
}

// Initialize 初始化 LSP 服务器
func (c *Client) Initialize(ctx context.Context) error {
	params := InitializeParams{
		ProcessID: os.Getpid(),
		RootURI:   c.rootURI,
		Capabilities: ClientCapabilities{
			TextDocument: TextDocumentClientCapabilities{
				Diagnostic: DiagnosticCapabilities{
					DynamicRegistration: true,
				},
			},
		},
	}

	var result InitializeResult
	if err := c.Call(ctx, "initialize", params, &result); err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	// 发送 initialized 通知
	return c.Notify(ctx, "initialized", nil)
}

// Call 发送请求并等待响应
func (c *Client) Call(ctx context.Context, method string, params any, result any) error {
	id := c.nextID.Add(1)

	req := map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      id,
		"method":  method,
	}

	if params != nil {
		paramsBytes, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
		req["params"] = json.RawMessage(paramsBytes)
	}

	// 发送请求
	if err := c.writeMessage(req); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	// 等待响应（简化实现，实际应该使用 channel 匹配）
	// 这里使用轮询方式读取响应
	// TODO: 实现更高效的响应匹配机制

	slog.Debug("lsp.client.call_sent",
		"component", utils.ComponentSystem,
		"method", method,
		"id", id)

	return nil
}

// Notify 发送通知（不等待响应）
func (c *Client) Notify(ctx context.Context, method string, params any) error {
	req := map[string]any{
		"jsonrpc": JSONRPCVersion,
		"method":  method,
	}

	if params != nil {
		paramsBytes, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
		req["params"] = json.RawMessage(paramsBytes)
	}

	return c.writeMessage(req)
}

// writeMessage 写入 JSON-RPC 消息
func (c *Client) writeMessage(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := c.stdin.Write([]byte(header + string(data))); err != nil {
		return err
	}

	return nil
}

// readMessages 读取服务器消息
func (c *Client) readMessages() {
	defer func() { _ = c.Stop() }()

	scanner := bufio.NewScanner(c.stdout)
	for scanner.Scan() {
		line := scanner.Text()

		// 解析 Content-Length 头
		var length int
		if _, err := fmt.Sscanf(line, "Content-Length: %d", &length); err != nil {
			continue
		}

		// 跳过空行
		scanner.Scan()

		// 读取消息体
		data := make([]byte, length)
		if n, err := io.ReadFull(c.stdout, data); err != nil || n != length {
			slog.Error("lsp.client.read_message.error",
				"component", utils.ComponentSystem,
				"error", err)
			break
		}

		// 处理消息
		c.handleMessage(data)
	}

	if err := scanner.Err(); err != nil {
		slog.Error("lsp.client.scan_error",
			"component", utils.ComponentSystem,
			"error", err)
	}
}

// handleMessage 处理单条消息
func (c *Client) handleMessage(data []byte) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		slog.Error("lsp.client.unmarshal.error",
			"component", utils.ComponentSystem,
			"data", string(data),
			"error", err)
		return
	}

	// 处理通知
	if msg.Method != "" && msg.ID == nil {
		// 检查是否是诊断通知
		if msg.Method == "textDocument/publishDiagnostics" {
			var params PublishDiagnosticsParams
			if err := json.Unmarshal(msg.Params, &params); err == nil {
				c.mu.RLock()
				if c.onDiagnostics != nil {
					c.onDiagnostics(params)
				}
				c.mu.RUnlock()
			}
		}

		// 发送到通知通道
		select {
		case c.notify <- Notification{
			Method: msg.Method,
			Params: msg.Params,
		}:
		default:
			slog.Warn("lsp.client.notify_channel_full",
				"component", utils.ComponentSystem,
				"method", msg.Method)
		}
	}
}

// handleNotifications 处理服务器通知
func (c *Client) handleNotifications() {
	for notif := range c.notify {
		switch notif.Method {
		case "textDocument/publishDiagnostics":
			// 已在 handleMessage 中处理
		case "window/logMessage":
			slog.Debug("lsp.server.log",
				"component", utils.ComponentSystem,
				"message", string(notif.Params))
		case "window/showMessage":
			slog.Debug("lsp.server.show_message",
				"component", utils.ComponentSystem,
				"message", string(notif.Params))
		}
	}
}

// DidOpen 打开文档
func (c *Client) DidOpen(ctx context.Context, uri, languageID, text string) error {
	params := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: languageID,
			Version:    1,
			Text:       text,
		},
	}
	return c.Notify(ctx, "textDocument/didOpen", params)
}

// DidChange 修改文档
func (c *Client) DidChange(ctx context.Context, uri string, version int, text string) error {
	params := DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     uri,
			Version: version,
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			{
				Text: text,
			},
		},
	}
	return c.Notify(ctx, "textDocument/didChange", params)
}

// Stop 停止客户端
func (c *Client) Stop() error {
	if !c.stopped.CompareAndSwap(false, true) {
		return nil
	}

	close(c.notify)

	if c.cmd != nil && c.cmd.Process != nil {
		// 发送 shutdown 请求
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.Call(ctx, "shutdown", nil, nil)

		// 发送 exit 通知
		_ = c.Notify(ctx, "exit", nil)

		// 等待进程结束或强制终止
		done := make(chan error, 1)
		go func() {
			done <- c.cmd.Wait()
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = c.cmd.Process.Kill()
		}
	}

	slog.Debug("lsp.client.stopped",
		"component", utils.ComponentSystem)

	return nil
}

// IsRunning 检查是否正在运行
func (c *Client) IsRunning() bool {
	return c.started.Load() && !c.stopped.Load()
}
