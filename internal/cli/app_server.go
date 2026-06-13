//go:build legacy

package cli

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/coder/websocket"

	sharedcore "github.com/dreamSailing/eos/pkg/core"
	"github.com/dreamSailing/eos/pkg/coreapi"
	"github.com/dreamSailing/eos/pkg/coreapi/engineprovider"
	coreapijsonrpc "github.com/dreamSailing/eos/pkg/coreapi/jsonrpc"
	"github.com/dreamSailing/eos/pkg/coreapi/sidecar"
	protocoljsonrpc "github.com/dreamSailing/eos/pkg/protocol/jsonrpc"
	"github.com/spf13/cobra"
)

const envRustCoreStoreDir = "EOS_CORE_STORE_DIR"

func newAppServerCmd() *cobra.Command {
	var transport string
	var workspace string
	var addr string
	var coreEngine string

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

			// app-server production path: 优先选 eos-core Rust sidecar。
			// sharedcore.NewRuntime / NewLegacyEngine 仅在 AllowFallback=true
			// (parity mode 或 EOS_CORE_ALLOW_FALLBACK=1) 时 lazy 创建，
			// 不污染默认启动路径。
			allowFallback := appServerAllowFallback(coreEngine)

			var rt *sharedcore.Runtime
			if allowFallback {
				rt = sharedcore.NewRuntime()
				defer rt.Close()
			}

			workspace = strings.TrimSpace(workspace)
			if workspace != "" && rt != nil {
				if err := rt.RememberWorkspace(workspace, true); err != nil {
					return err
				}
				if err := rt.SetForegroundWorkspace(workspace); err != nil {
					return err
				}
			}

			opts := engineprovider.Options{
				Mode:            engineprovider.Mode(coreEngine),
				Sidecar:         appServerSidecarOptions(),
				RequiredMethods: appServerRequiredMethods(),
				// app-server production path: 不允许静默回退到 legacy。
				// legacy 只允许 dev-only 显式 AllowFallback。
				AllowFallback: allowFallback,
			}
			if rt != nil {
				opts.Legacy = sharedcore.NewLegacyEngine(rt)
			}

			selected, err := engineprovider.Select(ctx, opts)
			if err != nil {
				return err
			}
			defer selected.Close()
			writeAppServerEngineSelection(cmd.ErrOrStderr(), selected)

			switch transport {
			case "stdio":
				return serveEngineStream(ctx, selected.Engine, os.Stdin, os.Stdout)
			case "ws":
				return serveEngineWS(ctx, selected.Engine, addr)
			default:
				return fmt.Errorf("unsupported app-server transport: %s", transport)
			}
		},
	}

	cmd.Flags().StringVar(&transport, "transport", "stdio", "transport: stdio, ws")
	cmd.Flags().StringVar(&workspace, "workspace", "", "foreground workspace path (optional)")
	cmd.Flags().StringVar(&addr, "addr", "localhost:8080", "listen address for ws transport")
	cmd.Flags().StringVar(&coreEngine, "core-engine", "", "core engine: auto, legacy, rust")
	_ = cmd.Flags().MarkHidden("core-engine")
	return cmd
}

func appServerSidecarOptions() sidecar.ProcessOptions {
	env := map[string]string{}
	if value, ok := os.LookupEnv(envRustCoreStoreDir); !ok || strings.TrimSpace(value) == "" {
		if dir := defaultRustCoreStoreDir(); dir != "" {
			env[envRustCoreStoreDir] = dir
		}
	}
	return productionSidecarProcessOptions(env)
}

func defaultRustCoreStoreDir() string {
	if dir, err := os.UserHomeDir(); err == nil && strings.TrimSpace(dir) != "" {
		return filepath.Join(dir, ".eos", "core")
	}
	return ""
}

func writeAppServerEngineSelection(w io.Writer, selected engineprovider.Selection) {
	if w == nil {
		return
	}
	if selected.FallbackUsed {
		fmt.Fprintf(w, "core engine selected: %s (fallback: %s)\n", selected.Kind, selected.FallbackReason)
		return
	}
	fmt.Fprintf(w, "core engine selected: %s\n", selected.Kind)
}

func serveEngineStream(ctx context.Context, engine coreapi.Engine, reader io.Reader, writer io.Writer) error {
	stream := protocoljsonrpc.NewStream(reader, writer)
	router := protocoljsonrpc.NewRouter()
	if err := coreapijsonrpc.Register(router, engine, coreapijsonrpc.Options{
		ServerName:      "eos-core",
		ProtocolVersion: "v1",
		Notifier:        protocoljsonrpc.StreamNotifier{Stream: stream},
	}); err != nil {
		return err
	}
	return protocoljsonrpc.ServeStream(ctx, router, stream)
}

func serveEngineWS(ctx context.Context, engine coreapi.Engine, addr string) error {
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
		wsConn := protocoljsonrpc.NewWSConn(conn)
		router := protocoljsonrpc.NewRouter()
		if err := coreapijsonrpc.Register(router, engine, coreapijsonrpc.Options{
			ServerName:      "eos-core",
			ProtocolVersion: "v1",
			Notifier:        protocoljsonrpc.WSNotifier{Conn: wsConn},
		}); err != nil {
			fmt.Fprintf(os.Stderr, "websocket router error: %v\n", err)
			return
		}
		if err := protocoljsonrpc.ServeWS(r.Context(), router, wsConn); err != nil {
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

// appServerAllowFallback 根据 coreEngine 模式决定是否允许 legacy 回退。
// production: 任何模式都拒绝静默回退。
// dev/parity: 显式 AllowFallback=true。
func appServerAllowFallback(coreEngine string) bool {
	switch strings.ToLower(strings.TrimSpace(coreEngine)) {
	case "parity":
		return true
	}
	// dev-only override via env (用于本地排查)；production 应设为 false。
	return strings.EqualFold(strings.TrimSpace(os.Getenv("EOS_CORE_ALLOW_FALLBACK")), "1")
}

func appServerRequiredMethods() []string {
	return []string{
		protocoljsonrpc.MethodStateSnapshot,
		protocoljsonrpc.MethodWorkspaceList,
		protocoljsonrpc.MethodWorkspaceRemember,
		protocoljsonrpc.MethodWorkspaceSetForeground,
		protocoljsonrpc.MethodSessionList,
		protocoljsonrpc.MethodSessionCreate,
		protocoljsonrpc.MethodSessionResume,
		protocoljsonrpc.MethodSessionCurrent,
		protocoljsonrpc.MethodSessionMessagesLoad,
		protocoljsonrpc.MethodSessionMessagesSave,
		protocoljsonrpc.MethodTurnStart,
		protocoljsonrpc.MethodTurnInterrupt,
		protocoljsonrpc.MethodApprovalRespond,
		protocoljsonrpc.MethodInquiryRespond,
		protocoljsonrpc.MethodToolCatalog,
		protocoljsonrpc.MethodToolExecute,
		protocoljsonrpc.MethodEventSubscribe,
		protocoljsonrpc.MethodEventUnsubscribe,
		protocoljsonrpc.MethodConfigReload,
		protocoljsonrpc.MethodAgentControl,
		protocoljsonrpc.MethodAgentInput,
		protocoljsonrpc.MethodAgentRun,
		protocoljsonrpc.MethodSandboxBackend,
	}
}
