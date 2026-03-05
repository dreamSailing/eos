package bridge

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/dreamSailing/vb-coding/internal/config"
	einoruntime "github.com/dreamSailing/vb-coding/internal/runtime"
	"github.com/dreamSailing/vb-coding/internal/tools"

	"github.com/cloudwego/eino/schema"
)

func (rc *RuntimeCore) graphInvokeWithRetry(ctx context.Context, rt *einoruntime.EinoRuntime, query, executionMode string, imagePaths []string) (*schema.Message, error) {
	if rt == nil {
		return nil, ErrRuntimeLoopUnavailable
	}

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
		if ne.Timeout() || ne.Temporary() {
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
	return false
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
