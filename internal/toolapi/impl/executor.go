package impl

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"

	"github.com/dreamSailing/eos/internal/toolapi"
)

type executor struct {
	bridge ToolExecutorBridge
}

func newExecutor(bridge ToolExecutorBridge) toolapi.Executor {
	return &executor{bridge: bridge}
}

func (e *executor) Execute(ctx context.Context, sess toolapi.ExecSession, calls []toolapi.ToolCall) ([]toolapi.ToolResult, error) {
	if e == nil || e.bridge == nil {
		return nil, fmt.Errorf("executor not initialized")
	}
	return e.bridge.Execute(ctx, sess, calls)
}
