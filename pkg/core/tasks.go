package core

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"errors"
	"slices"
	"strings"
	"time"
)

type BackgroundTask struct {
	ID        string
	Status    string
	StartedAt time.Time
	Label     string
	CanKill   bool
	Workspace string
}

type TodoItem struct {
	ID        string
	Content   string
	Status    string
	Priority  any
	UpdatedAt time.Time
}

type taskBackend interface {
	ListTasks() []BackgroundTask
	ListTodos() []TodoItem
	TailTask(taskID string, fromSeq int64, limit int) ([]string, error)
	KillTask(taskID string) error
	CleanupFinishedTasks() int
}

func (r *Runtime) taskBackend() taskBackend {
	if r != nil && r.tasks != nil {
		return r.tasks
	}
	return defaultTaskBackend{}
}

func (r *Runtime) ListTasks() []BackgroundTask {
	items := append([]BackgroundTask(nil), r.taskBackend().ListTasks()...)
	sortBackgroundTasks(items)
	return items
}

func (r *Runtime) ListTodos() []TodoItem {
	return append([]TodoItem(nil), r.taskBackend().ListTodos()...)
}

func (r *Runtime) TailTask(taskID string) ([]string, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, errors.New("task id required")
	}
	return r.taskBackend().TailTask(taskID, 0, 200)
}

func (r *Runtime) KillTask(taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return errors.New("task id required")
	}
	if err := r.taskBackend().KillTask(taskID); err != nil {
		return err
	}
	r.notifyStateChanged(StateTopicTasks, "task.kill")
	return nil
}

func (r *Runtime) CleanupTasks() int {
	n := r.taskBackend().CleanupFinishedTasks()
	if n > 0 {
		r.notifyStateChanged(StateTopicTasks, "task.cleanup")
	}
	return n
}

func sortBackgroundTasks(items []BackgroundTask) {
	slices.SortFunc(items, func(a, b BackgroundTask) int {
		if a.StartedAt.After(b.StartedAt) {
			return -1
		}
		if a.StartedAt.Before(b.StartedAt) {
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
}
