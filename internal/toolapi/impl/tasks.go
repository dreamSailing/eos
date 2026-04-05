package impl

import (
	"context"
	"fmt"
	"time"

	"github.com/dreamSailing/vb-coding/internal/runtime"
	"github.com/dreamSailing/vb-coding/internal/toolapi"
	"github.com/dreamSailing/vb-coding/internal/tools"
	"github.com/dreamSailing/vb-coding/internal/tools/bg"
)

type tasks struct{}

func newTasks() toolapi.Tasks {
	return &tasks{}
}

func (t *tasks) List(_ context.Context) ([]toolapi.TaskInfo, error) {
	shellTasks := bg.Default().List()
	todoItems := tools.DefaultTodoStore().List()
	agentTasks := runtime.DefaultAgentRegistry().ListSnapshots()

	out := make([]toolapi.TaskInfo, 0, len(shellTasks)+len(todoItems)+len(agentTasks))
	for _, it := range shellTasks {
		out = append(out, toolapi.TaskInfo{
			ID:        it.ID,
			Kind:      "shell_task",
			Status:    string(it.Status),
			StartedAt: it.StartedAt,
			UpdatedAt: maxTaskTime(it.StartedAt, it.ExitedAt),
			EndedAt:   it.ExitedAt,
			Label:     it.Command,
			Summary:   it.Error,
			CanKill:   it.Status == bg.StatusRunning,
		})
	}
	for idx, it := range todoItems {
		id := it.ID
		if id == "" {
			id = fmt.Sprintf("todo_%d", idx+1)
		}
		out = append(out, toolapi.TaskInfo{
			ID:        id,
			Kind:      "todo_item",
			Status:    it.Status,
			StartedAt: it.UpdatedAt,
			UpdatedAt: it.UpdatedAt,
			Label:     it.Content,
			Metadata: map[string]any{
				"priority": it.Priority,
			},
		})
	}
	for _, it := range agentTasks {
		out = append(out, toolapi.TaskInfo{
			ID:        it.ID,
			Kind:      "agent_task",
			Status:    string(it.Status),
			StartedAt: it.StartedAt,
			UpdatedAt: it.UpdatedAt,
			EndedAt:   it.CompletedAt,
			Label:     it.Task,
			Summary:   it.Result,
			CanKill:   it.Status == runtime.AgentStatusRunning,
			CanResume: it.CanResume,
			CanClose:  it.CanClose,
			Metadata: map[string]any{
				"agent_name":       it.Name,
				"context_strategy": it.Strategy,
				"messages":         it.Messages,
				"error":            it.Error,
				"allowed_tools":    append([]string(nil), it.AllowedTools...),
			},
		})
	}
	return out, nil
}

func (t *tasks) Kill(_ context.Context, id string) error {
	if _, err := bg.Default().Kill(id); err == nil {
		return nil
	}
	if runtime.DefaultAgentRegistry().RequestCancel(id) {
		return nil
	}
	return fmt.Errorf("task not found: %s", id)
}

func (t *tasks) Resume(_ context.Context, id string) error {
	if runtime.DefaultAgentRegistry().Resume(id, "") {
		return nil
	}
	return fmt.Errorf("task not resumable: %s", id)
}

func (t *tasks) Close(_ context.Context, id string) error {
	if runtime.DefaultAgentRegistry().Close(id) {
		return nil
	}
	return fmt.Errorf("task not closeable: %s", id)
}

func maxTaskTime(times ...time.Time) time.Time {
	var out time.Time
	for _, ts := range times {
		if ts.After(out) {
			out = ts
		}
	}
	return out
}
