package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dreamSailing/eos/pkg/core"
	"github.com/spf13/cobra"
)

type execOptions struct {
	Prompt         string
	Workspace      string
	Sandbox        string
	ExecutionMode  string
	Output         string
	Timeout        time.Duration
	AccessMode     string
	ApprovalMode   string
	SkipPermission bool
}

func newExecCmd() *cobra.Command {
	var (
		workspace      string
		sandbox        string
		executionMode  string
		output         string
		timeout        time.Duration
		accessMode     string
		approvalMode   string
		skipPermission bool
	)

	cmd := &cobra.Command{
		Use:   "exec <prompt>",
		Short: "Run a single prompt in headless mode.",
		Long:  "Execute a single prompt in headless mode without the TUI. Supports workspace, sandbox, execution-mode, output format, and timeout options.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modes := resolveModeConfig(accessMode, approvalMode, sandbox, skipPermission, false)
			return runExec(cmd.Context(), execOptions{
				Prompt:         args[0],
				Workspace:      strings.TrimSpace(workspace),
				Sandbox:        modes.SandboxMode,
				ExecutionMode:  strings.TrimSpace(executionMode),
				Output:         strings.TrimSpace(output),
				Timeout:        timeout,
				AccessMode:     modes.AccessMode,
				ApprovalMode:   modes.ApprovalMode,
				SkipPermission: modes.SkipAllChecks,
			})
		},
	}

	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace root path")
	cmd.Flags().StringVar(&sandbox, "sandbox", "workspace", "Sandbox mode: workspace or full_access")
	cmd.Flags().StringVar(&executionMode, "execution-mode", "", "Execution mode for the AI agent")
	cmd.Flags().StringVar(&output, "output", "text", "Output format: text or json")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "Maximum execution duration (e.g. 30s, 5m)")
	cmd.Flags().StringVar(&accessMode, "access-mode", "", "Access mode: read-only, workspace-write, or danger-full-access")
	cmd.Flags().StringVar(&approvalMode, "approval-mode", "", "Approval mode: untrusted, on-failure, on-request, or never")
	cmd.Flags().BoolVar(&skipPermission, "dangerously-skip-permissions", false, "Skip all permission checks")

	return cmd
}

func runExec(ctx context.Context, opts execOptions) error {
	if opts.Output == "" {
		opts.Output = "text"
	}

	rt := core.NewRuntime()
	defer rt.Close()

	rt.ApplyStartupOptions(core.StartupOptions{
		AccessMode:      opts.AccessMode,
		ApprovalMode:    opts.ApprovalMode,
		SandboxMode:     opts.Sandbox,
		SkipPermissions: opts.SkipPermission,
	})

	if strings.TrimSpace(opts.Workspace) != "" {
		rt.LegacyCore().SetActiveWorkspaceRoot(strings.TrimSpace(opts.Workspace))
	}

	if strings.TrimSpace(opts.ExecutionMode) != "" {
		rt.SetExecutionMode(opts.ExecutionMode)
	}

	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Minute
	}

	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	start := time.Now()
	events, invokeErr := rt.Invoke(ctx, opts.Prompt)

	var content string
	var execErr error
	if invokeErr != nil {
		execErr = invokeErr
	} else {
		for ev := range events {
			switch ev.Type {
			case "TextFinal":
				content = ev.Message
			case "Error":
				if execErr == nil {
					execErr = fmt.Errorf("%s", ev.Message)
				}
			}
		}
	}
	elapsed := time.Since(start)

	if execErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			execErr = fmt.Errorf("exec timed out after %s", opts.Timeout)
		}
		if opts.Output == "json" {
			writeExecJSON(os.Stdout, ExecResult{Error: execErr.Error()})
		} else {
			fmt.Fprintln(os.Stderr, "Error:", execErr.Error())
		}
		return execErr
	}

	usage := rt.UsageSummary()
	modelName := rt.ModelName()

	result := ExecResult{
		Content:     content,
		Model:       modelName,
		InputTokens: usage.InputTokens,
		ReplyTokens: usage.ReplyTokens,
		TotalTokens: usage.TotalTokens,
		DurationMs:  int(elapsed.Milliseconds()),
		CostUSD:     usage.CostUSD,
		Workspace:   opts.Workspace,
	}

	switch opts.Output {
	case "json":
		writeExecJSON(os.Stdout, result)
	default:
		fmt.Fprintln(os.Stdout, content)
		parts := []string{
			fmt.Sprintf("Model: %s", modelName),
			fmt.Sprintf("Duration: %v", elapsed.Round(time.Millisecond)),
		}
		if usage.TotalTokens != nil {
			parts = append(parts, fmt.Sprintf("Tokens: %d", *usage.TotalTokens))
		}
		if usage.CostUSD != nil {
			parts = append(parts, fmt.Sprintf("Cost: $%.6f", *usage.CostUSD))
		}
		if opts.Workspace != "" {
			parts = append(parts, fmt.Sprintf("Workspace: %s", opts.Workspace))
		}
		fmt.Fprintf(os.Stderr, "\n---\n%s\n", strings.Join(parts, " | "))
	}

	return nil
}

func writeExecJSON(f *os.File, v ExecResult) {
	bs, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marshal error: %v\n", err)
		return
	}
	fmt.Fprintln(f, string(bs))
}

type ExecResult struct {
	Content     string   `json:"content,omitempty"`
	Model       string   `json:"model,omitempty"`
	InputTokens *int     `json:"input_tokens,omitempty"`
	ReplyTokens *int     `json:"reply_tokens,omitempty"`
	TotalTokens *int     `json:"total_tokens,omitempty"`
	DurationMs  int      `json:"duration_ms"`
	CostUSD     *float64 `json:"cost_usd,omitempty"`
	Workspace   string   `json:"workspace,omitempty"`
	Error       string   `json:"error,omitempty"`
}
