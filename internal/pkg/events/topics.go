package events

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
