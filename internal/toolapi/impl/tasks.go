package impl

import (
	"context"

	"github.com/dreamSailing/vb-coding/internal/toolapi"
	"github.com/dreamSailing/vb-coding/internal/tools/bg"
)

type tasks struct{}

func newTasks() toolapi.Tasks {
	return &tasks{}
}

func (t *tasks) List(_ context.Context) ([]toolapi.TaskInfo, error) {
	items := bg.Default().List()
	out := make([]toolapi.TaskInfo, 0, len(items))
	for _, it := range items {
		out = append(out, toolapi.TaskInfo{
			ID:        it.ID,
			Status:    string(it.Status),
			StartedAt: it.StartedAt,
			Label:     it.Command,
			CanKill:   it.Status == bg.StatusRunning,
		})
	}
	return out, nil
}

func (t *tasks) Kill(_ context.Context, id string) error {
	_, err := bg.Default().Kill(id)
	return err
}

