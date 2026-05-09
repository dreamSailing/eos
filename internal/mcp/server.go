package mcp

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dreamSailing/eos/internal/toolapi"
	"github.com/dreamSailing/eos/internal/version"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

type ServerOptions struct {
	Transport             string
	DefaultWorkspacePath  string
	DefaultAllowedTools   []string
	DefaultSandboxMode    string
	PolicyPath            string
	SessionStorePath      string
	RequireApprovalDigest bool
	ListenAddr            string
	BaseURL               string
}

type Server struct {
	opts     ServerOptions
	services toolapi.Services
	mcp      *mcpserver.MCPServer
	policy   *serverPolicy

	mu                   sync.RWMutex
	sessions             map[string]*runtimeSession
	defaultSessionByConn map[string]string
}

func NewServer(opts ServerOptions, services toolapi.Services) (*Server, error) {
	if services == nil {
		return nil, fmt.Errorf("tools service required")
	}
	workspace := strings.TrimSpace(opts.DefaultWorkspacePath)
	if workspace == "" {
		return nil, fmt.Errorf("workspace required")
	}
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	opts.DefaultWorkspacePath = workspaceAbs
	opts.Transport = normalizeServerTransport(opts.Transport)
	opts.DefaultSandboxMode = toolapi.NormalizeSandboxMode(opts.DefaultSandboxMode)
	if opts.DefaultSandboxMode == "" {
		opts.DefaultSandboxMode = "workspace"
	}
	opts.DefaultAllowedTools = normalizeAllowedTools(opts.DefaultAllowedTools)

	policy, err := loadServerPolicy(opts.PolicyPath)
	if err != nil {
		return nil, err
	}

	base := mcpserver.NewMCPServer(
		"eos",
		version.AppVersion,
		mcpserver.WithInstructions("EOS MCP server exposes EOS tools, sessions, approvals, inquiries, tasks, and runtime resources."),
		mcpserver.WithToolCapabilities(true),
		mcpserver.WithResourceCapabilities(false, true),
		mcpserver.WithRecovery(),
		mcpserver.WithResourceRecovery(),
	)

	s := &Server{
		opts:                 opts,
		services:             services,
		mcp:                  base,
		policy:               policy,
		sessions:             map[string]*runtimeSession{},
		defaultSessionByConn: map[string]string{},
	}
	s.registerTools()
	s.registerResources()
	return s, nil
}

func normalizeServerTransport(transport string) string {
	transport = strings.TrimSpace(strings.ToLower(transport))
	if transport == "" {
		return "stdio"
	}
	return transport
}

func normalizeAllowedTools(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func allowedToolsMap(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]bool, len(values))
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			continue
		}
		out[key] = true
	}
	return out
}

func (s *Server) contextWithRequestMeta(ctx context.Context, header http.Header) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, serverHeaderKey{}, header)
}

func (s *Server) Run(ctx context.Context) error {
	switch normalizeServerTransport(s.opts.Transport) {
	case "stdio":
		return s.RunStdio(ctx, nil, nil, nil)
	case "sse":
		return s.RunSSE(ctx)
	default:
		return fmt.Errorf("unsupported transport: %s", s.opts.Transport)
	}
}
