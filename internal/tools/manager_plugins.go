package tools

import (
	"context"
	"github.com/dreamSailing/vb-coding/internal/pkg/plugins"
)

// RegisterPlugin 将插件注册到工具管理器中
func (m *Manager) RegisterPlugin(p plugins.ToolPlugin) {
	m.structured[p.Name()] = func(ctx context.Context, params map[string]any) ToolResult {
		res, err := p.Execute(ctx, params)
		if err != nil {
			return ToolResult{
				Tool:   p.Name(),
				Status: ToolStatusError,
				Error:  err.Error(),
			}
		}

		// 处理结果数据
		data := make(map[string]any)
		if res != nil {
			if m, ok := res.(map[string]any); ok {
				data = m
			} else {
				data["result"] = res
			}
		}

		return ToolResult{
			Tool:   p.Name(),
			Status: ToolStatusSuccess,
			Data:   data,
		}
	}
}

// LoadPluginsFromRegistry 从注册表批量加载插件
func (m *Manager) LoadPluginsFromRegistry(registry *plugins.Registry) {
	for _, p := range registry.List() {
		m.RegisterPlugin(p)
	}
}
