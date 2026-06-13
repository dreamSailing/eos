//go:build legacy

package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/pkg/utils"
	einoruntime "github.com/dreamSailing/eos/internal/runtime"
	"github.com/dreamSailing/eos/internal/tools"

	"github.com/cloudwego/eino/schema"
)

func (rc *RuntimeCore) graphInvokeWithRetry(ctx context.Context, rt *einoruntime.EinoRuntime, query, executionMode string, imagePaths []string) (*schema.Message, error) {
	if rt == nil {
		return nil, ErrRuntimeLoopUnavailable
	}
	settings := rc.GetSettings()
	ctx = einoruntime.WithPlanPromptStyle(ctx, settings.PlanPromptStyle)

	timeout := 7 * 24 * time.Hour
	if cfg, _ := config.Load(); cfg.Agent.InvokeTimeoutSeconds > 0 {
		timeout = time.Duration(cfg.Agent.InvokeTimeoutSeconds) * time.Second
	}
	if timeout < 30*time.Second {
		timeout = 30 * time.Second
	}
	if timeout > 30*24*time.Hour {
		timeout = 30 * 24 * time.Hour
	}
	ctx, pt := withPausableTimeout(ctx, timeout)
	ctx = tools.WithPauseController(ctx, pt)

	maxAttempts := 3
	baseDelay := 500 * time.Millisecond
	maxDelay := 5 * time.Second
	mult := 2.0

	consecutive529 := 0
	max529BeforeDowngrade := 3

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		msg, err := rt.GraphInvokeWithImages(ctx, query, executionMode, imagePaths)
		if err == nil {
			return msg, nil
		}
		lastErr = err

		if errors.Is(err, context.Canceled) {
			return nil, err
		}

		// Handle context overflow: auto-reduce context
		if isContextOverflowError(err) {
			slog.Warn("runtime.retry.context_overflow", "component", utils.ComponentSystem, "attempt", attempt)
			if rc.cm != nil {
				rc.cm.SnipToolOutputs()
				rc.cm.SnipAllToolOutputs()
			}
			if attempt == maxAttempts {
				return nil, err
			}
			delay := calcBackoff(baseDelay, maxDelay, mult, attempt-1)
			rc.emitMeta("phase.note:Context overflow detected, reducing context size and retrying...")
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			continue
		}

		// Handle max_output_tokens: continue the conversation
		if isMaxOutputTokensError(err) && attempt < maxAttempts {
			slog.Warn("runtime.retry.max_output_tokens", "component", utils.ComponentSystem, "attempt", attempt)
			delay := calcBackoff(baseDelay, maxDelay, mult, attempt-1)
			rc.emitMeta("phase.note:Output length limit reached, continuing generation...")
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			continue
		}

		// Handle 529 consecutive errors: attempt model downgrade
		if is529Error(err) {
			consecutive529++
			if consecutive529 >= max529BeforeDowngrade {
				slog.Warn("runtime.retry.529_downgrade", "component", utils.ComponentSystem, "consecutive", consecutive529)
				rc.emitMeta("phase.note:Server overloaded multiple times, consider switching to a backup model")
				// Don't auto-downgrade, just inform the user
				return nil, err
			}
		} else {
			consecutive529 = 0
		}

		if !isRetryableModelError(err) || attempt == maxAttempts {
			return nil, err
		}

		delay := calcBackoff(baseDelay, maxDelay, mult, attempt-1)
		rc.emitMeta("phase.note:AI 请求失败，将在 " + delay.String() + " 后重试 (" + itoa(attempt+1) + "/" + itoa(maxAttempts) + "): " + compactErr(err))

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

func isRetryableModelError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) {
		if ne.Timeout() {
			return true
		}
	}

	s := strings.ToLower(err.Error())
	if strings.Contains(s, "429") || strings.Contains(s, "too many requests") || strings.Contains(s, "rate limit") {
		return true
	}
	if strings.Contains(s, "500") || strings.Contains(s, "502") || strings.Contains(s, "503") || strings.Contains(s, "504") {
		return true
	}
	if strings.Contains(s, "bad gateway") || strings.Contains(s, "service unavailable") || strings.Contains(s, "gateway timeout") {
		return true
	}
	if strings.Contains(s, "timeout") || strings.Contains(s, "deadline exceeded") {
		return true
	}
	if strings.Contains(s, "connection reset") || strings.Contains(s, "connection refused") || strings.Contains(s, "eof") || strings.Contains(s, "tls handshake timeout") {
		return true
	}
	// 529 errors: site overloaded
	if strings.Contains(s, "529") || strings.Contains(s, "overloaded") {
		return true
	}
	return false
}

// isContextOverflowError checks if the error indicates the context window was exceeded
func isContextOverflowError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "input length and max_tokens exceed context limit") ||
		strings.Contains(s, "context window") ||
		strings.Contains(s, "maximum context length") ||
		strings.Contains(s, "token limit") ||
		strings.Contains(s, "too many tokens")
}

// isMaxOutputTokensError checks if the error indicates max_output_tokens was hit
func isMaxOutputTokensError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "max_output_tokens") ||
		strings.Contains(s, "output length") ||
		strings.Contains(s, "finish_reason\": \"length\"") ||
		strings.Contains(s, "stop reason: length") ||
		strings.Contains(s, "\"stop_reason\":\"length\"")
}

// is529Error checks for consecutive 529 (overloaded) errors
func is529Error(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "529") || strings.Contains(s, "overloaded")
}

func withDefaultTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), d)
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}

func calcBackoff(baseDelay, maxDelay time.Duration, multiplier float64, attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	pow := math.Pow(multiplier, float64(attempt))
	delay := time.Duration(float64(baseDelay) * pow)
	if delay > maxDelay {
		delay = maxDelay
	}
	jitter := 0.85 + rand.Float64()*0.3
	return time.Duration(float64(delay) * jitter)
}

func compactErr(err error) string {
	if err == nil {
		return ""
	}
	s := strings.TrimSpace(err.Error())
	if len(s) > 160 {
		return s[:160] + "…"
	}
	return s
}

func itoa(v int) string {
	return strconv.Itoa(v)
}
