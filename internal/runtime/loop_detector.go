package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


// LoopDetector 检测工具调用循环
type LoopDetector interface {
	// CheckLoop 检测工具调用是否形成循环
	// 返回 nil 表示正常，ErrLoopWarning 表示检测到循环（注入提示），ErrLoopForceBreak 表示强制中断
	CheckLoop(toolName string, args map[string]interface{}) error

	// Reset 重置检测器状态
	Reset()
}
