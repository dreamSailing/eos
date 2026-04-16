package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// mcpListPromptsStructured handles the mcp_list_prompts tool
func (m *Manager) mcpListPromptsStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	if m.mcpManager == nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolMCPListPrompts,
			Status:  "error",
			Error:   "MCP not initialized",
			Display: "Error: MCP manager is not available",
		}
	}

	allPrompts := m.mcpManager.GetAllPrompts()
	if len(allPrompts) == 0 {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolMCPListPrompts,
			Status:  "success",
			Data:    map[string]interface{}{"prompts": []interface{}{}, "count": 0},
			Display: "No MCP prompts available",
		}
	}

	type promptInfo struct {
		Server      string `json:"server"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	}

	var prompts []promptInfo
	for server, plist := range allPrompts {
		for _, p := range plist {
			pi := promptInfo{
				Server:      server,
				Name:        p.Name,
				Description: p.Description,
			}
			prompts = append(prompts, pi)
		}
	}

	sort.Slice(prompts, func(i, j int) bool {
		if prompts[i].Server != prompts[j].Server {
			return prompts[i].Server < prompts[j].Server
		}
		return prompts[i].Name < prompts[j].Name
	})

	raw, _ := json.Marshal(prompts)
	var rawList []interface{}
	json.Unmarshal(raw, &rawList)

	var displayLines []string
	for _, p := range prompts {
		line := fmt.Sprintf("- [%s] %s", p.Server, p.Name)
		if p.Description != "" {
			line += ": " + p.Description
		}
		displayLines = append(displayLines, line)
	}

	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolMCPListPrompts,
		Status:  "success",
		Data:    map[string]interface{}{"prompts": rawList, "count": len(prompts)},
		Display: fmt.Sprintf("Available MCP prompts (%d):\n%s", len(prompts), strings.Join(displayLines, "\n")),
	}
}

// mcpGetPromptStructured handles the mcp_get_prompt tool
func (m *Manager) mcpGetPromptStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	if m.mcpManager == nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolMCPGetPrompt,
			Status:  "error",
			Error:   "MCP not initialized",
			Display: "Error: MCP manager is not available",
		}
	}

	serverName, _ := params["server"].(string)
	promptName, _ := params["name"].(string)

	if strings.TrimSpace(serverName) == "" {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolMCPGetPrompt,
			Status:  "error",
			Error:   "server parameter is required",
			Display: "Error: specify the MCP server name",
		}
	}
	if strings.TrimSpace(promptName) == "" {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolMCPGetPrompt,
			Status:  "error",
			Error:   "name parameter is required",
			Display: "Error: specify the prompt name",
		}
	}

	// Extract optional arguments
	var args map[string]interface{}
	if raw, ok := params["arguments"]; ok {
		if m, ok := raw.(map[string]interface{}); ok {
			args = m
		}
	}

	result, err := m.mcpManager.GetPrompt(ctx, serverName, promptName, args)
	if err != nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolMCPGetPrompt,
			Status:  "error",
			Error:   err.Error(),
			Display: fmt.Sprintf("Error fetching prompt %s from %s: %s", promptName, serverName, err.Error()),
		}
	}

	raw, _ := json.Marshal(result)
	var resultData interface{}
	json.Unmarshal(raw, &resultData)

	display := fmt.Sprintf("Prompt: %s (from %s)", promptName, serverName)
	if result != nil && result.Description != "" {
		display += "\n" + result.Description
	}

	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolMCPGetPrompt,
		Status:  "success",
		Data:    map[string]interface{}{"prompt": resultData},
		Display: display,
	}
}
