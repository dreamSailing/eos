package styles

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme 定义了应用程序的主题结构
type Theme struct {
	// 主色
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Muted     lipgloss.Color

	// 背景色
	Background lipgloss.Color
	Surface    lipgloss.Color
	SurfaceAlt lipgloss.Color

	// 强调色
	Accent lipgloss.Color

	// 状态颜色
	Success lipgloss.Color
	Error   lipgloss.Color
	Warning lipgloss.Color
	Info    lipgloss.Color

	// 文本颜色
	Text      lipgloss.Color
	TextMuted lipgloss.Color

	// 边框样式
	Border lipgloss.Border

	// 表格样式
	TableHeader lipgloss.Style
	TableCell   lipgloss.Style
}

func newTheme(
	primary, secondary, muted lipgloss.Color,
	background, surface, surfaceAlt lipgloss.Color,
	accent, success, errColor, warning, info lipgloss.Color,
	text, textMuted, tableHeaderBg, tableHeaderFg, tableCellBg, tableCellFg lipgloss.Color,
) *Theme {
	return &Theme{
		Primary:    primary,
		Secondary:  secondary,
		Muted:      muted,
		Background: background,
		Surface:    surface,
		SurfaceAlt: surfaceAlt,
		Accent:     accent,
		Success:    success,
		Error:      errColor,
		Warning:    warning,
		Info:       info,
		Text:       text,
		TextMuted:  textMuted,
		Border:     lipgloss.NormalBorder(),
		TableHeader: lipgloss.NewStyle().
			Background(tableHeaderBg).
			Foreground(tableHeaderFg).
			Bold(true),
		TableCell: lipgloss.NewStyle().
			Background(tableCellBg).
			Foreground(tableCellFg),
	}
}

// DefaultDarkTheme 返回默认的深色主题
func DefaultDarkTheme() *Theme {
	return newTheme(
		lipgloss.Color("#6366f1"), lipgloss.Color("#8b5cf6"), lipgloss.Color("#64748b"),
		lipgloss.Color("#0f172a"), lipgloss.Color("#1e293b"), lipgloss.Color("#334155"),
		lipgloss.Color("#f59e0b"), lipgloss.Color("#10b981"), lipgloss.Color("#ef4444"), lipgloss.Color("#f59e0b"), lipgloss.Color("#3b82f6"),
		lipgloss.Color("#f1f5f9"), lipgloss.Color("#94a3b8"), lipgloss.Color("#334155"), lipgloss.Color("#f1f5f9"), lipgloss.Color("#1e293b"), lipgloss.Color("#f1f5f9"),
	)
}

// DefaultLightTheme 返回默认的浅色主题
func DefaultLightTheme() *Theme {
	return newTheme(
		lipgloss.Color("#4f46e5"), lipgloss.Color("#7c3aed"), lipgloss.Color("#64748b"),
		lipgloss.Color("#f8fafc"), lipgloss.Color("#ffffff"), lipgloss.Color("#f1f5f9"),
		lipgloss.Color("#f59e0b"), lipgloss.Color("#059669"), lipgloss.Color("#dc2626"), lipgloss.Color("#d97706"), lipgloss.Color("#2563eb"),
		lipgloss.Color("#0f172a"), lipgloss.Color("#64748b"), lipgloss.Color("#f1f5f9"), lipgloss.Color("#0f172a"), lipgloss.Color("#ffffff"), lipgloss.Color("#0f172a"),
	)
}

// HighContrastTheme 返回高对比度主题
func HighContrastTheme() *Theme {
	return newTheme(
		lipgloss.Color("#0000ff"), lipgloss.Color("#800080"), lipgloss.Color("#808080"),
		lipgloss.Color("#000000"), lipgloss.Color("#333333"), lipgloss.Color("#666666"),
		lipgloss.Color("#ffff00"), lipgloss.Color("#00ff00"), lipgloss.Color("#ff0000"), lipgloss.Color("#ffff00"), lipgloss.Color("#00ffff"),
		lipgloss.Color("#ffffff"), lipgloss.Color("#808080"), lipgloss.Color("#000000"), lipgloss.Color("#ffff00"), lipgloss.Color("#333333"), lipgloss.Color("#ffffff"),
	)
}

// GetTheme 返回指定名称的主题
func GetTheme(name string) *Theme {
	switch name {
	case "light":
		return DefaultLightTheme()
	case "high-contrast":
		return HighContrastTheme()
	default:
		return DefaultDarkTheme()
	}
}
