package toolapi

import "context"

type Executor interface {
	Execute(ctx context.Context, sess ExecSession, calls []ToolCall) ([]ToolResult, error)
}

type Catalog interface {
	List(ctx context.Context) ([]ToolDefinition, error)
	RiskLevel(toolName string) RiskLevel
}

type Tasks interface {
	List(ctx context.Context) ([]TaskInfo, error)
	Kill(ctx context.Context, id string) error
}

type Services interface {
	NewExecutor(workspaceRoot string) Executor
	Catalog() Catalog
	Tasks() Tasks
}

