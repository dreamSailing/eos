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

	eosmcp "github.com/dreamSailing/eos/internal/mcp"
	toolapiimpl "github.com/dreamSailing/eos/internal/toolapi/impl"
	"github.com/spf13/cobra"
)

func newMCPCmd() *cobra.Command {
	var transport string
	var workspace string
	var allowedTools string
	var sandboxMode string
	var policyPath string
	var sessionStorePath string
	var requireApprovalDigest bool
	var listenAddr string
	var baseURL string

	cmd := &cobra.Command{
		Use:    "mcp",
		Short:  "Run EOS as a standard MCP server.",
		Hidden: true,
	}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start EOS as an MCP server over stdio or SSE.",
		RunE: func(cmd *cobra.Command, args []string) error {
			transport = strings.TrimSpace(transport)
			if transport == "" {
				transport = "stdio"
			}
			if transport != "stdio" && transport != "sse" {
				return fmt.Errorf("unsupported transport: %s", transport)
			}
			if strings.TrimSpace(workspace) == "" {
				return errors.New("workspace required")
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			s, err := eosmcp.NewServer(eosmcp.ServerOptions{
				Transport:             transport,
				DefaultWorkspacePath:  workspace,
				DefaultAllowedTools:   splitCommaList(allowedTools),
				DefaultSandboxMode:    sandboxMode,
				PolicyPath:            policyPath,
				SessionStorePath:      sessionStorePath,
				RequireApprovalDigest: requireApprovalDigest,
				ListenAddr:            listenAddr,
				BaseURL:               baseURL,
			}, toolapiimpl.NewServices())
			if err != nil {
				return err
			}
			return s.Run(ctx)
		},
	}

	serveCmd.Flags().StringVar(&transport, "transport", "stdio", "MCP transport: stdio or sse")
	serveCmd.Flags().StringVar(&workspace, "workspace", "", "workspace path (required)")
	serveCmd.Flags().StringVar(&allowedTools, "allowed-tools", "", "comma-separated allowed tools (optional)")
	serveCmd.Flags().StringVar(&sandboxMode, "sandbox-mode", "workspace", "sandbox mode: workspace or full_access")
	serveCmd.Flags().StringVar(&policyPath, "policy", "", "policy json file path (optional)")
	serveCmd.Flags().StringVar(&sessionStorePath, "session-store", "", "session store file path (reserved)")
	serveCmd.Flags().BoolVar(&requireApprovalDigest, "require-approval-digest", true, "require approvals for medium/high risk tools")
	serveCmd.Flags().StringVar(&listenAddr, "listen", "127.0.0.1:8765", "listen address for SSE transport")
	serveCmd.Flags().StringVar(&baseURL, "base-url", "", "public base URL for SSE transport (optional)")
	_ = serveCmd.MarkFlagRequired("workspace")

	cmd.AddCommand(serveCmd)
	return cmd
}
