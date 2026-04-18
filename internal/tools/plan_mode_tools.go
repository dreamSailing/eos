package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"encoding/json"
	"fmt"
)

// OnModeChange callback for notifying mode changes to the bridge layer
var OnModeChange func(oldMode, newMode string)

// enterPlanModeStructured handles the enter_plan_mode tool
func (m *Manager) enterPlanModeStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	if OnModeChange == nil {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolEnterPlanMode,
			Status: "error",
			Error:  "mode change callback not registered",
		}
	}

	reason := ""
	if r, ok := params["reason"].(string); ok {
		reason = r
	}

	// Get current mode from the callback
	oldMode := ""
	if getCurrentMode := OnGetCurrentMode; getCurrentMode != nil {
		oldMode = getCurrentMode()
	}

	// Switch to plan mode
	OnModeChange(oldMode, "plan")

	resultMsg := "Entered plan mode. In this mode:\n"
	resultMsg += "- All write and dangerous operations will be denied\n"
	resultMsg += "- You can only read files, search, and plan\n"
	resultMsg += "- Use exit_plan_mode when your plan is ready\n"
	if reason != "" {
		resultMsg += fmt.Sprintf("\nReason: %s", reason)
	}

	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolEnterPlanMode,
		Status: "success",
		Data: map[string]interface{}{
			"content":  resultMsg,
			"old_mode": oldMode,
			"new_mode": "plan",
		},
		Display: "Entered plan mode",
	}
}

// exitPlanModeStructured handles the exit_plan_mode tool
func (m *Manager) exitPlanModeStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	if OnModeChange == nil {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolExitPlanMode,
			Status: "error",
			Error:  "mode change callback not registered",
		}
	}

	// Get the previous mode
	previousMode := "auto"
	if getPrev := OnGetPreviousMode; getPrev != nil {
		previousMode = getPrev()
	}

	// Switch back to previous mode
	OnModeChange("plan", previousMode)

	resultMsg := fmt.Sprintf("Exited plan mode. Restored mode: %s", previousMode)
	resultMsg += "\nYou can now execute operations normally."

	planSummary := ""
	if s, ok := params["plan_summary"].(string); ok {
		planSummary = s
	}
	if planSummary != "" {
		resultMsg += fmt.Sprintf("\n\nPlan summary:\n%s", planSummary)
	}

	bs, _ := json.Marshal(map[string]interface{}{
		"content":       resultMsg,
		"restored_mode": previousMode,
		"had_plan":      planSummary != "",
	})

	_ = bs // suppress unused warning

	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolExitPlanMode,
		Status: "success",
		Data: map[string]interface{}{
			"content":       resultMsg,
			"restored_mode": previousMode,
		},
		Display: fmt.Sprintf("Exited plan mode → %s", previousMode),
	}
}

// OnGetCurrentMode callback to retrieve the current execution mode
var OnGetCurrentMode func() string

// OnGetPreviousMode callback to retrieve the saved previous mode
var OnGetPreviousMode func() string
