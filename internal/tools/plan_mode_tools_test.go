package tools

import (
	"context"
	"strings"
	"testing"
)

func TestEnterPlanModeTool(t *testing.T) {
	mgr := NewManager()

	var modeChangeCalled bool
	var capturedNew string
	OnModeChange = func(_, new_ string) {
		modeChangeCalled = true
		capturedNew = new_
	}
	defer func() { OnModeChange = nil }()

	result := mgr.enterPlanModeStructured(context.Background(), map[string]interface{}{
		"reason": "need to plan refactoring",
	})

	if result.Status != "success" {
		t.Errorf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !modeChangeCalled {
		t.Error("expected mode change callback to be called")
	}
	if capturedNew != "plan" {
		t.Errorf("expected new mode 'plan', got %q", capturedNew)
	}
	if !strings.Contains(result.Data["content"].(string), "plan mode") {
		t.Error("result should mention plan mode")
	}
}

func TestExitPlanModeTool(t *testing.T) {
	mgr := NewManager()

	var modeChangeCalled bool
	OnModeChange = func(old, new_ string) {
		modeChangeCalled = true
	}
	OnGetPreviousMode = func() string { return "auto" }
	defer func() {
		OnModeChange = nil
		OnGetPreviousMode = nil
	}()

	result := mgr.exitPlanModeStructured(context.Background(), map[string]interface{}{
		"plan_summary": "Refactor auth module into separate package",
	})

	if result.Status != "success" {
		t.Errorf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !modeChangeCalled {
		t.Error("expected mode change callback to be called")
	}
	if !strings.Contains(result.Data["restored_mode"].(string), "auto") {
		t.Error("should restore to auto mode")
	}
}

func TestEnterPlanModeNoCallback(t *testing.T) {
	mgr := NewManager()
	OnModeChange = nil

	result := mgr.enterPlanModeStructured(context.Background(), map[string]interface{}{})
	if result.Status != "error" {
		t.Errorf("expected error when no callback, got %s", result.Status)
	}
}

func TestExitPlanModeNoCallback(t *testing.T) {
	mgr := NewManager()
	OnModeChange = nil

	result := mgr.exitPlanModeStructured(context.Background(), map[string]interface{}{})
	if result.Status != "error" {
		t.Errorf("expected error when no callback, got %s", result.Status)
	}
}
