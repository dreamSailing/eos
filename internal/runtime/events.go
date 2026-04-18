package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


// 统一的事件类型常量
// 所有事件格式：event_type:content
// 去掉 meta: 前缀，使用点分隔命名，风格统一

const (
	// 工具相关事件
	EventToolCall   = "tool.call"
	EventToolResult = "tool.result"

	// 助手相关事件
	EventAssistantStart = "assistant.start"
	EventAssistantDelta = "assistant.delta"
	EventAssistantFinal = "assistant.final"

	// Agent 相关事件
	EventAgentCall      = "agent.call"
	EventAgentTask      = "agent.task"
	EventAgentSkill     = "agent.skill"
	EventAgentStarted   = "agent.started"
	EventAgentProgress  = "agent.progress"
	EventAgentCompleted = "agent.completed"
	EventAgentFailed    = "agent.failed"
	EventAgentCancelled = "agent.cancelled"
	EventAgentFinal     = "agent.final" // 兼容旧事件名

	// 流程相关事件
	EventPhaseNote         = "phase.note"
	EventDispatchCompleted = "dispatch.completed"
	EventPlanReady         = "plan.ready"

	// 调试相关事件
	EventCrumb = "crumb"

	// 错误相关事件
	EventLoopBlock   = "loop.block"
	EventToolBlocked = "tool.blocked"
	EventToolTimeout = "tool.timeout"
)
