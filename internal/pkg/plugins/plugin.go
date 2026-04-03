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

// Metadata 描述插件来源与附加信息。
type Metadata struct {
	Source  string
	Kind    string
	Command string
}

// MetadataProvider 可选地暴露插件元数据。
type MetadataProvider interface {
	PluginMetadata() Metadata
}

// Registry 插件注册表
type Registry struct {
	plugins map[string]ToolPlugin
	enabled map[string]bool
	mu      sync.RWMutex
}

var defaultRegistry = NewRegistry()

// NewRegistry 创建新的注册表
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]ToolPlugin),
		enabled: make(map[string]bool),
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
	name := p.Name()
	r.plugins[name] = p
	if _, ok := r.enabled[name]; !ok {
		r.enabled[name] = true
	}
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

// SetEnabled 设置插件启用状态。即使插件尚未注册，也会保留该覆盖值。
func (r *Registry) SetEnabled(name string, enabled bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled[name] = enabled
}

// IsEnabled 返回插件启用状态，未显式配置时默认为启用。
func (r *Registry) IsEnabled(name string) bool {
	if r == nil {
		return true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	enabled, ok := r.enabled[name]
	if !ok {
		return true
	}
	return enabled
}

// MetadataOf 返回插件元数据，未提供时给出默认来源。
func MetadataOf(p ToolPlugin) Metadata {
	if p == nil {
		return Metadata{}
	}
	if provider, ok := p.(MetadataProvider); ok {
		meta := provider.PluginMetadata()
		if meta.Source == "" {
			meta.Source = "registry"
		}
		return meta
	}
	return Metadata{Source: "registry"}
}

func (r *Registry) Reset() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins = make(map[string]ToolPlugin)
	r.enabled = make(map[string]bool)
}
