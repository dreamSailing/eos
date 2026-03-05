package ui

import (
	"github.com/dreamSailing/vb-coding/internal/ai"
	"github.com/dreamSailing/vb-coding/internal/runtime"
	"github.com/dreamSailing/vb-coding/internal/session"
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
