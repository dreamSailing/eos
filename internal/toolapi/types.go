package toolapi

import "time"

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

type ExecSession struct {
	WorkspaceRoot string
	AllowedTools  map[string]bool
	TraceID       string
}

type ToolCall struct {
	ID     string
	Name   string
	Params map[string]any
}

type ToolResult struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Tool    string         `json:"tool"`
	Status  string         `json:"status"`
	Data    map[string]any `json:"data,omitempty"`
	Error   string         `json:"error,omitempty"`
	Display string         `json:"display,omitempty"`
	Ts      int64          `json:"ts,omitempty"`
}

type ParameterInfo struct {
	Type     string
	Required bool
	Desc     string
}

type ToolExample struct {
	Description string
	Input       map[string]any
}

type ToolDefinition struct {
	Name        string
	Description string
	RiskLevel   RiskLevel
	Params      map[string]ParameterInfo
	Examples    []ToolExample
}

type TaskInfo struct {
	ID        string
	Status    string
	StartedAt time.Time
	Label     string
	CanKill   bool
}

