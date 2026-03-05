package tools

import (
	"context"
	"testing"
)

func TestPlanStepsBasic(t *testing.T) {
	mgr := NewManager()
	req := "为依赖图工具添加跨工作区支持并补充测试"
	r := mgr.execStructured(context.Background(), ToolCall{Tool: "plan_steps", Parameters: map[string]interface{}{"user_request": req, "max_steps": 5, "context_k": 3, "neighbors_depth": 1}})
	if r.Status != "success" {
		t.Fatalf("plan_steps failed: %v", r.Error)
	}
	steps, ok := r.Data["steps"].([]map[string]interface{})
	if !ok || len(steps) == 0 {
		t.Fatalf("expected non-empty steps")
	}
	if _, ok := r.Data["context"].(map[string]interface{}); !ok {
		t.Fatalf("expected context")
	}
}
