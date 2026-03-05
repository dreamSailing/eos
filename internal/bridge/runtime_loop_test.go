package bridge

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRuntimeCore_CleanupPendingRequestsDeliversErrors(t *testing.T) {
	rc := &RuntimeCore{
		done:          make(chan struct{}),
		pendingReload: make(map[chan error]struct{}),
		pendingGraph:  make(map[chan graphInvokeRes]struct{}),
		pendingTools:  make(map[chan toolsNodeRes]struct{}),
		pendingSumm:   make(map[chan summarizeRes]struct{}),
	}

	rch := make(chan error, 1)
	gch := make(chan graphInvokeRes, 1)
	tch := make(chan toolsNodeRes, 1)
	sch := make(chan summarizeRes, 1)

	rc.addPendingReload(rch)
	rc.addPendingGraph(gch)
	rc.addPendingTools(tch)
	rc.addPendingSummarize(sch)

	rc.cleanupPendingRequests()

	select {
	case err := <-rch:
		if !errors.Is(err, ErrRuntimeLoopUnavailable) {
			t.Fatalf("unexpected err: %v", err)
		}
	default:
		t.Fatalf("expected reload error")
	}

	select {
	case res := <-gch:
		if !errors.Is(res.err, ErrRuntimeLoopUnavailable) {
			t.Fatalf("unexpected err: %v", res.err)
		}
	default:
		t.Fatalf("expected graph error")
	}

	select {
	case res := <-tch:
		if res.executed || res.cont || len(res.results) != 0 {
			t.Fatalf("unexpected tools result: %#v", res)
		}
	default:
		t.Fatalf("expected tools result")
	}

	select {
	case res := <-sch:
		if !errors.Is(res.err, ErrRuntimeLoopUnavailable) {
			t.Fatalf("unexpected err: %v", res.err)
		}
	default:
		t.Fatalf("expected summarize error")
	}
}

func TestRuntimeCore_ShouldRestartAfterPanicStopsAfterThreshold(t *testing.T) {
	rc := &RuntimeCore{}
	rc.panicAt = time.Now()
	if !rc.shouldRestartAfterPanic() {
		t.Fatalf("expected restart allowed")
	}
	if !rc.shouldRestartAfterPanic() {
		t.Fatalf("expected restart allowed")
	}
	if !rc.shouldRestartAfterPanic() {
		t.Fatalf("expected restart allowed")
	}
	if rc.shouldRestartAfterPanic() {
		t.Fatalf("expected restart blocked after too many panics")
	}
}

func TestWithDefaultTimeoutKeepsDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	out, outCancel := withDefaultTimeout(ctx, 3*time.Minute)
	defer outCancel()
	if _, ok := out.Deadline(); !ok {
		t.Fatalf("expected deadline")
	}
}
