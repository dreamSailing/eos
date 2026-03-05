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
	"github.com/dreamSailing/vb-coding/internal/i18n"
	"github.com/dreamSailing/vb-coding/internal/session"
	"github.com/dreamSailing/vb-coding/internal/tools"

	tea "github.com/charmbracelet/bubbletea"
)

// T 是 i18n.T 的别名，用于简化调用
func T(key, lang string, args ...interface{}) string {
	return i18n.T(key, lang, args...)
}

func StartInteractiveTUI() {
	cm := session.NewContextManager()
	tm := tools.NewManager()
	if p, _ := os.Getwd(); p != "" {
		core := bridge.NewRuntimeCore(cm, tm, nil) // 传入 nil 因为我们不再使用 CoreUI 接口
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
			// 打印用户友好的错误信息
			fmt.Fprintf(os.Stderr, "\nError: Application failed to start: %v\n", err)
			fmt.Fprintf(os.Stderr, "Please check the logs for more details.\n")
			os.Exit(1)
		}
		slog.Info("ui.startup.app.stopped")
		fmt.Println(T("goodbye.emoji", "zh") + " " + T("goodbye.message", "zh"))
		fmt.Println(T("goodbye.ended", "zh"))
		return
	}

	core := bridge.NewRuntimeCore(cm, tm, nil) // 传入 nil 因为我们不再使用 CoreUI 接口
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
		// 打印用户友好的错误信息
		fmt.Fprintf(os.Stderr, "\nError: Application failed to start: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please check the logs for more details.\n")
		os.Exit(1)
	}
	slog.Info("ui.startup.app.stopped")
	fmt.Println(T("goodbye.emoji", "zh") + " " + T("goodbye.message", "zh"))
	fmt.Println(T("goodbye.ended", "zh"))
}
