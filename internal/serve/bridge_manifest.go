package serve

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dreamSailing/vb-coding/internal/toolapi"
	"github.com/dreamSailing/vb-coding/internal/tools"
	"github.com/dreamSailing/vb-coding/internal/version"
)

const (
	serveProtocolVersion = "1.0"
	bridgeSchemaVersion  = "1.0"
)

type BridgeManifest struct {
	SchemaVersion      string                   `json:"schemaVersion"`
	Name               string                   `json:"name"`
	Version            string                   `json:"version"`
	ProtocolVersion    string                   `json:"protocolVersion"`
	Transport          string                   `json:"transport"`
	Launch             BridgeLaunchSpec         `json:"launch"`
	SessionDefaults    BridgeSessionDefaults    `json:"sessionDefaults"`
	ExecutionModes     []executionModeDTO       `json:"executionModes,omitempty"`
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
			launchCommand = "vb-coding"
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

	manifest := BridgeManifest{
		SchemaVersion:   bridgeSchemaVersion,
		Name:            "vb-coding-stdio-bridge",
		Version:         version.AppVersion,
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
			RequireApprovalDigest:  opts.RequireApprovalDigest,
			MaxConcurrentToolCalls: 1,
		},
		ExecutionModes:     modeDTOs(toolapi.SupportedExecutionModes()),
		ServerCapabilities: serverCapabilitiesSummary(),
		Methods:            supportedRPCMethods(),
	}

	if manifestOpts.Services == nil || (!manifestOpts.IncludeTools && !manifestOpts.IncludeCapabilities) {
		return manifest, nil
	}

	ctx := tools.WithWorkspaceRoot(context.Background(), workspaceAbs)
	defs, err := manifestOpts.Services.Catalog().List(ctx)
	if err != nil {
		return manifest, nil
	}

	sess := toolapi.ExecSession{
		WorkspaceRoot:         workspaceAbs,
		AllowedTools:          allowedToolsMap(allowedTools),
		ExecutionMode:         manifest.SessionDefaults.ExecutionMode,
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
