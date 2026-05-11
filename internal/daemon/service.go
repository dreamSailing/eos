package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/gateway"
	"github.com/dreamSailing/eos/internal/mcp"
	"github.com/dreamSailing/eos/internal/scheduler"
	"github.com/dreamSailing/eos/internal/toolapi"
	toolapiimpl "github.com/dreamSailing/eos/internal/toolapi/impl"
)

type Options struct {
	Workspace        string
	ListenAddr       string
	BaseURL          string
	AllowedTools     []string
	AccessMode       string
	ApprovalMode     string
	SandboxMode      string
	PolicyPath       string
	SessionStorePath string
	StateFile        string
	ScheduleFile     string
	LogFile          string
	MCPBasePath      string
}

type Service struct {
	opts      Options
	lock      *FileLock
	mcpServer *mcp.Server
	scheduler *scheduler.Service
	gateway   *gateway.Server
	services  toolapi.Services
	state     State
}

func normalizeOptions(opts Options) (Options, error) {
	if strings.TrimSpace(opts.Workspace) == "" {
		opts.Workspace = config.DefaultWorkspacePath()
	}
	workspace, err := config.ResolveWorkspacePath(opts.Workspace)
	if err != nil {
		return Options{}, err
	}
	opts.Workspace = workspace
	if err := os.MkdirAll(opts.Workspace, 0o755); err != nil {
		return Options{}, err
	}
	if strings.TrimSpace(opts.ListenAddr) == "" {
		opts.ListenAddr = "127.0.0.1:8765"
	}
	if strings.TrimSpace(opts.StateFile) == "" {
		opts.StateFile = DefaultStateFile()
	}
	if strings.TrimSpace(opts.ScheduleFile) == "" {
		opts.ScheduleFile = DefaultScheduleFile()
	}
	if strings.TrimSpace(opts.LogFile) == "" {
		opts.LogFile = DefaultLogFile()
	}
	if strings.TrimSpace(opts.MCPBasePath) == "" {
		opts.MCPBasePath = "/mcp"
	}
	if strings.TrimSpace(opts.SessionStorePath) == "" {
		opts.SessionStorePath = filepath.Join(opts.Workspace, ".eos", "serve", "sessions.json")
	}
	if strings.TrimSpace(opts.SandboxMode) == "" {
		opts.SandboxMode = "workspace"
	}
	return opts, nil
}

func NewService(opts Options) (*Service, error) {
	normalized, err := normalizeOptions(opts)
	if err != nil {
		return nil, err
	}
	lock, err := AcquireLock(filepath.Join(filepath.Dir(normalized.StateFile), "daemon.lock"))
	if err != nil {
		return nil, err
	}
	services := toolapiimpl.NewServices()
	mcpServer, err := mcp.NewServer(mcp.ServerOptions{
		Transport:             "sse",
		DefaultWorkspacePath:  normalized.Workspace,
		DefaultAllowedTools:   normalized.AllowedTools,
		DefaultAccessMode:     normalized.AccessMode,
		DefaultApprovalMode:   normalized.ApprovalMode,
		DefaultSandboxMode:    normalized.SandboxMode,
		PolicyPath:            normalized.PolicyPath,
		SessionStorePath:      normalized.SessionStorePath,
		RequireApprovalDigest: true,
		ListenAddr:            normalized.ListenAddr,
		BaseURL:               normalized.BaseURL,
	}, services)
	if err != nil {
		lock.Close()
		return nil, err
	}
	schedulerSvc := scheduler.NewService(normalized.ScheduleFile, services, normalized.Workspace)
	svc := &Service{
		opts:      normalized,
		lock:      lock,
		mcpServer: mcpServer,
		scheduler: schedulerSvc,
		services:  services,
	}
	gatewayServer := gateway.NewServer(gateway.Options{
		ListenAddr: normalized.ListenAddr,
		BaseURL:    normalized.BaseURL,
		MCPBasePath: normalized.MCPBasePath,
	}, &gateway.Context{
		MCP:       mcpServer,
		Scheduler: schedulerSvc,
		Services:  services,
		StatusSnapshot: svc.statusSnapshot,
	})
	svc.gateway = gatewayServer
	return svc, nil
}

func (s *Service) Start(ctx context.Context) error {
	if err := s.scheduler.Start(ctx); err != nil {
		return err
	}
	if err := s.gateway.Start(); err != nil {
		return err
	}
	s.state = State{
		PID:              os.Getpid(),
		StartedAt:        time.Now(),
		ListenAddr:       s.opts.ListenAddr,
		Workspace:        s.opts.Workspace,
		SessionStorePath: s.opts.SessionStorePath,
		SchedulePath:     s.opts.ScheduleFile,
		MCPBasePath:      strings.TrimRight(s.opts.MCPBasePath, "/") + "/sse",
		MCPMessagePath:   strings.TrimRight(s.opts.MCPBasePath, "/") + "/message",
		WebBaseURL:       s.gateway.BaseURL(),
		LogFile:          s.opts.LogFile,
	}
	if err := SaveState(s.opts.StateFile, s.state); err != nil {
		return err
	}
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Shutdown(shutdownCtx)
	return ctx.Err()
}

func (s *Service) Shutdown(ctx context.Context) error {
	if s.scheduler != nil {
		s.scheduler.Close()
	}
	if s.gateway != nil {
		_ = s.gateway.Shutdown(ctx)
	}
	if s.lock != nil {
		_ = s.lock.Close()
	}
	_ = RemoveState(s.opts.StateFile)
	return nil
}

func (s *Service) statusSnapshot() map[string]any {
	payload := map[string]any{
		"daemon": map[string]any{
			"pid":           s.state.PID,
			"started_at":    s.state.StartedAt,
			"listen_addr":   s.opts.ListenAddr,
			"workspace":     s.opts.Workspace,
			"web_base_url":  s.gateway.BaseURL(),
			"mcp_sse_url":   s.gateway.BaseURL() + strings.TrimRight(s.opts.MCPBasePath, "/") + "/sse",
			"schedule_file": s.opts.ScheduleFile,
			"log_file":      s.opts.LogFile,
		},
	}
	if s.mcpServer != nil {
		payload["mcp"] = s.mcpServer.StatusSnapshot()
	}
	if s.scheduler != nil {
		payload["scheduler"] = map[string]any{"count": len(s.scheduler.List())}
	}
	return payload
}

func EnsureDefaults(opts *Options) error {
	if opts == nil {
		return fmt.Errorf("options required")
	}
	if strings.TrimSpace(opts.Workspace) == "" {
		opts.Workspace = config.DefaultWorkspacePath()
	}
	return config.EnsureDefaultWorkspaceDir()
}
