package coreapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/dreamSailing/eos/pkg/agentcore"
	"github.com/dreamSailing/eos/pkg/protocol"
)

type AgentTurnModelRunner struct {
	Turns     TurnService
	Events    EventSubscriber
	SessionID string
}

func NewAgentTurnModelRunner(turns TurnService, events EventSubscriber) AgentTurnModelRunner {
	return AgentTurnModelRunner{Turns: turns, Events: events}
}

func (r AgentTurnModelRunner) WithSession(sessionID string) AgentTurnModelRunner {
	r.SessionID = strings.TrimSpace(sessionID)
	return r
}

func (r AgentTurnModelRunner) RunModel(ctx context.Context, req agentcore.ModelRequest) (agentcore.ModelResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if r.Turns == nil {
		return agentcore.ModelResponse{}, fmt.Errorf("agent model runner: %w", ErrUnsupported)
	}
	sessionID := strings.TrimSpace(r.SessionID)
	if sessionID == "" {
		return agentcore.ModelResponse{}, errors.New("agent model runner: session_id is required")
	}
	turnID := defaultAgentTurnID(req.Agent.ID)
	var (
		events <-chan protocol.Envelope
	)
	if r.Events != nil {
		ch, err := r.Events.Subscribe(ctx, EventFilter{
			SessionID: sessionID,
			TurnID:    turnID,
			AgentID:   strings.TrimSpace(req.Agent.ID),
		})
		if err != nil {
			return agentcore.ModelResponse{}, err
		}
		events = ch
	}
	turn, err := r.Turns.Start(ctx, StartTurnRequest{
		SessionID: sessionID,
		TurnID:    turnID,
		Input:     BuildAgentModelInput(req),
		Options:   append(json.RawMessage(nil), req.Options...),
	})
	if err != nil {
		return agentcore.ModelResponse{}, err
	}
	if events == nil {
		return agentcore.ModelResponse{Status: agentcore.AgentRunning}, nil
	}
	return r.waitForTurn(ctx, turn, events)
}

func (r AgentTurnModelRunner) waitForTurn(ctx context.Context, turn Turn, events <-chan protocol.Envelope) (agentcore.ModelResponse, error) {
	var finalText string
	for {
		select {
		case <-ctx.Done():
			_ = r.Turns.Interrupt(context.Background(), TurnRef{SessionID: turn.SessionID, TurnID: turn.ID})
			return agentcore.ModelResponse{Text: finalText, Status: agentcore.AgentCancelled}, ctx.Err()
		case ev, ok := <-events:
			if !ok {
				return agentcore.ModelResponse{Text: finalText, Status: agentcore.AgentCompleted}, nil
			}
			switch ev.EventType {
			case protocol.EventTypeTextFinal:
				finalText = eventPayloadString(ev.Payload, "text", "message")
			case protocol.EventTypeRequestDone:
				if text := eventPayloadString(ev.Payload, "text", "message", "output"); text != "" {
					finalText = text
				}
				return agentcore.ModelResponse{Text: finalText, Status: agentcore.AgentCompleted}, nil
			case protocol.EventTypeRequestFailed:
				errText := eventPayloadString(ev.Payload, "error", "message", "text")
				if errText == "" {
					errText = "agent model turn failed"
				}
				return agentcore.ModelResponse{Text: finalText, Status: agentcore.AgentFailed}, errors.New(errText)
			}
		}
	}
}

func BuildAgentModelInput(req agentcore.ModelRequest) string {
	var b strings.Builder
	roleID := strings.TrimSpace(req.Role.ID)
	if roleID != "" {
		b.WriteString("Role: ")
		b.WriteString(roleID)
		b.WriteString("\n")
	}
	if prompt := strings.TrimSpace(req.Role.SystemPrompt); prompt != "" {
		b.WriteString("System prompt:\n")
		b.WriteString(prompt)
		b.WriteString("\n")
	}
	if promptFile := strings.TrimSpace(req.Role.PromptFile); promptFile != "" {
		b.WriteString("Prompt file: ")
		b.WriteString(promptFile)
		b.WriteString("\n")
	}
	if req.ContextStrategy != "" {
		b.WriteString("Context strategy: ")
		b.WriteString(string(req.ContextStrategy))
		b.WriteString("\n")
	}
	if len(req.AllowedTools) > 0 {
		b.WriteString("Allowed tools: ")
		b.WriteString(strings.Join(req.AllowedTools, ", "))
		b.WriteString("\n")
	}
	if model := strings.TrimSpace(req.Model); model != "" {
		b.WriteString("Preferred model: ")
		b.WriteString(model)
		b.WriteString("\n")
	}
	if effort := strings.TrimSpace(req.ReasoningEffort); effort != "" {
		b.WriteString("Reasoning effort: ")
		b.WriteString(effort)
		b.WriteString("\n")
	}
	if task := strings.TrimSpace(req.Task); task != "" {
		b.WriteString("\nTask:\n")
		b.WriteString(task)
		b.WriteString("\n")
	}
	if len(req.Messages) > 0 {
		b.WriteString("\nMailbox:\n")
		for _, msg := range req.Messages {
			body := strings.TrimSpace(msg.Body)
			if body == "" {
				continue
			}
			from := strings.TrimSpace(msg.FromAgentID)
			if from == "" {
				from = "unknown"
			}
			b.WriteString("- ")
			b.WriteString(from)
			b.WriteString(": ")
			b.WriteString(body)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func defaultAgentTurnID(agentID string) string {
	agentID = sanitizeAgentRunnerIDPart(agentID)
	if agentID == "" {
		return "agent_turn"
	}
	return "agent_turn_" + agentID
}

func sanitizeAgentRunnerIDPart(value string) string {
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

func eventPayloadString(payload map[string]any, keys ...string) string {
	if len(payload) == 0 {
		return ""
	}
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		text, ok := value.(string)
		if !ok {
			continue
		}
		if text = strings.TrimSpace(text); text != "" {
			return text
		}
	}
	return ""
}
