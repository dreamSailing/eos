package adapter

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"github.com/dreamSailing/eos/internal/bridge"
	"github.com/dreamSailing/eos/internal/pkg/settings"
	"github.com/dreamSailing/eos/internal/pkg/workspace"
	"github.com/dreamSailing/eos/internal/session"
	"github.com/dreamSailing/eos/internal/tools"
	"sync"
)

// RuntimeAdapter �?RuntimeCore �?Bubble Tea UI 之间的适配�?
type RuntimeAdapter struct {
	core       *bridge.RuntimeCore
	eventsOnce sync.Once
	eventsCh   chan bridge.Event
}

// NewRuntimeAdapter 创建新的 RuntimeAdapter 实例
func NewRuntimeAdapter(core *bridge.RuntimeCore) *RuntimeAdapter {
	return &RuntimeAdapter{core: core}
}

// Events 返回运行时事件通道
func (a *RuntimeAdapter) Events() <-chan bridge.Event {
	a.eventsOnce.Do(func() {
		a.eventsCh = make(chan bridge.Event, 128)
		go func() {
			defer close(a.eventsCh)
			for event := range a.core.Events() {
				a.eventsCh <- normalizeRuntimeEvent(event)
			}
		}()
	})
	return a.eventsCh
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
