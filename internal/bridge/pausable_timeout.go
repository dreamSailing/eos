package bridge

import (
	"context"
	"sync"
	"time"
)

type pausableTimeout struct {
	parent context.Context
	ctx    context.Context
	cancel context.CancelFunc

	mu        sync.Mutex
	remaining time.Duration
	lastStart time.Time
	paused    bool
	timer     *time.Timer
	doneOnce  sync.Once
}

func withPausableTimeout(parent context.Context, d time.Duration) (context.Context, *pausableTimeout) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	pt := &pausableTimeout{
		parent:    parent,
		ctx:       ctx,
		cancel:    cancel,
		remaining: d,
		lastStart: time.Now(),
	}

	pt.startTimerLocked(d)

	go func() {
		select {
		case <-parent.Done():
			pt.doneOnce.Do(func() { cancel() })
		case <-ctx.Done():
		}
	}()

	return ctx, pt
}

func (pt *pausableTimeout) startTimerLocked(d time.Duration) {
	if d <= 0 {
		return
	}
	if pt.timer != nil {
		pt.timer.Stop()
	}
	pt.timer = time.AfterFunc(d, func() {
		pt.doneOnce.Do(func() {
			pt.cancel()
		})
	})
}

func (pt *pausableTimeout) Pause() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if pt.paused {
		return
	}
	if pt.timer != nil {
		pt.timer.Stop()
	}
	elapsed := time.Since(pt.lastStart)
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed >= pt.remaining {
		pt.remaining = 0
		pt.doneOnce.Do(func() { pt.cancel() })
		pt.paused = true
		return
	}
	pt.remaining -= elapsed
	pt.paused = true
}

func (pt *pausableTimeout) Resume() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if !pt.paused {
		return
	}
	if pt.remaining <= 0 {
		pt.doneOnce.Do(func() { pt.cancel() })
		return
	}
	pt.lastStart = time.Now()
	pt.paused = false
	pt.startTimerLocked(pt.remaining)
}

