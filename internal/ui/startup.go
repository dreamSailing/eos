package ui

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/dreamSailing/eos/internal/i18n"
	sharedcore "github.com/dreamSailing/eos/pkg/core"

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

func StartInteractiveTUIWithOptions(opts TUIOptions) {
	runtime := sharedcore.NewRuntime()
	defer runtime.Close()

	root, _ := os.Getwd()
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	rememberKnownWorkspace(root, true)
	applyTUIOptions(runtime, opts)
	runtime.PrepareStartupContext(context.Background(), root)
	if strings.TrimSpace(opts.SessionID) != "" {
		slog.Info("ui.startup.session", "session_id", opts.SessionID)
	}

	m := NewAppModelFromRuntime(runtime)
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

// applyTUIOptions applies CLI-provided overrides to the runtime core
func applyTUIOptions(runtime *sharedcore.Runtime, opts TUIOptions) {
	if runtime == nil {
		return
	}
	runtime.ApplyStartupOptions(sharedcore.StartupOptions{
		ModelOverride:   opts.ModelOverride,
		MaxTurns:        opts.MaxTurns,
		AllowedTools:    append([]string(nil), opts.AllowedTools...),
		DisallowedTools: append([]string(nil), opts.DisallowedTools...),
		AccessMode:      opts.AccessMode,
		ApprovalMode:    opts.ApprovalMode,
		SandboxMode:     opts.SandboxMode,
		SkipPermissions: opts.SkipPermissions,
	})
}
