package shell

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"os"
	"strings"
)

type ctxKey string

const ctxKeyEnv ctxKey = "eos.shell.env"

func WithEnv(ctx context.Context, env []string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(env) == 0 {
		return ctx
	}
	cp := append([]string(nil), env...)
	return context.WithValue(ctx, ctxKeyEnv, cp)
}

func envFromContext(ctx context.Context) []string {
	if ctx != nil {
		if v, ok := ctx.Value(ctxKeyEnv).([]string); ok && len(v) > 0 {
			return append([]string(nil), v...)
		}
	}
	return os.Environ()
}

func mergeEnv(base []string, add []string) []string {
	if len(add) == 0 {
		return append([]string(nil), base...)
	}
	out := append([]string(nil), base...)
	for _, entry := range add {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		prefix := strings.ToUpper(key) + "="
		replaced := false
		for i := range out {
			if strings.HasPrefix(strings.ToUpper(out[i]), prefix) {
				out[i] = entry
				replaced = true
				break
			}
		}
		if !replaced {
			out = append(out, entry)
		}
	}
	return out
}
