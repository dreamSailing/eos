package styles

import (
    "github.com/charmbracelet/lipgloss"
)

// Theme 定义了应用程序的主题结构
type Theme struct {
    // 主色�?
    Primary     lipgloss.Color
    Secondary   lipgloss.Color
    Muted       lipgloss.Color

    // 背景�?
    Background  lipgloss.Color
    Surface     lipgloss.Color
    SurfaceAlt  lipgloss.Color

    // 强调�?
    Accent      lipgloss.Color

    // 状态颜�?
    Success     lipgloss.Color
    Error       lipgloss.Color
    Warning     lipgloss.Color
    Info        lipgloss.Color

    // 文本颜色
    Text        lipgloss.Color
    TextMuted   lipgloss.Color

    // 边框样式
    Border      lipgloss.Border

    // 表格样式
    TableHeader lipgloss.Style
    TableCell   lipgloss.Style
}

// DefaultDarkTheme 返回默认的深色主�?
func DefaultDarkTheme() *Theme {
    return &Theme{
        Primary:    lipgloss.Color("#6366f1"), // Indigo
        Secondary:  lipgloss.Color("#8b5cf6"), // Violet
        Muted:      lipgloss.Color("#64748b"), // Slate

        Background: lipgloss.Color("#0f172a"), // Slate 900
        Surface:    lipgloss.Color("#1e293b"), // Slate 800
        SurfaceAlt: lipgloss.Color("#334155"), // Slate 700

        Accent:     lipgloss.Color("#f59e0b"), // Amber

        Success:    lipgloss.Color("#10b981"), // Emerald
        Error:      lipgloss.Color("#ef4444"), // Red
        Warning:    lipgloss.Color("#f59e0b"), // Amber
        Info:       lipgloss.Color("#3b82f6"), // Blue

        Text:       lipgloss.Color("#f1f5f9"), // Slate 100
        TextMuted:  lipgloss.Color("#94a3b8"), // Slate 400

        Border:     lipgloss.NormalBorder(),

        TableHeader: lipgloss.NewStyle().
            Background(lipgloss.Color("#334155")).
            Foreground(lipgloss.Color("#f1f5f9")).
            Bold(true),
        TableCell: lipgloss.NewStyle().
            Background(lipgloss.Color("#1e293b")).
            Foreground(lipgloss.Color("#f1f5f9")),
    }
}

// DefaultLightTheme 返回默认的浅色主�?
func DefaultLightTheme() *Theme {
    return &Theme{
        Primary:    lipgloss.Color("#4f46e5"), // Indigo
        Secondary:  lipgloss.Color("#7c3aed"), // Violet
        Muted:      lipgloss.Color("#64748b"), // Slate

        Background: lipgloss.Color("#f8fafc"), // Slate 50
        Surface:    lipgloss.Color("#ffffff"), // White
        SurfaceAlt: lipgloss.Color("#f1f5f9"), // Slate 100

        Accent:     lipgloss.Color("#f59e0b"), // Amber

        Success:    lipgloss.Color("#059669"), // Emerald
        Error:      lipgloss.Color("#dc2626"), // Red
        Warning:    lipgloss.Color("#d97706"), // Amber
        Info:       lipgloss.Color("#2563eb"), // Blue

        Text:       lipgloss.Color("#0f172a"), // Slate 900
        TextMuted:  lipgloss.Color("#64748b"), // Slate 500

        Border:     lipgloss.NormalBorder(),

        TableHeader: lipgloss.NewStyle().
            Background(lipgloss.Color("#f1f5f9")).
            Foreground(lipgloss.Color("#0f172a")).
            Bold(true),
        TableCell: lipgloss.NewStyle().
            Background(lipgloss.Color("#ffffff")).
            Foreground(lipgloss.Color("#0f172a")),
    }
}

// HighContrastTheme 返回高对比度主题
func HighContrastTheme() *Theme {
    return &Theme{
        Primary:    lipgloss.Color("#0000ff"), // Blue
        Secondary:  lipgloss.Color("#800080"), // Purple
        Muted:      lipgloss.Color("#808080"), // Gray

        Background: lipgloss.Color("#000000"), // Black
        Surface:    lipgloss.Color("#333333"), // Dark Gray
        SurfaceAlt: lipgloss.Color("#666666"), // Medium Gray

        Accent:     lipgloss.Color("#ffff00"), // Yellow

        Success:    lipgloss.Color("#00ff00"), // Green
        Error:      lipgloss.Color("#ff0000"), // Red
        Warning:    lipgloss.Color("#ffff00"), // Yellow
        Info:       lipgloss.Color("#00ffff"), // Cyan

        Text:       lipgloss.Color("#ffffff"), // White
        TextMuted:  lipgloss.Color("#808080"), // Gray

        Border:     lipgloss.NormalBorder(),

        TableHeader: lipgloss.NewStyle().
            Background(lipgloss.Color("#000000")).
            Foreground(lipgloss.Color("#ffff00")).
            Bold(true),
        TableCell: lipgloss.NewStyle().
            Background(lipgloss.Color("#333333")).
            Foreground(lipgloss.Color("#ffffff")),
    }
}

// GetTheme 返回指定名称的主�?
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

