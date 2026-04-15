package session

import (
	"strings"
)

// ReactiveCompactionConfig holds configuration for reactive compression triggers
type ReactiveCompactionConfig struct {
	WarnThreshold    float64 // Trigger AI compression at this usage ratio (default 0.80)
	EmergencyThreshold float64 // Trigger emergency trimming at this ratio (default 0.95)
}

// DefaultReactiveConfig returns the default reactive compression configuration
func DefaultReactiveConfig() ReactiveCompactionConfig {
	return ReactiveCompactionConfig{
		WarnThreshold:      0.80,
		EmergencyThreshold: 0.95,
	}
}

// ReactiveCompact checks token usage and triggers appropriate compression.
// Returns the compression action taken: "none", "ai_compress", "emergency_trim"
func (c *ContextManager) ReactiveCompact(cfg ReactiveCompactionConfig, summarizeFunc func(string) (string, error)) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.autoCompressEnabled {
		return "none"
	}

	usageRatio := c.usageRatioLocked()

	if usageRatio >= cfg.EmergencyThreshold {
		// Emergency: aggressive trimming
		c.aggressiveCompactLocked(int(float64(c.maxPromptTokens) * 0.70))
		c.compressionStats.Strategy = CompressionAggressive
		return "emergency_trim"
	}

	if usageRatio >= cfg.WarnThreshold {
		// Warning: try AI-driven compression, fallback to standard compact
		if summarizeFunc != nil && len(c.recent) > aiCompressKeepRounds {
			head := c.recent[:len(c.recent)-aiCompressKeepRounds]
			var sb strings.Builder
			for _, m := range head {
				sb.WriteString(formatCompactLine(m))
				sb.WriteString("\n")
			}

			summary, err := summarizeFunc(sb.String())
			if err == nil && strings.TrimSpace(summary) != "" {
				_ = summary // Used above
			}
		}

		c.compactLocked()
		c.compressionStats.Strategy = CompressionBalanced
		return "ai_compress"
	}

	return "none"
}

// CheckAndCompact is a convenience method that should be called before each graph invoke.
// It uses default thresholds.
func (c *ContextManager) CheckAndCompact(summarizeFunc func(string) (string, error)) string {
	return c.ReactiveCompact(DefaultReactiveConfig(), summarizeFunc)
}

// usageRatioLocked returns current token usage as a ratio of max (0.0 - 1.0+)
func (c *ContextManager) usageRatioLocked() float64 {
	if c.maxPromptTokens <= 0 {
		return 0
	}
	total := c.estimateTotalTokensLocked()
	return float64(total) / float64(c.maxPromptTokens)
}
