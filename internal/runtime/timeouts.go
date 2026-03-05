package runtime

import (
	"time"
	"github.com/dreamSailing/vb-coding/internal/config"
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
