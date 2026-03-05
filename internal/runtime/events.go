package runtime

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
	EventAgentCall  = "agent.call"
	EventAgentTask  = "agent.task"
	EventAgentSkill = "agent.skill"
	EventAgentFinal = "agent.final" // 子 Agent 的最终输出（使用白色圆点）

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
