package toolapi

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


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
	Resume(ctx context.Context, id string) error
	Close(ctx context.Context, id string) error
}

type Services interface {
	NewExecutor(workspaceRoot string) Executor
	Catalog() Catalog
	Tasks() Tasks
}
