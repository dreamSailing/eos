package plugins

import (
	"context"
	"sync"
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
	mu      sync.RWMutex
}

var defaultRegistry = NewRegistry()

// NewRegistry 创建新的注册表
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]ToolPlugin),
	}
}

func DefaultRegistry() *Registry {
	return defaultRegistry
}

// Register 注册插件
func (r *Registry) Register(p ToolPlugin) {
	if r == nil || p == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins[p.Name()] = p
}

// Get 获取插件
func (r *Registry) Get(name string) (ToolPlugin, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// List 列出所有插件
func (r *Registry) List() []ToolPlugin {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]ToolPlugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		list = append(list, p)
	}
	return list
}

func (r *Registry) Reset() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins = make(map[string]ToolPlugin)
}
