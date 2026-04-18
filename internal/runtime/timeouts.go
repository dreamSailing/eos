package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"time"
	"github.com/dreamSailing/eos/internal/config"
)

func resolveAgentToolTimeout(cfg config.Config) time.Duration {
	tt := 2 * time.Hour
	if cfg.Agent.ToolTimeoutSeconds > 0 {
		tt = time.Duration(cfg.Agent.ToolTimeoutSeconds) * time.Second
	}
	if tt < 30*time.Second {
		tt = 30 * time.Second
	}
	if tt > 30*24*time.Hour {
		tt = 30 * 24 * time.Hour
	}
	return tt
}
