package adapter

import (
	"context"
	"github.com/dreamSailing/vb-coding/internal/bridge"
	"github.com/dreamSailing/vb-coding/internal/pkg/workspace"
	"github.com/dreamSailing/vb-coding/internal/pkg/settings"
	"github.com/dreamSailing/vb-coding/internal/session"
	"github.com/dreamSailing/vb-coding/internal/tools"
)

// RuntimeAdapter �?RuntimeCore �?Bubble Tea UI 之间的适配�?
type RuntimeAdapter struct {
	core *bridge.RuntimeCore
}

// NewRuntimeAdapter 创建新的 RuntimeAdapter 实例
func NewRuntimeAdapter(core *bridge.RuntimeCore) *RuntimeAdapter {
	return &RuntimeAdapter{core: core}
}

// Events 返回运行时事件通道
func (a *RuntimeAdapter) Events() <-chan bridge.Event {
	return a.core.Events()
}

// Invoke 调用 RuntimeCore �?GraphInvokePlanWithImages 方法
func (a *RuntimeAdapter) Invoke(ctx context.Context, query, executionMode string, imagePaths []string) (string, error) {
	msg, err := a.core.GraphInvokePlanWithImages(ctx, query, executionMode, imagePaths)
	if err != nil {
		return "", err
	}
	if msg == nil {
		return "", nil
	}
	return msg.Content, nil
}

// GetContext 获取会话上下文管理器
func (a *RuntimeAdapter) GetContext() *session.ContextManager {
	return a.core.GetContext()
}

// GetTools 获取工具管理�?
func (a *RuntimeAdapter) GetTools() *tools.Manager {
	return a.core.GetTools()
}

// GetSettings 获取设置管理�?
func (a *RuntimeAdapter) GetSettings() *settings.Manager {
	return a.core.GetSettingsManager()
}

// GetWorkspace 获取工作区管理器
func (a *RuntimeAdapter) GetWorkspace() *workspace.Manager {
	return a.core.GetWorkspace()
}

// GetModelInfo 获取当前模型信息
func (a *RuntimeAdapter) GetModelInfo() (modelName, modelBase string) {
	return a.core.ModelName(), a.core.ModelBase()
}

// ResolveAPIConfig 解析 API 配置
func (a *RuntimeAdapter) ResolveAPIConfig() (base, provider, model, key string) {
	return a.core.ResolveAPIConfig()
}

// SetActiveModel 设置活动模型
func (a *RuntimeAdapter) SetActiveModel(name string) bool {
	return a.core.SetActiveModel(name)
}

// GetCore 获取 RuntimeCore 实例（用于高级操作）
func (a *RuntimeAdapter) GetCore() *bridge.RuntimeCore {
	return a.core
}

// Reload 重新加载运行时
func (a *RuntimeAdapter) Reload() error {
	return a.core.Reload()
}

