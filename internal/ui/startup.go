package ui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/dreamSailing/vb-coding/internal/ai"
	"github.com/dreamSailing/vb-coding/internal/bridge"
	"github.com/dreamSailing/vb-coding/internal/config"
	"github.com/dreamSailing/vb-coding/internal/i18n"
	"github.com/dreamSailing/vb-coding/internal/session"
	"github.com/dreamSailing/vb-coding/internal/tools"

	tea "github.com/charmbracelet/bubbletea"
)

// TUIOptions holds CLI-provided overrides for the interactive TUI
type TUIOptions struct {
	SessionID        string   // --continue ("latest") or --resume ("session-id")
	ModelOverride    string   // --model
	MaxTurns         int      // --max-turns
	AllowedTools     []string // --allowed-tools
	DisallowedTools  []string // --disallowed-tools
	SkipPermissions  bool     // --dangerously-skip-permissions
}

// T 是 i18n.T 的别名，用于简化调用
func T(key, lang string, args ...interface{}) string {
	return i18n.T(key, lang, args...)
}

func StartInteractiveTUI() {
	StartInteractiveTUIWithOptions(TUIOptions{})
}

func StartInteractiveTUIWithOptions(opts TUIOptions) {
	cm := session.NewContextManager()
	tm := tools.NewManager()
	if p, _ := os.Getwd(); p != "" {
		rememberKnownWorkspace(p, true)
		core := bridge.NewRuntimeCore(cm, tm, nil)
		applyTUIOptions(core, cm, opts)
		base, key, model, _ := core.ResolveAPIConfig()
		if model != "" {
			ai.PrimeContextWindowFromProvider(context.Background(), base, key, model)
			cm.SetModel(model)
			window := ai.ContextWindowTokens(model)
			switch {
			case window >= 128000:
				cm.SetCompressionStrategy(session.CompressionConservative)
			case window >= 32000:
				cm.SetCompressionStrategy(session.CompressionBalanced)
			default:
				cm.SetCompressionStrategy(session.CompressionAggressive)
			}
		}
		injectProjectConventions(cm, p)
		if raw, err := os.ReadFile(filepath.Join(p, "VB.md")); err == nil && strings.TrimSpace(string(raw)) != "" {
			cm.SetPinnedDoc("VB.md", string(raw), 20000)
		}
		if raw, err := os.ReadFile(filepath.Join(p, ".vb", "Rules.md")); err == nil && strings.TrimSpace(string(raw)) != "" {
			cm.SetPinnedDoc(".vb/Rules.md", string(raw), 20000)
		}
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			if raw, err := os.ReadFile(filepath.Join(home, ".vb", "Rules.md")); err == nil && strings.TrimSpace(string(raw)) != "" {
				cm.SetPinnedDoc("~/.vb/Rules.md", string(raw), 20000)
			}
		}
		m := NewAppModel(core)
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
		return
	}

	core := bridge.NewRuntimeCore(cm, tm, nil)
	applyTUIOptions(core, cm, opts)
	base, key, model, _ := core.ResolveAPIConfig()
	if model != "" {
		ai.PrimeContextWindowFromProvider(context.Background(), base, key, model)
		cm.SetModel(model)
		window := ai.ContextWindowTokens(model)
		switch {
		case window >= 128000:
			cm.SetCompressionStrategy(session.CompressionConservative)
		case window >= 32000:
			cm.SetCompressionStrategy(session.CompressionBalanced)
		default:
			cm.SetCompressionStrategy(session.CompressionAggressive)
		}
	}
	if raw, err := os.ReadFile(filepath.Join(".", "VB.md")); err == nil && strings.TrimSpace(string(raw)) != "" {
		cm.SetPinnedDoc("VB.md", string(raw), 20000)
	}
	if raw, err := os.ReadFile(filepath.Join(".", ".vb", "Rules.md")); err == nil && strings.TrimSpace(string(raw)) != "" {
		cm.SetPinnedDoc(".vb/Rules.md", string(raw), 20000)
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		if raw, err := os.ReadFile(filepath.Join(home, ".vb", "Rules.md")); err == nil && strings.TrimSpace(string(raw)) != "" {
			cm.SetPinnedDoc("~/.vb/Rules.md", string(raw), 20000)
		}
	}
	m := NewAppModel(core)
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
func applyTUIOptions(core *bridge.RuntimeCore, cm *session.ContextManager, opts TUIOptions) {
	if opts.ModelOverride != "" {
		// Try to find the model in config and activate it
		cfg, _ := config.Load()
		for _, m := range cfg.Models {
			if m.Name == opts.ModelOverride || m.Model == opts.ModelOverride {
				core.SetModelOverride(m.Model, m.APIBase)
				break
			}
		}
	}
	if opts.MaxTurns > 0 {
		core.SetMaxTurns(opts.MaxTurns)
	}
	if len(opts.AllowedTools) > 0 || len(opts.DisallowedTools) > 0 {
		core.SetToolPermissions(opts.AllowedTools, opts.DisallowedTools)
	}
	if opts.SkipPermissions {
		core.SetSkipPermissions(true)
	}
	if opts.SessionID != "" {
		slog.Info("ui.startup.session", "session_id", opts.SessionID)
	}
}
