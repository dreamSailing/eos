package runtime

// LoopDetector 检测工具调用循环
type LoopDetector interface {
	// CheckLoop 检测工具调用是否形成循环
	// 返回 nil 表示正常，ErrLoopWarning 表示检测到循环（注入提示），ErrLoopForceBreak 表示强制中断
	CheckLoop(toolName string, args map[string]interface{}) error

	// Reset 重置检测器状态
	Reset()
}
