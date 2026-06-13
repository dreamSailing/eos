//go:build legacy

package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"log/slog"
	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/pkg/utils"
)

// ClearContext 清除上下文
func (rc *RuntimeCore) ClearContext() {
	rc.cm.Clear()
}

// ExecuteClear 执行清除命令
func (rc *RuntimeCore) ExecuteClear(ui CoreUI) {
	slog.Info("bridge.clear_command.start", "component", utils.ComponentSystem)
	rc.ClearContext()
	ui.ClearContent()
	slog.Info("bridge.clear_command.completed", "component", utils.ComponentSystem)
}

// CompactContext 压缩上下文
func (rc *RuntimeCore) CompactContext() {
	rc.cm.Compact()
}

// ExecuteCompact 执行压缩命令
func (rc *RuntimeCore) ExecuteCompact(ui CoreUI) {
	slog.Info("bridge.compact_command.start", "component", utils.ComponentSystem)
	rc.CompactContext()
	ui.WriteLine("white", ui.T("context.compacted"))
	slog.Info("bridge.compact_command.completed", "component", utils.ComponentSystem)
}

// AddAssistant 添加助手消息
func (rc *RuntimeCore) AddAssistant(text string) {
	rc.cm.AddAssistant(text)
}

// AddUser 添加用户消息
func (rc *RuntimeCore) AddUser(text string) {
	rc.cm.AddUser(text)
}

// AddUserWithImages 添加带图片的用户消息
func (rc *RuntimeCore) AddUserWithImages(text string, imagePaths []string) {
	rc.cm.AddUserWithImages(text, imagePaths)
}

// AddPinnedSystem 添加固定的系统消息
func (rc *RuntimeCore) AddPinnedSystem(text string) {
	rc.cm.AddPinned(ai.Message{
		Role:    "system",
		Content: text,
	})
}

// BuildPreview 构建预览消息
func (rc *RuntimeCore) BuildPreview() []ai.Message {
	return rc.cm.BuildPreview()
}

// AddEphemeral 添加临时消息
func (rc *RuntimeCore) AddEphemeral(text string) {
	rc.cm.AddEphemeral(text)
}

// AddPinned 添加固定消息
func (rc *RuntimeCore) AddPinned(msg ai.Message) {
	rc.cm.AddPinned(msg)
}
