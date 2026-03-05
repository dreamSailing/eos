package tools

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

