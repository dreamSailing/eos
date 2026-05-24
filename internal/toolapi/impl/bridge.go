package impl

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"

	"github.com/dreamSailing/eos/internal/toolapi"
)

// ToolExecutorBridge abstracts the tool execution engine so that executor.go
// does not directly depend on internal/tools. The legacy implementation in
// legacy_bridge.go delegates to tools.Manager; future implementations may
// route through coreapi.Engine or other backends.
type ToolExecutorBridge interface {
	Execute(ctx context.Context, sess toolapi.ExecSession, calls []toolapi.ToolCall) ([]toolapi.ToolResult, error)
}

// TaskSourceBridge abstracts task listing and lifecycle management so that
// tasks.go does not directly depend on internal/tools, internal/tools/bg,
// or internal/runtime. The legacy implementation in legacy_bridge.go
// aggregates from bg.Default(), tools.DefaultTodoStore(), and
// runtime.DefaultAgentRegistry().
type TaskSourceBridge interface {
	List(ctx context.Context) ([]toolapi.TaskInfo, error)
	Kill(ctx context.Context, id string) error
	Resume(ctx context.Context, id string) error
	Close(ctx context.Context, id string) error
}
