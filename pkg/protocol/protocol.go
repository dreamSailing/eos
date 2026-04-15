package protocol

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Version string

const (
	VersionV1 Version = "v1"
)

type Source string

const (
	SourceCore   Source = "core"
	SourceCLI    Source = "cli"
	SourceGUI    Source = "gui"
	SourceServe  Source = "serve"
	SourceAgent  Source = "agent"
	SourceTool   Source = "tool"
	SourceSystem Source = "system"
)

type EventType string

const (
	EventTypeTextDelta      EventType = "text.delta"
	EventTypeTextFinal      EventType = "text.final"
	EventTypeTextReasoning  EventType = "text.reasoning"
	EventTypeToolCall       EventType = "tool.call"
	EventTypeToolResult     EventType = "tool.result"
	EventTypeToolStep       EventType = "tool.step"
	EventTypeApprovalReq    EventType = "approval.required"
	EventTypeApprovalDone   EventType = "approval.resolved"
	EventTypeInquiryReq     EventType = "inquiry.required"
	EventTypeInquiryDone    EventType = "inquiry.resolved"
	EventTypeRequestStarted EventType = "request.started"
	EventTypeRequestDone    EventType = "request.completed"
	EventTypeRequestFailed  EventType = "request.failed"
	EventTypeSessionUpdated EventType = "session.updated"
	EventTypeAgentStarted   EventType = "agent.started"
	EventTypeAgentProgress  EventType = "agent.progress"
	EventTypeAgentDone      EventType = "agent.completed"
	EventTypeAgentFailed    EventType = "agent.failed"
	EventTypeAgentCancelled EventType = "agent.cancelled"
	EventTypeTaskStarted    EventType = "task.started"
	EventTypeTaskUpdated    EventType = "task.updated"
	EventTypeTaskDone       EventType = "task.completed"
	EventTypeTaskFailed     EventType = "task.failed"
	EventTypeModeChanged    EventType = "mode.changed"
	EventTypeBudgetUpdated  EventType = "budget.updated"
	EventTypeMemorySuggestion EventType = "memory.suggestion"
)

type Envelope struct {
	Version       Version        `json:"version"`
	EventID       string         `json:"event_id"`
	EventType     EventType      `json:"event_type"`
	SessionID     string         `json:"session_id,omitempty"`
	ThreadID      string         `json:"thread_id,omitempty"`
	RequestID     string         `json:"request_id,omitempty"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	Timestamp     time.Time      `json:"timestamp"`
	Source        Source         `json:"source"`
	Payload       map[string]any `json:"payload,omitempty"`
}

type EventOptions struct {
	EventID       string
	SessionID     string
	ThreadID      string
	RequestID     string
	CorrelationID string
	Timestamp     time.Time
	Source        Source
	Payload       map[string]any
}

func NewEvent(eventType EventType, opts EventOptions) Envelope {
	eventID := strings.TrimSpace(opts.EventID)
	if eventID == "" {
		eventID = "evt_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}

	ts := opts.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	src := opts.Source
	if strings.TrimSpace(string(src)) == "" {
		src = SourceCore
	}

	return Envelope{
		Version:       VersionV1,
		EventID:       eventID,
		EventType:     eventType,
		SessionID:     strings.TrimSpace(opts.SessionID),
		ThreadID:      strings.TrimSpace(opts.ThreadID),
		RequestID:     strings.TrimSpace(opts.RequestID),
		CorrelationID: strings.TrimSpace(opts.CorrelationID),
		Timestamp:     ts,
		Source:        src,
		Payload:       ClonePayload(opts.Payload),
	}
}

func ClonePayload(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
