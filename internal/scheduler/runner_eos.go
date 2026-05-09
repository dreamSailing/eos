package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/toolapi"
)

func runEOSSchedule(ctx context.Context, services toolapi.Services, item Schedule, workspace string) error {
	if services == nil {
		return fmt.Errorf("services unavailable")
	}
	toolName, _ := item.Payload["tool"].(string)
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return fmt.Errorf("tool required")
	}
	params := map[string]any{}
	if raw, ok := item.Payload["parameters"].(map[string]any); ok {
		for k, v := range raw {
			params[k] = v
		}
	}
	executor := services.NewExecutor(workspace)
	results, err := executor.Execute(ctx, toolapi.ExecSession{
		WorkspaceRoot: workspace,
		TraceID:       "schedule_" + item.ID + "_" + time.Now().Format("150405"),
	}, []toolapi.ToolCall{{
		ID:     item.ID,
		Name:   toolName,
		Params: params,
	}})
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return fmt.Errorf("empty tool result")
	}
	res := results[0]
	if strings.EqualFold(strings.TrimSpace(res.Status), "success") || strings.TrimSpace(res.Status) == "" {
		return nil
	}
	if strings.TrimSpace(res.Error) != "" {
		return errors.New(strings.TrimSpace(res.Error))
	}
	if strings.TrimSpace(res.Display) != "" {
		return errors.New(strings.TrimSpace(res.Display))
	}
	return fmt.Errorf("tool status: %s", res.Status)
}
