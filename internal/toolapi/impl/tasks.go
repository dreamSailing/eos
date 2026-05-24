package impl

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"

	"github.com/dreamSailing/eos/internal/toolapi"
)

type tasks struct {
	bridge TaskSourceBridge
}

func newTasks(bridge TaskSourceBridge) toolapi.Tasks {
	return &tasks{bridge: bridge}
}

func (t *tasks) List(ctx context.Context) ([]toolapi.TaskInfo, error) {
	return t.bridge.List(ctx)
}

func (t *tasks) Kill(ctx context.Context, id string) error {
	return t.bridge.Kill(ctx, id)
}

func (t *tasks) Resume(ctx context.Context, id string) error {
	return t.bridge.Resume(ctx, id)
}

func (t *tasks) Close(ctx context.Context, id string) error {
	return t.bridge.Close(ctx, id)
}
