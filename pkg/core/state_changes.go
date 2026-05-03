package core

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"strings"
	"sync"
	"time"
)

const (
	StateTopicRuntime   = "runtime"
	StateTopicModels    = "models"
	StateTopicTasks     = "tasks"
	StateTopicWorkspace = "workspace"
	StateTopicSessions  = "sessions"
	StateTopicMCP       = "mcp"
	StateTopicRules     = "rules"
	StateTopicSettings  = "settings"
	StateTopicVersions  = "versions"
	StateTopicLSP       = "lsp"
	StateTopicSkills    = "skills"
	StateTopicPlugins   = "plugins"
	StateTopicContext   = "context"
	StateTopicReview    = "review"
)

type StateChangeEvent struct {
	Topic  string
	Source string
	At     time.Time
}

func (r *Runtime) SubscribeStateChanges(buffer int) (<-chan StateChangeEvent, func()) {
	if buffer <= 0 {
		buffer = 16
	}
	ch := make(chan StateChangeEvent, buffer)
	if r == nil {
		close(ch)
		return ch, func() {}
	}

	r.stateChangesMu.Lock()
	if r.stateChangeSubscribers == nil {
		r.stateChangeSubscribers = map[int]chan StateChangeEvent{}
	}
	id := r.nextStateSubscriberID
	r.nextStateSubscriberID++
	r.stateChangeSubscribers[id] = ch
	r.stateChangesMu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			r.stateChangesMu.Lock()
			if cur, ok := r.stateChangeSubscribers[id]; ok {
				delete(r.stateChangeSubscribers, id)
				close(cur)
			}
			r.stateChangesMu.Unlock()
		})
	}
	return ch, unsubscribe
}

func (r *Runtime) notifyStateChanged(topic, source string) {
	if r == nil {
		return
	}
	event := StateChangeEvent{
		Topic:  strings.TrimSpace(topic),
		Source: strings.TrimSpace(source),
		At:     time.Now(),
	}
	if event.Topic == "" {
		event.Topic = StateTopicRuntime
	}
	if event.Source == "" {
		event.Source = event.Topic
	}

	r.stateChangesMu.RLock()
	defer r.stateChangesMu.RUnlock()
	for _, ch := range r.stateChangeSubscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (r *Runtime) closeStateChangeSubscribers() {
	if r == nil {
		return
	}
	r.stateChangesMu.Lock()
	defer r.stateChangesMu.Unlock()
	for id, ch := range r.stateChangeSubscribers {
		delete(r.stateChangeSubscribers, id)
		close(ch)
	}
}
