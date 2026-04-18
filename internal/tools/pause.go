package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
)

type PauseController interface {
	Pause()
	Resume()
}

type pauseControllerKey struct{}

func WithPauseController(ctx context.Context, pc PauseController) context.Context {
	if ctx == nil || pc == nil {
		return ctx
	}
	return context.WithValue(ctx, pauseControllerKey{}, pc)
}

func PauseControllerFromContext(ctx context.Context) PauseController {
	if ctx == nil {
		return nil
	}
	if v := ctx.Value(pauseControllerKey{}); v != nil {
		if pc, ok := v.(PauseController); ok {
			return pc
		}
	}
	return nil
}

