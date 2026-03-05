// Package mcp 提供 MCP (Model Context Protocol) 服务管理和工具集成
package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"github.com/dreamSailing/vb-coding/internal/config"
	"github.com/dreamSailing/vb-coding/internal/pkg/utils"

	"github.com/cloudwego/eino/components/tool"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	einomcp "github.com/cloudwego/eino-ext/components/tool/mcp"
)

// Manager MCP 服务管理器，管理多个 MCP 客户端并提供工具集成
type Manager struct {
	clients map[string]mcpclient.MCPClient // name -> client
	tools   map[string][]tool.BaseTool     // name -> tools
	status  map[string]ServerStatus        // name -> status (last reload)
	mu      sync.RWMutex
}

type ServerStatus struct {
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Loaded    bool   `json:"loaded"`
	Tools     int    `json:"tools"`
	LastError string `json:"last_error,omitempty"`
}

// NewManager 创建 MCP 管理器
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]mcpclient.MCPClient),
		tools:   make(map[string][]tool.BaseTool),
		status:  make(map[string]ServerStatus),
	}
}

// LoadFromConfig 从配置加载并初始化 MCP 服务
func (m *Manager) LoadFromConfig(ctx context.Context, cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	servers := config.GetEnabledMCPServers(cfg)
	slog.Info("mcp.manager.load_from_config.start", "component", utils.ComponentSystem, "servers_count", len(servers))

	m.mu.Lock()
	m.status = make(map[string]ServerStatus)
	m.mu.Unlock()

	for _, server := range servers {
		if err := m.loadServer(ctx, server); err != nil {
			m.mu.Lock()
			m.status[server.Name] = ServerStatus{
				Name:      server.Name,
				Enabled:   true,
				Loaded:    false,
				Tools:     0,
				LastError: err.Error(),
			}
			m.mu.Unlock()
			slog.Error("mcp.manager.load_server.error", "component", utils.ComponentSystem, "name", server.Name, "type", server.Type, "error", err.Error())
			// 继续加载其他服务
			continue
		}
		m.mu.Lock()
		m.status[server.Name] = ServerStatus{
			Name:    server.Name,
			Enabled: true,
			Loaded:  true,
			Tools:   len(m.tools[server.Name]),
		}
		m.mu.Unlock()
	}

	slog.Info("mcp.manager.load_from_config.complete", "component", utils.ComponentSystem, "loaded_count", len(m.clients))
	return nil
}

// loadServer 加载单个 MCP 服务
func (m *Manager) loadServer(ctx context.Context, server config.MCPEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 根据类型创建客户端
	var cli mcpclient.MCPClient
	var err error

	switch server.Type {
	case config.MCPTypeStdio:
		envSlice := make([]string, 0, len(server.Envs))
		for k, v := range server.Envs {
			envSlice = append(envSlice, k+"="+v)
		}
		cli, err = mcpclient.NewStdioMCPClient(server.Command, envSlice, server.Args...)
		if err != nil {
			slog.Error("mcp.manager.create_stdio_client.error", "component", utils.ComponentSystem, "name", server.Name, "error", err.Error())
			return fmt.Errorf("create_stdio_client: %w", err)
		}
	case config.MCPTypeSSE:
		sseCli, err := mcpclient.NewSSEMCPClient(server.BaseURL)
		if err != nil {
			slog.Error("mcp.manager.create_sse_client.error", "component", utils.ComponentSystem, "name", server.Name, "error", err.Error())
			return fmt.Errorf("create_sse_client: %w", err)
		}
		// SSE 客户端需要启动
		if err := sseCli.Start(ctx); err != nil {
			slog.Error("mcp.manager.start_sse_client.error", "component", utils.ComponentSystem, "name", server.Name, "error", err.Error())
			return fmt.Errorf("start_sse_client: %w", err)
		}
		cli = sseCli
	default:
		slog.Warn("mcp.manager.unknown_server_type", "component", utils.ComponentSystem, "name", server.Name, "type", server.Type)
		return nil
	}

	// 初始化 MCP 客户端
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "vb-coding",
		Version: "1.0.0",
	}
	if _, err := cli.Initialize(ctx, initReq); err != nil {
		slog.Error("mcp.manager.initialize_client.error", "component", utils.ComponentSystem, "name", server.Name, "error", err.Error())
		return fmt.Errorf("initialize_client: %w", err)
	}

	// 使用 eino MCP 获取工具
	tools, err := einomcp.GetTools(ctx, &einomcp.Config{Cli: cli})
	if err != nil {
		slog.Error("mcp.manager.get_tools.error", "component", utils.ComponentSystem, "name", server.Name, "error", err.Error())
		return fmt.Errorf("get_tools: %w", err)
	}

	m.clients[server.Name] = cli
	m.tools[server.Name] = tools

	slog.Info("mcp.manager.load_server.success", "component", utils.ComponentSystem, "name", server.Name, "tools_count", len(tools))
	return nil
}

// GetAllTools 获取所有 MCP 工具
func (m *Manager) GetAllTools() []tool.BaseTool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var allTools []tool.BaseTool
	for _, tools := range m.tools {
		allTools = append(allTools, tools...)
	}
	return allTools
}

// GetToolsByServer 获取指定服务的工具
func (m *Manager) GetToolsByServer(name string) []tool.BaseTool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tools[name]
}

// ListServers 列出已加载的服务名称
func (m *Manager) ListServers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.clients))
	for name := range m.clients {
		names = append(names, name)
	}
	return names
}

// Reload 重新加载配置
func (m *Manager) Reload(ctx context.Context, cfg *config.Config) error {
	m.Close()
	return m.LoadFromConfig(ctx, cfg)
}

func (m *Manager) GetServerStatuses(cfg *config.Config) []ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var out []ServerStatus
	if cfg == nil {
		return out
	}
	for _, e := range cfg.MCP {
		s, ok := m.status[e.Name]
		if !ok {
			s = ServerStatus{Name: e.Name}
		}
		s.Enabled = e.Enabled
		out = append(out, s)
	}
	return out
}

// Close 关闭管理器，清理所有客户端连接
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name := range m.clients {
		delete(m.clients, name)
		delete(m.tools, name)
		delete(m.status, name)
	}
	slog.Info("mcp.manager.close.success", "component", utils.ComponentSystem)
}
