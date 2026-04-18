package ui

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/runtime"
	"github.com/dreamSailing/eos/internal/session"
)

// injectProjectConventions 增强系统提示词，注入项目信息和意图识别
func injectProjectConventions(cm *session.ContextManager, cwd string) {
	prompt := runtime.BuildProjectPromptAdditions(cwd)
	if prompt != "" {
		cm.AddPinned(ai.Message{
			Role:    "system",
			Content: prompt,
		})
	}
}
