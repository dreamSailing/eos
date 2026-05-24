package coreapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/dreamSailing/eos/pkg/agentcore"
)

type AgentToolRunner struct {
	Executor  ToolExecutor
	SessionID string
	TurnID    string
}

func NewAgentToolRunner(executor ToolExecutor) AgentToolRunner {
	return AgentToolRunner{Executor: executor}
}

func (r AgentToolRunner) WithSession(sessionID, turnID string) AgentToolRunner {
	r.SessionID = strings.TrimSpace(sessionID)
	r.TurnID = strings.TrimSpace(turnID)
	return r
}

func (r AgentToolRunner) RunTool(ctx context.Context, call agentcore.ToolCall) (agentcore.ToolOutput, error) {
	if r.Executor == nil {
		return agentcore.ToolOutput{}, fmt.Errorf("agent tool runner: %w", ErrUnsupported)
	}
	name := strings.TrimSpace(call.Name)
	if name == "" {
		return agentcore.ToolOutput{}, fmt.Errorf("agent tool runner: tool name required")
	}
	requestID := strings.TrimSpace(r.TurnID)
	if requestID == "" {
		requestID = defaultAgentToolRequestID(call.Agent.ID, name)
	}
	result, err := r.Executor.Execute(ctx, ToolRequest{
		SessionID: strings.TrimSpace(r.SessionID),
		TurnID:    strings.TrimSpace(r.TurnID),
		RequestID: requestID,
		AgentID:   strings.TrimSpace(call.Agent.ID),
		Name:      name,
		Args:      append(json.RawMessage(nil), call.Args...),
	})
	output := agentcore.ToolOutput{
		Name:    strings.TrimSpace(result.Name),
		Display: strings.TrimSpace(result.Display),
		Output:  append(json.RawMessage(nil), result.Output...),
		Error:   strings.TrimSpace(result.Error),
	}
	if output.Name == "" {
		output.Name = name
	}
	if err != nil {
		if output.Error == "" {
			output.Error = err.Error()
		}
		return output, err
	}
	return output, nil
}

func defaultAgentToolRequestID(agentID, name string) string {
	agentID = sanitizeRequestIDPart(agentID)
	name = sanitizeRequestIDPart(name)
	switch {
	case agentID != "" && name != "":
		return "agent_" + agentID + "_" + name
	case agentID != "":
		return "agent_" + agentID
	case name != "":
		return "agent_tool_" + name
	default:
		return "agent_tool"
	}
}

func sanitizeRequestIDPart(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}
