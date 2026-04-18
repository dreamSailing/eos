package events

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


const (
	// LSP 诊断更新事件
	TopicLSPDiagnostics = "lsp.diagnostics"

	// 文件变更事件
	TopicFileChanged = "file.changed"

	// Agent 状态变更事件
	TopicAgentStatus = "agent.status"

	// UI 通知事件
	TopicUINotify = "ui.notify"
)
