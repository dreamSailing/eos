package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dreamSailing/eos/pkg/coreapi"
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
			modes := resolveModeConfig(accessMode, approvalMode, sandbox, skipPermission)
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
	cmd.Flags().StringVar(&output, "output", "text", "Output format: text, json, or stream-json")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "Maximum execution duration (e.g. 30s, 5m)")
	cmd.Flags().StringVar(&accessMode, "access-mode", "", "Access mode: read-only, workspace-write, or danger-full-access")
	cmd.Flags().StringVar(&approvalMode, "approval-mode", "", "Approval mode: untrusted, on-failure, on-request, or never")
	cmd.Flags().BoolVar(&skipPermission, "dangerously-skip-permissions", false, "Skip all permission checks")

	return cmd
}

// runExec 是 eos exec <prompt> 的生产入口。
//
// 引擎为 Rust-only：启动 eos-core sidecar，走 engine.Turns().Start + 事件订阅。
// 不存在 Go legacy runtime 回退路径。
func runExec(ctx context.Context, opts execOptions) error {
	if opts.Output == "" {
		opts.Output = "text"
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Minute
	}
	startedAt := time.Now()

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	selected, err := startRustOnlyEngine(ctx, "exec", execOptionEnv(opts))
	if err != nil {
		return err
	}
	defer selected.Close()

	engine := selected.Engine
	if err := applyExecStartup(ctx, engine, opts); err != nil {
		return err
	}

	// stream-json 走真增量 JSONL 流式（与 print 模式一致，对齐 codex exec --json）。
	if strings.EqualFold(strings.TrimSpace(opts.Output), "stream-json") {
		if err := runStreamJSONTurn(ctx, engine, opts.Prompt, startedAt, ""); err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				err = fmt.Errorf("exec timed out after %s", opts.Timeout)
			}
			writeExecError(opts.Output, err)
			return err
		}
		return nil
	}

	content, err := runSingleTurn(ctx, engine, opts.Prompt, opts.Output, "")
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			err = fmt.Errorf("exec timed out after %s", opts.Timeout)
		}
		writeExecError(opts.Output, err)
		return err
	}

	usage := coreapi.UsageSummary{}
	if summary, err := engine.Usage().Summary(ctx); err == nil {
		usage = summary
	}
	modelName, _ := resolveActiveModelName(ctx, engine)

	result := ExecResult{
		Content:     content,
		Model:       modelName,
		InputTokens: usage.InputTokens,
		ReplyTokens: usage.ReplyTokens,
		TotalTokens: usage.TotalTokens,
		DurationMs:  int(time.Since(startedAt).Milliseconds()),
		CostUSD:     usage.CostUSD,
		Workspace:   opts.Workspace,
	}
	writeExecOutput(opts.Output, result)
	return nil
}

// execOptionEnv 把 exec flags 透传到 eos-core 子进程环境变量。
// 与 internal/ui/adapter.tuiOptionEnv 字段保持一致。
func execOptionEnv(opts execOptions) map[string]string {
	env := map[string]string{}
	if v := strings.TrimSpace(opts.AccessMode); v != "" {
		env["EOS_ACCESS_MODE"] = v
	}
	if v := strings.TrimSpace(opts.ApprovalMode); v != "" {
		env["EOS_APPROVAL_MODE"] = v
	}
	if v := strings.TrimSpace(opts.Sandbox); v != "" {
		env["EOS_SANDBOX_MODE"] = v
	}
	if ws := strings.TrimSpace(opts.Workspace); ws != "" {
		env["EOS_WORKSPACE_ROOT"] = ws
		env["EOS_SANDBOX_WORKSPACE_ROOT"] = ws
	} else if cwd, err := os.Getwd(); err == nil {
		if cwd := strings.TrimSpace(cwd); cwd != "" {
			env["EOS_WORKSPACE_ROOT"] = cwd
			env["EOS_SANDBOX_WORKSPACE_ROOT"] = cwd
		}
	}
	if opts.SkipPermission {
		// 双轴（approval=Never + sandbox=DangerFullAccess）由内核 bin 侧读
		// EOS_SKIP_PERMISSIONS 后用 permission_enter_full_access 单一真相源派生。
		// 清掉可能残留的 mode env，避免与 skip 标志共存触发内核 fail-fast
		// （AGENTS.md §3：壳层不做业务裁决）。resolveModeConfig 在 skip=true 时
		// 已把 AccessMode/ApprovalMode/SandboxMode 清空，这里是防御性兜底。
		env["EOS_SKIP_PERMISSIONS"] = "1"
		delete(env, "EOS_ACCESS_MODE")
		delete(env, "EOS_APPROVAL_MODE")
		delete(env, "EOS_SANDBOX_MODE")
	}
	return env
}

// applyExecStartup 把 workspace/execution-mode 等 startup 配置应用到已 handshake 的 engine。
func applyExecStartup(ctx context.Context, engine coreapi.Engine, opts execOptions) error {
	if engine == nil {
		return fmt.Errorf("core engine unavailable")
	}
	if ws := strings.TrimSpace(opts.Workspace); ws != "" {
		if err := engine.Workspaces().SetForeground(ctx, coreapi.WorkspacePathRequest{Path: ws}); err != nil {
			return fmt.Errorf("set foreground workspace: %w", err)
		}
	}
	if mode := strings.TrimSpace(opts.ExecutionMode); mode != "" {
		if err := engine.Modes().SetExecutionMode(ctx, coreapi.SetModeRequest{Mode: mode}); err != nil {
			return fmt.Errorf("set execution mode: %w", err)
		}
	}
	return nil
}

func writeExecError(output string, err error) {
	if output == "json" {
		writeExecJSON(os.Stdout, ExecResult{Error: err.Error()})
		return
	}
	fmt.Fprintln(os.Stderr, "Error:", err.Error())
}

func writeExecOutput(output string, result ExecResult) {
	if output == "json" {
		writeExecJSON(os.Stdout, result)
		return
	}
	fmt.Fprintln(os.Stdout, result.Content)
	parts := []string{
		fmt.Sprintf("Model: %s", result.Model),
		fmt.Sprintf("Duration: %v", time.Duration(result.DurationMs)*time.Millisecond),
	}
	if result.TotalTokens != nil {
		parts = append(parts, fmt.Sprintf("Tokens: %d", *result.TotalTokens))
	}
	if result.CostUSD != nil {
		parts = append(parts, fmt.Sprintf("Cost: $%.6f", *result.CostUSD))
	}
	if result.Workspace != "" {
		parts = append(parts, fmt.Sprintf("Workspace: %s", result.Workspace))
	}
	fmt.Fprintf(os.Stderr, "\n---\n%s\n", strings.Join(parts, " | "))
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
