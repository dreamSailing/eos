//go:build legacy

package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"log/slog"
	"time"

	"github.com/dreamSailing/eos/internal/pkg/utils"
)

func (rc *RuntimeCore) emitMeta(line string) {
	rc.mu.RLock()
	cb := rc.onMeta
	rc.mu.RUnlock()
	if cb != nil {
		cb(line)
	}
}

func (rc *RuntimeCore) addPendingReload(ch chan error) {
	rc.pendingMu.Lock()
	rc.pendingReload[ch] = struct{}{}
	rc.pendingMu.Unlock()
}

func (rc *RuntimeCore) removePendingReload(ch chan error) {
	rc.pendingMu.Lock()
	delete(rc.pendingReload, ch)
	rc.pendingMu.Unlock()
}

func (rc *RuntimeCore) addPendingGraph(ch chan graphInvokeRes) {
	rc.pendingMu.Lock()
	rc.pendingGraph[ch] = struct{}{}
	rc.pendingMu.Unlock()
}

func (rc *RuntimeCore) removePendingGraph(ch chan graphInvokeRes) {
	rc.pendingMu.Lock()
	delete(rc.pendingGraph, ch)
	rc.pendingMu.Unlock()
}

func (rc *RuntimeCore) addPendingTools(ch chan toolsNodeRes) {
	rc.pendingMu.Lock()
	rc.pendingTools[ch] = struct{}{}
	rc.pendingMu.Unlock()
}

func (rc *RuntimeCore) removePendingTools(ch chan toolsNodeRes) {
	rc.pendingMu.Lock()
	delete(rc.pendingTools, ch)
	rc.pendingMu.Unlock()
}

func (rc *RuntimeCore) addPendingSummarize(ch chan summarizeRes) {
	rc.pendingMu.Lock()
	rc.pendingSumm[ch] = struct{}{}
	rc.pendingMu.Unlock()
}

func (rc *RuntimeCore) removePendingSummarize(ch chan summarizeRes) {
	rc.pendingMu.Lock()
	delete(rc.pendingSumm, ch)
	rc.pendingMu.Unlock()
}

func (rc *RuntimeCore) addPendingPredict(ch chan predictNextRes) {
	rc.pendingMu.Lock()
	rc.pendingPred[ch] = struct{}{}
	rc.pendingMu.Unlock()
}

func (rc *RuntimeCore) removePendingPredict(ch chan predictNextRes) {
	rc.pendingMu.Lock()
	delete(rc.pendingPred, ch)
	rc.pendingMu.Unlock()
}

func (rc *RuntimeCore) shouldRestartAfterPanic() bool {
	rc.panicMu.Lock()
	defer rc.panicMu.Unlock()

	now := time.Now()
	if !rc.panicAt.IsZero() && now.Sub(rc.panicAt) < 10*time.Second {
		rc.panicHits++
	} else {
		rc.panicHits = 1
	}
	rc.panicAt = now

	return rc.panicHits <= 3
}

func (rc *RuntimeCore) restartLoopAsync() {
	select {
	case <-rc.done:
		return
	default:
	}
	if !rc.shouldRestartAfterPanic() {
		slog.Error("runtime.loop.restart.skipped",
			"component", utils.ComponentSystem,
			"reason", "too_many_panics",
			"panic_hits", rc.panicHits,
		)
		return
	}
	slog.Warn("runtime.loop.restart",
		"component", utils.ComponentSystem,
	)
	rc.wg.Add(1)
	go rc.loop()
}
