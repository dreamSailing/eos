package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/dreamSailing/eos/internal/pkg/utils"
)

// Team represents a named group of collaborating agents
type Team struct {
	Name     string
	Agents   []TeamAgent
	Messages []TeamMessage
	mu       sync.RWMutex
}

// TeamAgent describes an agent in a team
type TeamAgent struct {
	Role         string `json:"role"`
	SubagentType string `json:"subagent_type"`
	Prompt       string `json:"prompt"`
}

// TeamMessage is a message passed between team agents
type TeamMessage struct {
	From string `json:"from"`
	To   string `json:"to"`
	Text string `json:"text"`
}

var (
	teams   map[string]*Team
	teamsMu sync.RWMutex
)

func init() {
	teams = make(map[string]*Team)
}

// teamCreateStructured creates a new named team with configured agent roles
func (m *Manager) teamCreateStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	name, _ := params["name"].(string)
	if name == "" {
		return ToolResult{Type: "tool_result", Tool: ToolTeamCreate, Status: "error", Error: "name parameter is required"}
	}

	agentsRaw, _ := params["agents"].([]interface{})
	if len(agentsRaw) == 0 {
		return ToolResult{Type: "tool_result", Tool: ToolTeamCreate, Status: "error", Error: "agents parameter is required and must be non-empty"}
	}

	var agents []TeamAgent
	for _, a := range agentsRaw {
		agentMap, ok := a.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := agentMap["role"].(string)
		prompt, _ := agentMap["prompt"].(string)
		subType, _ := agentMap["subagent_type"].(string)
		if role == "" {
			continue
		}
		agents = append(agents, TeamAgent{
			Role:         role,
			SubagentType: subType,
			Prompt:       prompt,
		})
	}

	if len(agents) == 0 {
		return ToolResult{Type: "tool_result", Tool: ToolTeamCreate, Status: "error", Error: "no valid agent configurations provided"}
	}

	teamsMu.Lock()
	defer teamsMu.Unlock()

	if _, exists := teams[name]; exists {
		return ToolResult{Type: "tool_result", Tool: ToolTeamCreate, Status: "error", Error: fmt.Sprintf("team %q already exists", name)}
	}

	teams[name] = &Team{
		Name:   name,
		Agents: agents,
	}

	slog.Info("tools.team.create", "component", utils.ComponentTool, "team", name, "agents", len(agents))

	agentNames := make([]string, len(agents))
	for i, a := range agents {
		agentNames[i] = a.Role
	}
	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolTeamCreate,
		Status:  "success",
		Data:    map[string]interface{}{"team": name, "agents": agentNames},
		Display: fmt.Sprintf("Created team %q with %d agents: %v", name, len(agents), agentNames),
	}
}

// teamDeleteStructured stops and cleans up a team
func (m *Manager) teamDeleteStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	name, _ := params["name"].(string)
	if name == "" {
		return ToolResult{Type: "tool_result", Tool: ToolTeamDelete, Status: "error", Error: "name parameter is required"}
	}

	teamsMu.Lock()
	defer teamsMu.Unlock()

	if _, exists := teams[name]; !exists {
		return ToolResult{Type: "tool_result", Tool: ToolTeamDelete, Status: "error", Error: fmt.Sprintf("team %q not found", name)}
	}

	delete(teams, name)
	slog.Info("tools.team.delete", "component", utils.ComponentTool, "team", name)

	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolTeamDelete,
		Status:  "success",
		Data:    map[string]interface{}{"team": name},
		Display: fmt.Sprintf("Deleted team %q", name),
	}
}

// teamSendMessageStructured sends a message between agents within a team
func (m *Manager) teamSendMessageStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	teamName, _ := params["team"].(string)
	fromAgent, _ := params["from_agent"].(string)
	toAgent, _ := params["to_agent"].(string)
	message, _ := params["message"].(string)

	if teamName == "" {
		return ToolResult{Type: "tool_result", Tool: ToolTeamSendMsg, Status: "error", Error: "team parameter is required"}
	}
	if fromAgent == "" {
		return ToolResult{Type: "tool_result", Tool: ToolTeamSendMsg, Status: "error", Error: "from_agent parameter is required"}
	}
	if toAgent == "" {
		return ToolResult{Type: "tool_result", Tool: ToolTeamSendMsg, Status: "error", Error: "to_agent parameter is required"}
	}
	if message == "" {
		return ToolResult{Type: "tool_result", Tool: ToolTeamSendMsg, Status: "error", Error: "message parameter is required"}
	}

	teamsMu.Lock()
	defer teamsMu.Unlock()

	team, exists := teams[teamName]
	if !exists {
		return ToolResult{Type: "tool_result", Tool: ToolTeamSendMsg, Status: "error", Error: fmt.Sprintf("team %q not found", teamName)}
	}

	// Verify agents exist in team
	fromExists := false
	toExists := false
	for _, a := range team.Agents {
		if a.Role == fromAgent {
			fromExists = true
		}
		if a.Role == toAgent {
			toExists = true
		}
	}
	if !fromExists {
		return ToolResult{Type: "tool_result", Tool: ToolTeamSendMsg, Status: "error", Error: fmt.Sprintf("agent %q not found in team %q", fromAgent, teamName)}
	}
	if !toExists {
		return ToolResult{Type: "tool_result", Tool: ToolTeamSendMsg, Status: "error", Error: fmt.Sprintf("agent %q not found in team %q", toAgent, teamName)}
	}

	team.Messages = append(team.Messages, TeamMessage{
		From: fromAgent,
		To:   toAgent,
		Text: message,
	})

	slog.Debug("tools.team.send_message", "component", utils.ComponentTool, "team", teamName, "from", fromAgent, "to", toAgent)

	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolTeamSendMsg,
		Status:  "success",
		Data:    map[string]interface{}{"team": teamName, "from": fromAgent, "to": toAgent, "message": message},
		Display: fmt.Sprintf("[%s] %s -> %s: %s", teamName, fromAgent, toAgent, truncateString(message, 100)),
	}
}

// GetTeam returns a team by name (for external access)
func GetTeam(name string) *Team {
	teamsMu.RLock()
	defer teamsMu.RUnlock()
	return teams[name]
}

// ListTeams returns all active team names
func ListTeams() []string {
	teamsMu.RLock()
	defer teamsMu.RUnlock()
	names := make([]string, 0, len(teams))
	for name := range teams {
		names = append(names, name)
	}
	return names
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
