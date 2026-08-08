package ui

// app_versions.go — 应用版本检查与更新提示。
//
// checkForUpdates 在后台调用 update.CheckLatest；若发现新版本则
// 通过 VersionCheckMsg 回到 Update，由 handleVersionCheck 把信息
// 写到 shell 的状态栏。
//
// VersionCheckMsg 类型本身定义在 msg.go，本文件只承载检查与处理函数。
//
// 代码原位于 app.go，仅做物理拆分，不改行为。

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dreamSailing/eos/internal/update"
)

// checkForUpdates checks for updates in the background
func (m *AppModel) checkForUpdates() tea.Cmd {
	return func() tea.Msg {
		result, err := update.CheckLatest(context.Background())
		if err != nil || result == nil {
			return nil
		}
		return VersionCheckMsg{Result: result}
	}
}

func (m *AppModel) handleVersionCheck(msg VersionCheckMsg) {
	if msg.Result != nil && msg.Result.HasUpdate {
		m.shell.SetUpdateInfo(msg.Result)
	}
}
