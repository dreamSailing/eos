package ui

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/i18n"
	sidecarclient "github.com/dreamSailing/eos/pkg/coreapi/sidecar/client"

	tea "github.com/charmbracelet/bubbletea"
)

// TUIOptions holds CLI-provided overrides for the interactive TUI
type TUIOptions struct {
	SessionID       string   // --continue ("latest") or --resume ("session-id")
	ModelOverride   string   // --model
	MaxTurns        int      // --max-turns
	AllowedTools    []string // --allowed-tools
	DisallowedTools []string // --disallowed-tools
	AccessMode      string   // --access-mode
	ApprovalMode    string   // --approval-mode
	SandboxMode     string   // --sandbox-mode legacy alias
	SkipPermissions bool     // --dangerously-skip-permissions
}

// T 是 i18n.T 的别名，用于简化调用
func T(key, lang string, args ...interface{}) string {
	return i18n.T(key, lang, args...)
}

func StartInteractiveTUI() {
	StartInteractiveTUIWithOptions(TUIOptions{})
}

// StartInteractiveTUIWithOptions 启动 TUI 入口。
// 生产路径：直接启动 eos-core --app-server --stdio sidecar 子进程，
// 用 sidecarclient.Client + NewAppModelFromCoreClient 构造 AppModel。
//
// StartCoreClient 是显式注入点，测试可通过 StartCoreClientForTest 替换为 stub。
var StartCoreClient = func(ctx context.Context, opts sidecarclient.Options) (*sidecarclient.Client, error) {
	return sidecarclient.Start(ctx, opts)
}

func StartInteractiveTUIWithOptions(opts TUIOptions) {
	root, _ := os.Getwd()
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	rememberKnownWorkspace(root, true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := StartCoreClient(ctx, tuiSidecarClientOptions(opts))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start eos-core sidecar: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	if strings.TrimSpace(opts.SessionID) != "" {
		slog.Info("ui.startup.session", "session_id", opts.SessionID)
	}

	m := NewAppModelFromCoreClient(client)
	slog.Info("ui.startup.app.run")
	if _, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		slog.Error("ui.startup.app.run.error", "error", err)
		fmt.Fprintf(os.Stderr, "\nError: Application failed to start: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please check the logs for more details.\n")
		os.Exit(1)
	}
	slog.Info("ui.startup.app.stopped")
	fmt.Println(T("goodbye.emoji", "zh") + " " + T("goodbye.message", "zh"))
	fmt.Println(T("goodbye.ended", "zh"))
}

func tuiSidecarClientOptions(opts TUIOptions) sidecarclient.Options {
	return sidecarclient.Options{
		Env:              tuiOptionEnv(opts),
		Stderr:           newSidecarStderrWriter(),
		VerifyChecksum:   true,
		RequireSignature: true,
	}
}

// tuiOptionEnv 把 TUIOptions 透传到 eos-core 子进程环境变量。
// 字段名与 eos-core-protocol 的 ENV_* 常量保持一致。
func tuiOptionEnv(opts TUIOptions) map[string]string {
	env := map[string]string{}
	// Production TUI never inherits a fake provider selection from the parent shell.
	env["EOS_MODEL_PROVIDER"] = ""
	// 诊断：开启 sidecar debug 日志，配合 newSidecarStderrWriter 落盘排查 turn 恢复卡死。
	env["EOS_LOG_LEVEL"] = "debug"
	if storeDir := defaultRustCoreStoreDir(); storeDir != "" {
		env["EOS_CORE_STORE_DIR"] = storeDir
	}
	if v := strings.TrimSpace(opts.ModelOverride); v != "" {
		env["EOS_MODEL_OVERRIDE"] = v
	}
	if opts.MaxTurns > 0 {
		env["EOS_MAX_TURNS"] = fmt.Sprintf("%d", opts.MaxTurns)
	}
	if len(opts.AllowedTools) > 0 {
		env["EOS_ALLOWED_TOOLS"] = strings.Join(opts.AllowedTools, ",")
	}
	if len(opts.DisallowedTools) > 0 {
		env["EOS_DISALLOWED_TOOLS"] = strings.Join(opts.DisallowedTools, ",")
	}
	if v := strings.TrimSpace(opts.AccessMode); v != "" {
		env["EOS_ACCESS_MODE"] = v
	}
	if v := strings.TrimSpace(opts.ApprovalMode); v != "" {
		env["EOS_APPROVAL_MODE"] = v
	}
	if v := strings.TrimSpace(opts.SandboxMode); v != "" {
		env["EOS_SANDBOX_MODE"] = v
	}
	// workspace-write 沙箱需要至少一个可写根；不透传工作区根时 sidecar 的沙箱策略
	// workspace_root 为空，会导致工作区内所有写操作被拒（审批通过后仍拒绝并触发 turn 恢复卡死）。
	if cwd, err := os.Getwd(); err == nil {
		if ws := strings.TrimSpace(cwd); ws != "" {
			env["EOS_WORKSPACE_ROOT"] = ws
			env["EOS_SANDBOX_WORKSPACE_ROOT"] = ws
		}
	}
	if opts.SkipPermissions {
		// 双轴（approval=Never + sandbox=DangerFullAccess）由内核 bin 侧读
		// EOS_SKIP_PERMISSIONS 后用 permission_enter_full_access 单一真相源派生。
		// 这里必须清掉可能由 flag 默认值（如 --sandbox-mode=workspace）带入的
		// EOS_APPROVAL_MODE/EOS_SANDBOX_MODE，否则内核会因 skip 与显式 mode 共存
		// 而 fail-fast（AGENTS.md §3：壳层不做业务裁决）。
		env["EOS_SKIP_PERMISSIONS"] = "1"
		delete(env, "EOS_ACCESS_MODE")
		delete(env, "EOS_APPROVAL_MODE")
		delete(env, "EOS_SANDBOX_MODE")
	}
	return env
}

func defaultRustCoreStoreDir() string {
	if dir, err := os.UserHomeDir(); err == nil && strings.TrimSpace(dir) != "" {
		return filepath.Join(dir, ".eos", "core")
	}
	return ""
}

// newSidecarStderrWriter 把 eos-core sidecar 的 stderr（tracing 日志与 panic）
// 落盘到日志目录，避免被 io.Discard 吞掉导致排查无痕。打开失败时回退到丢弃，
// 不阻断 TUI 启动。
func newSidecarStderrWriter() io.Writer {
	dir := filepath.Join(config.ConfiguredLogDir(), "core")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Error("ui.sidecar.stderr.log_dir.error", "error", err)
		return io.Discard
	}
	f, err := os.OpenFile(filepath.Join(dir, "eos-core.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		slog.Error("ui.sidecar.stderr.open.error", "error", err)
		return io.Discard
	}
	return f
}
