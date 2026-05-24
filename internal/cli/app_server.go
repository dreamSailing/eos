package cli

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/coder/websocket"

	sharedcore "github.com/dreamSailing/eos/pkg/core"
	"github.com/spf13/cobra"
)

func newAppServerCmd() *cobra.Command {
	var transport string
	var workspace string
	var addr string

	cmd := &cobra.Command{
		Use:    "app-server",
		Short:  "Start the Codex-aligned core JSON-RPC app server.",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			transport = strings.TrimSpace(transport)
			if transport == "" {
				transport = "stdio"
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			rt := sharedcore.NewRuntime()
			defer rt.Close()

			workspace = strings.TrimSpace(workspace)
			if workspace != "" {
				if err := rt.RememberWorkspace(workspace, true); err != nil {
					return err
				}
				if err := rt.SetForegroundWorkspace(workspace); err != nil {
					return err
				}
			}

			switch transport {
			case "stdio":
				return rt.ServeJSONRPCStream(ctx, os.Stdin, os.Stdout)
			case "ws":
				return serveWS(ctx, rt, addr)
			default:
				return fmt.Errorf("unsupported app-server transport: %s", transport)
			}
		},
	}

	cmd.Flags().StringVar(&transport, "transport", "stdio", "transport: stdio, ws")
	cmd.Flags().StringVar(&workspace, "workspace", "", "foreground workspace path (optional)")
	cmd.Flags().StringVar(&addr, "addr", "localhost:8080", "listen address for ws transport")
	return cmd
}

func serveWS(ctx context.Context, rt *sharedcore.Runtime, addr string) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = "localhost:8080"
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	defer ln.Close()

	fmt.Fprintf(os.Stderr, "JSON-RPC WebSocket server listening on %s\n", ln.Addr())

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "websocket accept error: %v\n", err)
			return
		}
		if err := rt.ServeJSONRPCWS(r.Context(), conn); err != nil {
			fmt.Fprintf(os.Stderr, "websocket serve error: %v\n", err)
		}
	})

	srv := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
