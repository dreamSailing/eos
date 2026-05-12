package cli

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/dreamSailing/eos/internal/serve"
	toolapiimpl "github.com/dreamSailing/eos/internal/toolapi/impl"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var transport string
	var workspace string
	var allowedTools string
	var accessMode string
	var approvalMode string
	var sandboxMode string
	var policyPath string
	var sessionStorePath string
	var requireApprovalDigest bool
	var skipPermissions bool

	cmd := &cobra.Command{
		Use:    "serve",
		Short:  "Start eos as a local tool service (for agents/platform).",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			transport = strings.TrimSpace(transport)
			if transport == "" {
				transport = "stdio"
			}
			if transport != "stdio" {
				return fmt.Errorf("unsupported transport: %s", transport)
			}
			if strings.TrimSpace(workspace) == "" {
				return errors.New("workspace required")
			}
			modes := resolveModeConfig(accessMode, approvalMode, sandboxMode, skipPermissions, requireApprovalDigest)

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			s, err := serve.NewServer(serve.Options{
				Transport:             transport,
				DefaultWorkspacePath:  workspace,
				DefaultAllowedTools:   splitCommaList(allowedTools),
				DefaultAccessMode:     modes.AccessMode,
				DefaultApprovalMode:   modes.ApprovalMode,
				DefaultSandboxMode:    modes.SandboxMode,
				PolicyPath:            policyPath,
				SessionStorePath:      sessionStorePath,
				RequireApprovalDigest: requireApprovalDigest,
			}, os.Stdin, os.Stdout, os.Stderr, toolapiimpl.NewServices())
			if err != nil {
				return err
			}
			return s.Run(ctx)
		},
	}

	cmd.Flags().StringVar(&transport, "transport", "stdio", "transport: stdio")
	cmd.Flags().StringVar(&workspace, "workspace", "", "workspace path (required)")
	cmd.Flags().StringVar(&allowedTools, "allowed-tools", "", "comma-separated allowed tools (optional)")
	cmd.Flags().StringVar(&accessMode, "access-mode", "", "default access mode: read-only, workspace-write, or danger-full-access")
	cmd.Flags().StringVar(&approvalMode, "approval-mode", "", "default approval mode: untrusted, on-failure, on-request, or never")
	cmd.Flags().StringVar(&sandboxMode, "sandbox-mode", "workspace", "legacy sandbox mode alias: workspace or full_access")
	cmd.Flags().StringVar(&policyPath, "policy", "", "policy json file path (optional)")
	cmd.Flags().StringVar(&sessionStorePath, "session-store", "", "session store file path (optional)")
	cmd.Flags().BoolVar(&requireApprovalDigest, "require-approval-digest", true, "require approvalDigest for medium/high risk tools")
	cmd.Flags().BoolVar(&skipPermissions, "dangerously-skip-permissions", false, "compatibility alias for --access-mode danger-full-access --approval-mode never")

	_ = cmd.MarkFlagRequired("workspace")
	return cmd
}

func splitCommaList(s string) []string {
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, it := range raw {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		out = append(out, it)
	}
	return out
}
