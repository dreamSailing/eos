package mcp

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/lsp"
	pluginpkg "github.com/dreamSailing/eos/internal/pkg/plugins"
	"github.com/dreamSailing/eos/internal/toolapi"
	"github.com/dreamSailing/eos/internal/version"
	mcpmodel "github.com/mark3labs/mcp-go/mcp"
)

const (
	resourceSessions        = "eos://sessions"
	resourceCatalogTools    = "eos://catalog/tools"
	resourceCatalogCaps     = "eos://catalog/capabilities"
	resourceRuntimeMCP      = "eos://runtime/mcp-status"
	resourceRuntimeLSP      = "eos://runtime/lsp-status"
	resourceRuntimeVersion  = "eos://runtime/version"
	sessionResourceTemplate = "eos://sessions/{id}"
	approvalTemplate        = "eos://sessions/{id}/approvals"
	inquiryTemplate         = "eos://sessions/{id}/inquiries"
	taskTemplate            = "eos://sessions/{id}/tasks"
)

func (s *Server) registerResources() {
	s.mcp.AddResource(
		mcpmodel.NewResource(resourceSessions, "sessions", mcpmodel.WithResourceDescription("EOS sessions"), mcpmodel.WithMIMEType("application/json")),
		func(ctx context.Context, request mcpmodel.ReadResourceRequest) ([]mcpmodel.ResourceContents, error) {
			items := s.listSessions()
			out := make([]map[string]any, 0, len(items))
			for _, item := range items {
				out = append(out, item.snapshot())
			}
			return makeTextResource(resourceSessions, map[string]any{"sessions": out})
		},
	)
	s.mcp.AddResource(
		mcpmodel.NewResource(resourceCatalogTools, "catalog-tools", mcpmodel.WithResourceDescription("EOS executable tool catalog"), mcpmodel.WithMIMEType("application/json")),
		func(ctx context.Context, request mcpmodel.ReadResourceRequest) ([]mcpmodel.ResourceContents, error) {
			sess, _ := s.ensureDefaultSession(ctx)
			return makeTextResource(resourceCatalogTools, map[string]any{"tools": s.toolCatalogSnapshot(sess, true)})
		},
	)
	s.mcp.AddResource(
		mcpmodel.NewResource(resourceCatalogCaps, "catalog-capabilities", mcpmodel.WithResourceDescription("EOS capability catalog"), mcpmodel.WithMIMEType("application/json")),
		func(ctx context.Context, request mcpmodel.ReadResourceRequest) ([]mcpmodel.ResourceContents, error) {
			sess, _ := s.ensureDefaultSession(ctx)
			return makeTextResource(resourceCatalogCaps, map[string]any{"capabilities": s.toolCatalogSnapshot(sess, false)})
		},
	)
	s.mcp.AddResource(
		mcpmodel.NewResource(resourceRuntimeVersion, "runtime-version", mcpmodel.WithResourceDescription("EOS version information"), mcpmodel.WithMIMEType("application/json")),
		func(ctx context.Context, request mcpmodel.ReadResourceRequest) ([]mcpmodel.ResourceContents, error) {
			return makeTextResource(resourceRuntimeVersion, map[string]any{
				"name":         version.AppName,
				"version":      version.AppVersion,
				"build_commit": version.BuildCommit,
				"build_date":   version.BuildDate,
			})
		},
	)
	s.mcp.AddResource(
		mcpmodel.NewResource(resourceRuntimeMCP, "runtime-mcp-status", mcpmodel.WithResourceDescription("Configured MCP client status inside EOS"), mcpmodel.WithMIMEType("application/json")),
		func(ctx context.Context, request mcpmodel.ReadResourceRequest) ([]mcpmodel.ResourceContents, error) {
			payload := s.mcpStatusSnapshot()
			return makeTextResource(resourceRuntimeMCP, payload)
		},
	)
	s.mcp.AddResource(
		mcpmodel.NewResource(resourceRuntimeLSP, "runtime-lsp-status", mcpmodel.WithResourceDescription("Current LSP detection summary"), mcpmodel.WithMIMEType("application/json")),
		func(ctx context.Context, request mcpmodel.ReadResourceRequest) ([]mcpmodel.ResourceContents, error) {
			payload := s.lspStatusSnapshot()
			return makeTextResource(resourceRuntimeLSP, payload)
		},
	)

	s.mcp.AddResourceTemplate(
		mcpmodel.NewResourceTemplate(sessionResourceTemplate, "session", mcpmodel.WithTemplateDescription("EOS session details"), mcpmodel.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, request mcpmodel.ReadResourceRequest) ([]mcpmodel.ResourceContents, error) {
			sessionID := stringArg(request.Params.Arguments, "id")
			sess := s.getSession(sessionID)
			if sess == nil {
				return makeTextResource(request.Params.URI, map[string]any{"error": "session not found", "session_id": sessionID})
			}
			return makeTextResource(request.Params.URI, map[string]any{"session": sess.snapshot()})
		},
	)
	s.mcp.AddResourceTemplate(
		mcpmodel.NewResourceTemplate(approvalTemplate, "session-approvals", mcpmodel.WithTemplateDescription("Pending approvals for a session"), mcpmodel.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, request mcpmodel.ReadResourceRequest) ([]mcpmodel.ResourceContents, error) {
			sessionID := stringArg(request.Params.Arguments, "id")
			sess := s.getSession(sessionID)
			if sess == nil {
				return makeTextResource(request.Params.URI, map[string]any{"error": "session not found", "session_id": sessionID})
			}
			return makeTextResource(request.Params.URI, map[string]any{"session_id": sessionID, "approvals": sess.approvalList()})
		},
	)
	s.mcp.AddResourceTemplate(
		mcpmodel.NewResourceTemplate(inquiryTemplate, "session-inquiries", mcpmodel.WithTemplateDescription("Pending inquiries for a session"), mcpmodel.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, request mcpmodel.ReadResourceRequest) ([]mcpmodel.ResourceContents, error) {
			sessionID := stringArg(request.Params.Arguments, "id")
			sess := s.getSession(sessionID)
			if sess == nil {
				return makeTextResource(request.Params.URI, map[string]any{"error": "session not found", "session_id": sessionID})
			}
			return makeTextResource(request.Params.URI, map[string]any{"session_id": sessionID, "inquiries": sess.inquiryList()})
		},
	)
	s.mcp.AddResourceTemplate(
		mcpmodel.NewResourceTemplate(taskTemplate, "session-tasks", mcpmodel.WithTemplateDescription("Global EOS tasks visible to the current server"), mcpmodel.WithTemplateMIMEType("application/json")),
		func(ctx context.Context, request mcpmodel.ReadResourceRequest) ([]mcpmodel.ResourceContents, error) {
			sessionID := stringArg(request.Params.Arguments, "id")
			if s.getSession(sessionID) == nil {
				return makeTextResource(request.Params.URI, map[string]any{"error": "session not found", "session_id": sessionID})
			}
			items, err := s.services.Tasks().List(ctx)
			if err != nil {
				return makeTextResource(request.Params.URI, map[string]any{"error": err.Error(), "session_id": sessionID})
			}
			return makeTextResource(request.Params.URI, map[string]any{"session_id": sessionID, "tasks": taskInfoList(items)})
		},
	)
}

func (s *Server) toolCatalogSnapshot(sess *runtimeSession, executableOnly bool) []map[string]any {
	workspace := s.opts.DefaultWorkspacePath
	allowed := allowedToolsMap(s.opts.DefaultAllowedTools)
	executionMode := "auto"
	sandboxMode := s.opts.DefaultSandboxMode
	requireDigest := s.opts.RequireApprovalDigest
	if sess != nil {
		workspace = sess.WorkspaceAbs
		allowed = sess.AllowedTools
		executionMode = sess.ExecutionMode
		sandboxMode = sess.SandboxMode
		requireDigest = sess.RequireApprovalDigest
	}
	defs := s.currentToolDefinitions(workspace)
	execSess := toolapi.ExecSession{
		WorkspaceRoot:         workspace,
		AllowedTools:          allowed,
		ExecutionMode:         executionMode,
		SandboxMode:           sandboxMode,
		RequireApprovalDigest: requireDigest,
	}
	if executableOnly {
		defs = toolapi.FilterVisibleTools(defs, execSess)
	} else {
		defs = toolapi.FilterVisibleCapabilities(defs, execSess)
	}
	out := make([]map[string]any, 0, len(defs))
	for _, def := range defs {
		access := toolapi.EvaluateToolAccess(def, execSess)
		out = append(out, map[string]any{
			"name":                 def.Name,
			"description":          def.Description,
			"risk_level":           def.RiskLevel,
			"source":               def.Source,
			"category":             def.Category,
			"read_only":            def.ReadOnly,
			"invocable":            def.Invocable,
			"requires_full_access": def.RequiresFullAccess,
			"visible_in":           append([]string(nil), def.VisibleIn...),
			"tags":                 append([]string(nil), def.Tags...),
			"metadata":             def.Metadata,
			"access": map[string]any{
				"mode":           access.Mode,
				"visible":        access.Visible,
				"executable":     access.Executable,
				"needs_approval": access.NeedsApproval,
				"reason":         access.Reason,
			},
		})
	}
	return out
}

func (s *Server) mcpStatusSnapshot() map[string]any {
	cfg, _ := config.Load()
	merged := cfg
	merged.MCP = pluginpkg.MergeMCPEntries(&cfg, s.opts.DefaultWorkspacePath)
	manager := NewManager()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = manager.LoadFromConfig(ctx, &merged)
	statuses := manager.GetServerStatuses(&merged)
	out := make([]map[string]any, 0, len(statuses))
	for _, item := range statuses {
		out = append(out, map[string]any{
			"name":       item.Name,
			"enabled":    item.Enabled,
			"loaded":     item.Loaded,
			"tools":      item.Tools,
			"last_error": item.LastError,
		})
	}
	manager.Close()
	return map[string]any{"servers": out}
}

func (s *Server) lspStatusSnapshot() map[string]any {
	cfg, _ := config.Load()
	detector := lsp.NewDetector()
	detectedLanguage := ""
	if strings.TrimSpace(s.opts.DefaultWorkspacePath) != "" {
		detectedLanguage = string(detector.DetectLanguage(s.opts.DefaultWorkspacePath))
	}
	servers := make([]map[string]any, 0, 4)
	for _, lang := range []lsp.LanguageType{lsp.LanguageGo, lsp.LanguagePython, lsp.LanguageTypeScript, lsp.LanguageJavaScript} {
		item := map[string]any{"language": string(lang)}
		info, err := detector.FindServer(lang)
		if err != nil || info == nil {
			item["found"] = false
			item["command"] = "not found"
		} else {
			item["found"] = true
			command := strings.TrimSpace(info.Command)
			if len(info.Args) > 0 {
				command = strings.TrimSpace(command + " " + strings.Join(info.Args, " "))
			}
			item["command"] = command
		}
		servers = append(servers, item)
	}
	return map[string]any{
		"workspace":         s.opts.DefaultWorkspacePath,
		"enabled":           cfg.LSP.EnabledValue(),
		"auto_detect":       cfg.LSP.AutoDetectValue(),
		"detected_language": detectedLanguage,
		"servers":           servers,
	}
}
