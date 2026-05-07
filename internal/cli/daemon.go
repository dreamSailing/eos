package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/daemon"
	"github.com/spf13/cobra"
)

func newDaemonCmd() *cobra.Command {
	var workspace string
	var listenAddr string
	var baseURL string
	var allowedTools string
	var sandboxMode string
	var policyPath string
	var sessionStorePath string
	var stateFile string
	var scheduleFile string
	var logFile string
	var mcpBasePath string

	buildOptions := func() daemon.Options {
		return daemon.Options{
			Workspace:        workspace,
			ListenAddr:       listenAddr,
			BaseURL:          baseURL,
			AllowedTools:     splitCommaList(allowedTools),
			SandboxMode:      sandboxMode,
			PolicyPath:       policyPath,
			SessionStorePath: sessionStorePath,
			StateFile:        stateFile,
			ScheduleFile:     scheduleFile,
			LogFile:          logFile,
			MCPBasePath:      mcpBasePath,
		}
	}

	cmd := &cobra.Command{Use: "daemon", Short: "Run EOS as a background daemon with HTTP gateway."}

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start EOS daemon in background.",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := buildOptions()
			if strings.TrimSpace(opts.Workspace) == "" {
				opts.Workspace = config.DefaultWorkspacePath()
			}
			if err := daemon.EnsureDefaults(&opts); err != nil {
				return err
			}
			state, err := daemon.NewManager(opts).StartBackground()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "EOS daemon started\nPID: %d\nWorkspace: %s\nWeb: %s\nMCP SSE: %s%s\n", state.PID, state.Workspace, state.WebBaseURL, state.WebBaseURL, state.MCPBasePath)
			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show EOS daemon status.",
		RunE: func(cmd *cobra.Command, args []string) error {
			state, ok, err := daemon.Status(stateFile)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(cmd.OutOrStdout(), "EOS daemon is not running")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "EOS daemon is running\nPID: %d\nWorkspace: %s\nWeb: %s\nMCP SSE: %s%s\n", state.PID, state.Workspace, state.WebBaseURL, state.WebBaseURL, state.MCPBasePath)
			return nil
		},
	}

	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop EOS daemon.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := daemon.Stop(stateFile); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "EOS daemon stopped")
			return nil
		},
	}

	runCmd := &cobra.Command{
		Use:    "run",
		Short:  "Run EOS daemon in foreground.",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			opts := buildOptions()
			if err := daemon.EnsureDefaults(&opts); err != nil {
				return err
			}
			return daemon.NewManager(opts).RunForeground(ctx)
		},
	}

	for _, sub := range []*cobra.Command{startCmd, statusCmd, stopCmd, runCmd} {
		sub.Flags().StringVar(&workspace, "workspace", "", "workspace path (default is EOS default workspace)")
		sub.Flags().StringVar(&listenAddr, "listen", "127.0.0.1:8765", "HTTP gateway listen address")
		sub.Flags().StringVar(&baseURL, "base-url", "", "public base URL (optional)")
		sub.Flags().StringVar(&allowedTools, "allowed-tools", "", "comma-separated allowed tools")
		sub.Flags().StringVar(&sandboxMode, "sandbox-mode", "workspace", "sandbox mode")
		sub.Flags().StringVar(&policyPath, "policy", "", "policy file path")
		sub.Flags().StringVar(&sessionStorePath, "session-store", "", "session store file path")
		sub.Flags().StringVar(&stateFile, "state-file", daemon.DefaultStateFile(), "daemon state file")
		sub.Flags().StringVar(&scheduleFile, "schedule-file", daemon.DefaultScheduleFile(), "schedule file")
		sub.Flags().StringVar(&logFile, "log-file", daemon.DefaultLogFile(), "daemon log file")
		sub.Flags().StringVar(&mcpBasePath, "mcp-base-path", "/mcp", "MCP base path on HTTP gateway")
	}

	cmd.AddCommand(startCmd, statusCmd, stopCmd, runCmd)
	return cmd
}
