package plugins

import (
	"context"
)

// ToolPlugin 定义了一个工具插件
type ToolPlugin interface {
	Name() string
	Description() string
	Execute(ctx context.Context, params map[string]any) (any, error)
}

// Registry 插件注册表
type Registry struct {
	plugins map[string]ToolPlugin
}

// NewRegistry 创建新的注册表
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]ToolPlugin),
	}
}

// Register 注册插件
func (r *Registry) Register(p ToolPlugin) {
	r.plugins[p.Name()] = p
}

// Get 获取插件
func (r *Registry) Get(name string) (ToolPlugin, bool) {
	p, ok := r.plugins[name]
	return p, ok
}

// List 列出所有插件
func (r *Registry) List() []ToolPlugin {
	list := make([]ToolPlugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		list = append(list, p)
	}
	return list
}
