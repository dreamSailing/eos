package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"fmt"
	"github.com/dreamSailing/eos/internal/pkg/plugins"
)

// RegisterPlugin 将插件注册到工具管理器中
func (m *Manager) RegisterPlugin(p plugins.ToolPlugin) {
	plugins.DefaultRegistry().Register(p)
	m.structured[p.Name()] = func(ctx context.Context, params map[string]any) ToolResult {
		if !plugins.DefaultRegistry().IsEnabled(p.Name()) {
			return ToolResult{
				Tool:   p.Name(),
				Status: ToolStatusError,
				Error:  fmt.Sprintf("plugin disabled: %s", p.Name()),
			}
		}
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
