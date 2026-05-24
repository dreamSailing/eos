package core

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"github.com/dreamSailing/eos/internal/tools"
	"github.com/dreamSailing/eos/internal/tools/bg"
)

type defaultTaskBackend struct{}

func (defaultTaskBackend) ListTasks() []BackgroundTask {
	items := bg.Default().List()
	out := make([]BackgroundTask, 0, len(items))
	for _, t := range items {
		out = append(out, BackgroundTask{
			ID:        t.ID,
			Status:    string(t.Status),
			StartedAt: t.StartedAt,
			Label:     t.Command,
			CanKill:   t.Status == bg.StatusRunning,
			Workspace: normalizeWorkspacePath(t.WorkingDir),
		})
	}
	return out
}

func (defaultTaskBackend) ListTodos() []TodoItem {
	items := tools.DefaultTodoStore().List()
	out := make([]TodoItem, 0, len(items))
	for _, item := range items {
		out = append(out, TodoItem{
			ID:        item.ID,
			Content:   item.Content,
			Status:    item.Status,
			Priority:  item.Priority,
			UpdatedAt: item.UpdatedAt,
		})
	}
	return out
}

func (defaultTaskBackend) TailTask(taskID string, fromSeq int64, limit int) ([]string, error) {
	res, err := bg.Default().Tail(taskID, &bg.TailOptions{FromSeq: fromSeq, Limit: limit})
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(res.Entries))
	for _, e := range res.Entries {
		lines = append(lines, e.Stream+": "+e.Line)
	}
	return lines, nil
}

func (defaultTaskBackend) KillTask(taskID string) error {
	_, err := bg.Default().Kill(taskID)
	return err
}

func (defaultTaskBackend) CleanupFinishedTasks() int {
	return bg.Default().CleanupFinished()
}

var _ taskBackend = defaultTaskBackend{}
