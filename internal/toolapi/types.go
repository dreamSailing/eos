package toolapi

import "time"

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

type CapabilitySource string

const (
	SourceBuiltin CapabilitySource = "builtin"
	SourceRuntime CapabilitySource = "runtime"
	SourceAgent   CapabilitySource = "agent"
	SourceSkill   CapabilitySource = "skill"
	SourcePlugin  CapabilitySource = "plugin"
	SourceMCP     CapabilitySource = "mcp"
	SourceLSP     CapabilitySource = "lsp"
)

type ExecSession struct {
	WorkspaceRoot         string
	AllowedTools          map[string]bool
	TraceID               string
	ExecutionMode         string
	RequireApprovalDigest bool
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
	Source      CapabilitySource
	Category    string
	VisibleIn   []string
	ReadOnly    bool
	Invocable   bool
	Tags        []string
	Metadata    map[string]any
}

type TaskInfo struct {
	ID        string
	Kind      string
	Status    string
	StartedAt time.Time
	UpdatedAt time.Time
	EndedAt   time.Time
	Label     string
	Summary   string
	CanKill   bool
	CanResume bool
	CanClose  bool
	Metadata  map[string]any
}
