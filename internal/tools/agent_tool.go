package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// agentToolStructured handles the agent tool for delegating tasks to sub-agents
func (m *Manager) agentToolStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	prompt, _ := params["prompt"].(string)
	if strings.TrimSpace(prompt) == "" {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolAgent,
			Status: "error",
			Error:  "prompt is required",
			Display: "Error: prompt parameter is required for agent tool",
		}
	}

	subagentType, _ := params["subagent_type"].(string)
	if subagentType == "" {
		subagentType = "general-purpose"
	}

	runInBackground := false
	if bg, ok := params["run_in_background"].(bool); ok {
		runInBackground = bg
	}

	description, _ := params["description"].(string)
	model, _ := params["model"].(string)

	// Emit agent started event
	if OnAgentToolEvent != nil {
		OnAgentToolEvent("agent.started", map[string]interface{}{
			"subagent_type": subagentType,
			"prompt":        prompt,
			"background":    runInBackground,
			"description":   description,
		})
	}

	if runInBackground {
		return m.executeAgentBackground(ctx, prompt, subagentType, description, model)
	}

	return m.executeAgentSync(ctx, prompt, subagentType, description, model)
}

// executeAgentSync runs a sub-agent synchronously and collects the result
func (m *Manager) executeAgentSync(ctx context.Context, prompt, subagentType, description, model string) ToolResult {
	// Check if there's a registered executor for sub-agents
	if AgentToolExecutor == nil {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolAgent,
			Status: "error",
			Error:  "agent executor not registered",
			Display: "Error: sub-agent execution is not available",
		}
	}

	startTime := time.Now()
	result, err := AgentToolExecutor(ctx, prompt, subagentType, description, model)
	elapsed := time.Since(startTime)

	if err != nil {
		if OnAgentToolEvent != nil {
			OnAgentToolEvent("agent.failed", map[string]interface{}{
				"subagent_type": subagentType,
				"error":         err.Error(),
				"elapsed_ms":    elapsed.Milliseconds(),
			})
		}
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolAgent,
			Status: "error",
			Error:  err.Error(),
			Display: fmt.Sprintf("Agent (%s) failed: %s", subagentType, err.Error()),
		}
	}

	if OnAgentToolEvent != nil {
		OnAgentToolEvent("agent.completed", map[string]interface{}{
			"subagent_type": subagentType,
			"elapsed_ms":    elapsed.Milliseconds(),
		})
	}

	resultData := map[string]interface{}{
		"content":        result,
		"subagent_type":  subagentType,
		"elapsed_ms":     elapsed.Milliseconds(),
	}
	if description != "" {
		resultData["description"] = description
	}

	display := fmt.Sprintf("Agent (%s) completed in %v", subagentType, elapsed.Round(time.Millisecond))
	if len(result) > 200 {
		display += fmt.Sprintf(" (%d chars)", len(result))
	}

	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolAgent,
		Status:  "success",
		Data:    resultData,
		Display: display,
	}
}

// executeAgentBackground starts a sub-agent in the background
func (m *Manager) executeAgentBackground(ctx context.Context, prompt, subagentType, description, model string) ToolResult {
	if AgentToolBackgroundExecutor == nil {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolAgent,
			Status: "error",
			Error:  "background agent executor not registered",
			Display: "Error: background sub-agent execution is not available",
		}
	}

	taskID, err := AgentToolBackgroundExecutor(ctx, prompt, subagentType, description, model)
	if err != nil {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolAgent,
			Status: "error",
			Error:  err.Error(),
			Display: fmt.Sprintf("Failed to start background agent: %s", err.Error()),
		}
	}

	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolAgent,
		Status: "success",
		Data: map[string]interface{}{
			"content":       fmt.Sprintf("Background agent started with task ID: %s", taskID),
			"task_id":       taskID,
			"subagent_type": subagentType,
			"background":    true,
		},
		Display: fmt.Sprintf("Background agent (%s) started: %s", subagentType, taskID),
	}
}

// AgentToolExecutor is the callback for synchronous sub-agent execution
// Returns (result string, error)
var AgentToolExecutor func(ctx context.Context, prompt, subagentType, description, model string) (string, error)

// AgentToolBackgroundExecutor is the callback for async sub-agent execution
// Returns (taskID string, error)
var AgentToolBackgroundExecutor func(ctx context.Context, prompt, subagentType, description, model string) (string, error)

// OnAgentToolEvent is a callback for agent tool events (started/progress/completed/failed)
var OnAgentToolEvent func(eventType string, data map[string]interface{})

// buildAgentResultJSON builds a JSON result for the agent tool
func buildAgentResultJSON(result string, subagentType string) string {
	data := map[string]interface{}{
		"result":         result,
		"subagent_type":  subagentType,
	}
	bs, _ := json.Marshal(data)
	return string(bs)
}
