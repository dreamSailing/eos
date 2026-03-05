package shell

import (
	"os"
	"strings"

	"github.com/dreamSailing/vb-coding/internal/version"
	"github.com/dreamSailing/vb-coding/internal/ui/styles"

	"github.com/charmbracelet/lipgloss"
)

// WelcomeCard 欢迎卡片组件
type WelcomeCard struct {
	width     int
	height    int
	styles    *styles.Styles
	modelName string
	apiInfo   string
	workDir   string
	appVersion string
}

// NewWelcomeCard 创建新的欢迎卡片
func NewWelcomeCard(s *styles.Styles) *WelcomeCard {
	wd, _ := os.Getwd()
	return &WelcomeCard{
		width:     80,
		height:    24,
		styles:    s,
		modelName: "AI Assistant",
		apiInfo:   "Ready",
		workDir:   wd,
		appVersion: version.AppVersion,
	}
}

// SetSize 设置大小
func (w *WelcomeCard) SetSize(width, height int) {
	w.width = width
	w.height = height
}

// SetInfo 设置信息（空值不覆盖）
func (w *WelcomeCard) SetInfo(modelName, apiInfo, workDir string) {
	if modelName != "" {
		w.modelName = modelName
	}
	if apiInfo != "" {
		w.apiInfo = apiInfo
	}
	if workDir != "" {
		w.workDir = workDir
	}
}

// View 渲染欢迎卡片
func (w *WelcomeCard) View() string {
	if w.width == 0 {
		w.width = 80
	}

	// 大 ASCII Logo 行
	logoLines := []string{
		" __      ______     _____          _ _             ",
		" \\ \\    / /  _ \\   / ____|        | (_)            ",
		"  \\ \\  / /| |_) | | |     ___   __| |_ _ __   __ _ ",
		"   \\ \\/ / |  _ <  | |    / _ \\ / _` | | '_ \\ / _` |",
		"    \\  /  | |_) | | |___| (_) | (_| | | | | | (_| |",
		"     \\/   |____/   \\_____\\___/ \\__,_|_|_| |_|\\__, |",
		"                                              __/ |",
		"                                             |___/ ",
	}

	// Logo 样式（橙色/黄色）
	logoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f59e0b")).
		Align(lipgloss.Center)

	// 副标题样式
	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#94a3b8")).
		Align(lipgloss.Center).
		Width(w.width - 4)

	// 信息行样式
	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f1f5f9")).
		PaddingLeft(4)

	// 标签样式
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6366f1"))

	// 边框样式
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#f59e0b")).
		Padding(2).
		Width(w.width - 4)

	var content strings.Builder

	// 渲染 ASCII Logo
	for _, line := range logoLines {
		content.WriteString(logoStyle.Render(line))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(subtitleStyle.Render("AI Powered Development Assistant"))
	content.WriteString("\n")
	content.WriteString(subtitleStyle.Render(version.AppName + " " + w.appVersion))
	content.WriteString("\n\n")

	// 信息行
	content.WriteString(infoStyle.Render("🧠 Model: " + w.modelName))
	content.WriteString("\n")
	content.WriteString(infoStyle.Render("🔗 API: " + w.apiInfo))
	content.WriteString("\n")
	content.WriteString(infoStyle.Render("📂 Path: " + w.workDir))
	content.WriteString("\n\n")

	// 快捷键提示
	content.WriteString(labelStyle.Render("快捷键:"))
	content.WriteString("\n")
	content.WriteString(infoStyle.Render("?  显示帮助   F2 切换模式   /  命令提示"))
	content.WriteString("\n")
	content.WriteString(infoStyle.Render("Enter 发送   Esc 清空      Ctrl+C 退出"))

	return borderStyle.Render(content.String())
}
