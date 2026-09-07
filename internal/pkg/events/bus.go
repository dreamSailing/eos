package events

// Copyright (c) 2026 EOSAIOS
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"sync"
)

// Event 基础事件结构
type Event struct {
	Topic string
	Data  any
}

// Handler 事件处理器函数
type Handler func(ev Event)

// Bus 事件总线接口
type Bus interface {
	Publish(topic string, data any)
	Subscribe(topic string, handler Handler)
	Unsubscribe(topic string, handler Handler)
}

// MemoryBus 内存中的事件总线实现
type MemoryBus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

// NewMemoryBus 创建一个新的内存事件总线
func NewMemoryBus() *MemoryBus {
	return &MemoryBus{
		handlers: make(map[string][]Handler),
	}
}

// Publish 发布事件到指定主题
func (b *MemoryBus) Publish(topic string, data any) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	ev := Event{Topic: topic, Data: data}
	if handlers, ok := b.handlers[topic]; ok {
		for _, h := range handlers {
			// 异步执行处理器，防止阻塞发布者
			go h(ev)
		}
	}

	// 同时也发布到通配符主题 "*"
	if handlers, ok := b.handlers["*"]; ok {
		for _, h := range handlers {
			go h(ev)
		}
	}
}

// Subscribe 订阅指定主题的事件
func (b *MemoryBus) Subscribe(topic string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[topic] = append(b.handlers[topic], handler)
}

// Unsubscribe 取消订阅指定主题的事件
func (b *MemoryBus) Unsubscribe(topic string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if handlers, ok := b.handlers[topic]; ok {
		for i, h := range handlers {
			// 比较函数地址（注意：在 Go 中比较函数可能有限制，通常建议使用 token 或 id）
			// 这里简单处理，实际应用中可能需要更完善的取消订阅机制
			if &h == &handler {
				b.handlers[topic] = append(handlers[:i], handlers[i+1:]...)
				break
			}
		}
	}
}

// 全局单例总线，方便在整个项目中使用
var (
	GlobalBus = NewMemoryBus()
)

// Publish 全局发布函数
func Publish(topic string, data any) {
	GlobalBus.Publish(topic, data)
}

// Subscribe 全局订阅函数
func Subscribe(topic string, handler Handler) {
	GlobalBus.Subscribe(topic, handler)
}
