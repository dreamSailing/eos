package serve

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dreamSailing/eos/internal/toolapi"
	"github.com/dreamSailing/eos/internal/version"
	"github.com/dreamSailing/eos/pkg/coreapi"
)

const (
	serveProtocolVersion = "1.0"
	bridgeSchemaVersion  = "1.0"
)

type BridgeManifest struct {
	SchemaVersion      string                   `json:"schemaVersion"`
	Name               string                   `json:"name"`
	Version            string                   `json:"version"`
	BuildCommit        string                   `json:"buildCommit,omitempty"`
	BuildDate          string                   `json:"buildDate,omitempty"`
	ProtocolVersion    string                   `json:"protocolVersion"`
	Transport          string                   `json:"transport"`
	Launch             BridgeLaunchSpec         `json:"launch"`
	SessionDefaults    BridgeSessionDefaults    `json:"sessionDefaults"`
	ExecutionModes     []executionModeDTO       `json:"executionModes,omitempty"`
	AccessModes        []accessModeDTO          `json:"accessModes,omitempty"`
	ApprovalModes      []approvalModeDTO        `json:"approvalModes,omitempty"`
	ServerCapabilities BridgeServerCapabilities `json:"serverCapabilities"`
	Methods            []string                 `json:"methods"`
	Tools              []toolDefinitionDTO      `json:"tools,omitempty"`
	Capabilities       []toolDefinitionDTO      `json:"capabilities,omitempty"`
}

type BridgeLaunchSpec struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Cwd     string            `json:"cwd,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type BridgeSessionDefaults struct {
	WorkspacePath          string   `json:"workspacePath"`
	AllowedTools           []string `json:"allowedTools,omitempty"`
	ExecutionMode          string   `json:"executionMode"`
	AccessMode             string   `json:"accessMode"`
	ApprovalMode           string   `json:"approvalMode"`
	SandboxMode            string   `json:"sandboxMode"`
	RequireApprovalDigest  bool     `json:"requireApprovalDigest"`
	MaxConcurrentToolCalls int      `json:"maxConcurrentToolCalls"`
}

type BridgeServerCapabilities struct {
	Events            bool `json:"events"`
	Invoke            bool `json:"invoke"`
	Tools             bool `json:"tools"`
	Confirmations     bool `json:"confirmations"`
	Sessions          bool `json:"sessions"`
	Requests          bool `json:"requests"`
	Tasks             bool `json:"tasks"`
	CapabilityCatalog bool `json:"capabilityCatalog"`
}

type BridgeManifestOptions struct {
	LaunchCommand       string
	WorkingDirectory    string
	Services            toolapi.Services
	ToolCatalogService  coreapi.ToolCatalogService
	IncludeTools        bool
	IncludeCapabilities bool
}

func BuildBridgeManifest(opts Options, manifestOpts BridgeManifestOptions) (BridgeManifest, error) {
	workspacePath := strings.TrimSpace(opts.DefaultWorkspacePath)
	if workspacePath == "" {
		return BridgeManifest{}, fmt.Errorf("workspace required")
	}
	workspaceAbs, err := filepath.Abs(workspacePath)
	if err != nil {
		return BridgeManifest{}, err
	}

	transport := normalizeBridgeTransport(opts.Transport)
	allowedTools := normalizeAllowedTools(opts.DefaultAllowedTools)
	workingDirectory := strings.TrimSpace(manifestOpts.WorkingDirectory)
	if workingDirectory == "" {
		workingDirectory = workspaceAbs
	}
	launchCommand := strings.TrimSpace(manifestOpts.LaunchCommand)
	if launchCommand == "" {
		if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
			launchCommand = exe
		} else {
			launchCommand = "eos"
		}
	}

	args := []string{
		"serve",
		"--transport", transport,
		"--workspace", workspaceAbs,
		fmt.Sprintf("--require-approval-digest=%t", opts.RequireApprovalDigest),
	}
	if len(allowedTools) > 0 {
		args = append(args, "--allowed-tools", strings.Join(allowedTools, ","))
	}
	if policyPath := strings.TrimSpace(opts.PolicyPath); policyPath != "" {
		policyAbs, err := filepath.Abs(policyPath)
		if err != nil {
			return BridgeManifest{}, err
		}
		args = append(args, "--policy", policyAbs)
	}
	accessMode := toolapi.NormalizeAccessMode(opts.DefaultAccessMode)
	if strings.TrimSpace(opts.DefaultAccessMode) == "" {
		accessMode = toolapi.ResolveAccessMode(toolapi.ExecSession{SandboxMode: opts.DefaultSandboxMode})
	}
	approvalMode := toolapi.NormalizeApprovalMode(opts.DefaultApprovalMode)
	if strings.TrimSpace(opts.DefaultApprovalMode) == "" {
		approvalMode = toolapi.ResolveApprovalMode(toolapi.ExecSession{RequireApprovalDigest: opts.RequireApprovalDigest})
	}
	if strings.TrimSpace(opts.DefaultAccessMode) != "" {
		args = append(args, "--access-mode", accessMode)
	}
	if strings.TrimSpace(opts.DefaultApprovalMode) != "" {
		args = append(args, "--approval-mode", approvalMode)
	}
	args = append(args, "--sandbox-mode", toolapi.NormalizeSandboxMode(opts.DefaultSandboxMode))

	manifest := BridgeManifest{
		SchemaVersion:   bridgeSchemaVersion,
		Name:            "eos-stdio-bridge",
		Version:         version.AppVersion,
		BuildCommit:     version.BuildCommit,
		BuildDate:       version.BuildDate,
		ProtocolVersion: serveProtocolVersion,
		Transport:       transport,
		Launch: BridgeLaunchSpec{
			Command: launchCommand,
			Args:    args,
			Cwd:     workingDirectory,
		},
		SessionDefaults: BridgeSessionDefaults{
			WorkspacePath:          workspaceAbs,
			AllowedTools:           append([]string(nil), allowedTools...),
			ExecutionMode:          "auto",
			AccessMode:             accessMode,
			ApprovalMode:           approvalMode,
			SandboxMode:            toolapi.NormalizeSandboxMode(opts.DefaultSandboxMode),
			RequireApprovalDigest:  opts.RequireApprovalDigest,
			MaxConcurrentToolCalls: 1,
		},
		ExecutionModes:     modeDTOs(toolapi.SupportedExecutionModes()),
		AccessModes:        accessModeDTOs(toolapi.SupportedAccessModes()),
		ApprovalModes:      approvalModeDTOs(toolapi.SupportedApprovalModes()),
		ServerCapabilities: serverCapabilitiesSummary(),
		Methods:            supportedRPCMethods(),
	}

	if !manifestOpts.IncludeTools && !manifestOpts.IncludeCapabilities {
		return manifest, nil
	}
	if manifestOpts.ToolCatalogService == nil && manifestOpts.Services == nil {
		return manifest, nil
	}

	var defs []toolapi.ToolDefinition
	if manifestOpts.ToolCatalogService != nil {
		coreDefs, listErr := manifestOpts.ToolCatalogService.List(context.Background(), coreapi.ListToolCatalogRequest{
			WorkspaceRoot: workspaceAbs,
		})
		if listErr != nil {
			return manifest, nil
		}
		defs = coreapiDefsToToolAPIDefs(coreDefs)
	} else {
		ctx := contextWithWorkspaceRoot(context.Background(), workspaceAbs)
		var listErr error
		defs, listErr = manifestOpts.Services.Catalog().List(ctx)
		if listErr != nil {
			return manifest, nil
		}
	}

	sess := toolapi.ExecSession{
		WorkspaceRoot:         workspaceAbs,
		AllowedTools:          allowedToolsMap(allowedTools),
		ExecutionMode:         manifest.SessionDefaults.ExecutionMode,
		AccessMode:            manifest.SessionDefaults.AccessMode,
		ApprovalMode:          manifest.SessionDefaults.ApprovalMode,
		SandboxMode:           manifest.SessionDefaults.SandboxMode,
		RequireApprovalDigest: manifest.SessionDefaults.RequireApprovalDigest,
	}
	if manifestOpts.IncludeTools {
		manifest.Tools = defsToDTOsForSession(toolapi.FilterVisibleTools(defs, sess), sess)
	}
	if manifestOpts.IncludeCapabilities {
		manifest.Capabilities = defsToDTOsForSession(toolapi.FilterVisibleCapabilities(defs, sess), sess)
	}
	return manifest, nil
}

func serverCapabilitiesSummary() BridgeServerCapabilities {
	return BridgeServerCapabilities{
		Events:            true,
		Invoke:            false,
		Tools:             true,
		Confirmations:     true,
		Sessions:          true,
		Requests:          true,
		Tasks:             true,
		CapabilityCatalog: true,
	}
}

func serverCapabilitiesPayload() map[string]any {
	caps := serverCapabilitiesSummary()
	return map[string]any{
		"events":            caps.Events,
		"invoke":            caps.Invoke,
		"tools":             caps.Tools,
		"confirmations":     caps.Confirmations,
		"sessions":          caps.Sessions,
		"requests":          caps.Requests,
		"tasks":             caps.Tasks,
		"capabilityCatalog": caps.CapabilityCatalog,
	}
}

func supportedRPCMethods() []string {
	return []string{
		"initialize",
		"session.create",
		"session.get",
		"session.list",
		"session.resume",
		"session.close",
		"session.delete",
		"request.start",
		"request.cancel",
		"tool.list",
		"capability.list",
		"tool.preflight",
		"prompt.resolve",
		"approval.resolve",
		"inquiry.resolve",
		"tool.execute",
		"tool.cancel",
		"task.list",
		"task.resume",
		"task.cancel",
		"task.kill",
		"task.close",
	}
}

func normalizeBridgeTransport(transport string) string {
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
	slices.SortFunc(out, func(a, b string) int {
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	})
	return out
}

func allowedToolsMap(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		out[value] = true
	}
	return out
}

func coreapiDefsToToolAPIDefs(coreDefs []coreapi.ToolDefinition) []toolapi.ToolDefinition {
	out := make([]toolapi.ToolDefinition, 0, len(coreDefs))
	for _, d := range coreDefs {
		params := make(map[string]toolapi.ParameterInfo, len(d.Params))
		for k, v := range d.Params {
			params[k] = toolapi.ParameterInfo{
				Type:     v.Type,
				Required: v.Required,
				Desc:     v.Desc,
			}
		}
		examples := make([]toolapi.ToolExample, 0, len(d.Examples))
		for _, ex := range d.Examples {
			examples = append(examples, toolapi.ToolExample{
				Description: ex.Description,
				Input:       ex.Input,
			})
		}
		out = append(out, toolapi.ToolDefinition{
			Name:               d.Name,
			Description:        d.Description,
			RiskLevel:          toolapi.RiskLevel(d.RiskLevel),
			Params:             params,
			Examples:           examples,
			Source:             toolapi.CapabilitySource(d.Source),
			Category:           d.Category,
			VisibleIn:          append([]string(nil), d.VisibleIn...),
			ReadOnly:           d.ReadOnly,
			Invocable:          d.Invocable,
			RequiresFullAccess: d.RequiresFullAccess,
			Tags:               append([]string(nil), d.Tags...),
			Metadata:           d.Metadata,
		})
	}
	return out
}
