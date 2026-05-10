package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func mustNotPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	fn()
}

func TestNewTokenBudget(t *testing.T) {
	tb := NewTokenBudget()
	if tb.WarnThreshold != 0.80 {
		t.Errorf("expected warn threshold 0.80, got %f", tb.WarnThreshold)
	}
	status := tb.Check()
	if status != BudgetOK {
		t.Errorf("empty budget should be OK, got %v", status)
	}
}

func TestTokenBudgetUnlimited(t *testing.T) {
	tb := NewTokenBudget()
	// No limits set, should always be OK
	tb.RecordUsage(10000, 10000)
	tb.RecordUsage(10000, 10000)
	tb.RecordUsage(10000, 10000)

	status := tb.Check()
	if status != BudgetOK {
		t.Errorf("unlimited budget should always be OK, got %v", status)
	}
}

func TestTokenBudgetSessionExceeded(t *testing.T) {
	tb := NewTokenBudget()
	tb.SetBudgets(0, 100) // 100 session tokens max

	tb.RecordUsage(30, 30) // total: 60, ratio: 0.6
	status := tb.Check()
	if status != BudgetOK {
		t.Errorf("at 60%% should be OK, got %v", status)
	}

	tb.RecordUsage(20, 20) // total: 100, ratio: 1.0, exceeded
	status = tb.Check()
	if status != BudgetExceeded {
		t.Errorf("exceeding budget should be Exceeded, got %v", status)
	}
}

func TestTokenBudgetTurnWarning(t *testing.T) {
	tb := NewTokenBudget()
	tb.SetBudgets(100, 0) // 100 turn tokens, unlimited session

	tb.RecordUsage(40, 40)
	status := tb.Check()
	if status != BudgetWarn {
		t.Errorf("at 80%% of turn budget should be Warn, got %v", status)
	}
}

func TestTokenBudgetResetTurn(t *testing.T) {
	tb := NewTokenBudget()
	tb.SetBudgets(100, 0)

	tb.RecordUsage(50, 50)
	tb.ResetTurn()

	status := tb.Check()
	if status != BudgetOK {
		t.Errorf("after reset turn should be OK, got %v", status)
	}

	// Session total should still be tracked
	_, _, sessionTotal, _, _ := tb.Snapshot()
	if sessionTotal != 100 {
		t.Errorf("session total should be 100, got %d", sessionTotal)
	}
}

func TestTokenBudgetSnapshot(t *testing.T) {
	tb := NewTokenBudget()
	tb.SetBudgets(200, 1000)
	tb.RecordUsage(30, 20)

	turnIn, turnOut, sessionTotal, maxTurn, maxSession := tb.Snapshot()
	if turnIn != 30 {
		t.Errorf("turn input should be 30, got %d", turnIn)
	}
	if turnOut != 20 {
		t.Errorf("turn output should be 20, got %d", turnOut)
	}
	if sessionTotal != 50 {
		t.Errorf("session total should be 50, got %d", sessionTotal)
	}
	if maxTurn != 200 {
		t.Errorf("max turn should be 200, got %d", maxTurn)
	}
	if maxSession != 1000 {
		t.Errorf("max session should be 1000, got %d", maxSession)
	}
}

func TestTokenBudgetUsageRatio(t *testing.T) {
	tb := NewTokenBudget()
	tb.SetBudgets(0, 1000)
	tb.RecordUsage(300, 200)

	ratio := tb.UsageRatio()
	if ratio != 0.5 {
		t.Errorf("expected ratio 0.5, got %f", ratio)
	}
}

func TestRuntimeCoreRecordTokenUsageWithoutBudgetIsOK(t *testing.T) {
	rc := &RuntimeCore{}

	var status TokenBudgetStatus
	mustNotPanic(t, func() {
		status = rc.RecordTokenUsage(10, 20)
	})

	if status != BudgetOK {
		t.Fatalf("expected BudgetOK without token budget, got %v", status)
	}
}

func TestRuntimeCoreResetTurnBudgetWithoutBudgetDoesNotPanic(t *testing.T) {
	rc := &RuntimeCore{}

	mustNotPanic(t, func() {
		rc.ResetTurnBudget()
	})
}

func TestRuntimeCoreRecordTokenUsageTracksTurnAndSession(t *testing.T) {
	rc := &RuntimeCore{
		eventsCh: make(chan Event, 4),
	}
	rc.InitializeTokenBudget(1000, 5000)

	var status TokenBudgetStatus
	mustNotPanic(t, func() {
		status = rc.RecordTokenUsage(30, 20)
	})

	if status != BudgetOK {
		t.Fatalf("expected BudgetOK, got %v", status)
	}

	turnIn, turnOut, sessionTotal, _, _ := rc.tokenBudget.Snapshot()
	if turnIn != 30 || turnOut != 20 {
		t.Fatalf("expected turn usage 30/20, got %d/%d", turnIn, turnOut)
	}
	if sessionTotal != 50 {
		t.Fatalf("expected session total 50, got %d", sessionTotal)
	}
}

func TestRuntimeCoreResetTurnBudgetClearsTurnOnly(t *testing.T) {
	rc := &RuntimeCore{
		eventsCh: make(chan Event, 4),
	}
	rc.InitializeTokenBudget(1000, 5000)

	mustNotPanic(t, func() {
		rc.RecordTokenUsage(40, 10)
	})

	mustNotPanic(t, func() {
		rc.ResetTurnBudget()
	})

	turnIn, turnOut, sessionTotal, _, _ := rc.tokenBudget.Snapshot()
	if turnIn != 0 || turnOut != 0 {
		t.Fatalf("expected turn usage reset to 0/0, got %d/%d", turnIn, turnOut)
	}
	if sessionTotal != 50 {
		t.Fatalf("expected session total to remain 50, got %d", sessionTotal)
	}
}

func TestRuntimeCoreAddTokenRecordWithModelLeavesUsageUnknownWhenMissing(t *testing.T) {
	rc := &RuntimeCore{}
	rc.AddTokenRecordWithModel(nil, "deepseek-v4-flash")

	history := rc.GetTokenHistory()
	if len(history) != 1 {
		t.Fatalf("len(history)=%d, want 1", len(history))
	}
	if history[0].Input != nil || history[0].Reply != nil || history[0].Total != nil || history[0].CostUSD != nil {
		t.Fatalf("expected unknown usage/cost, got %#v", history[0])
	}
}

func TestRuntimeCoreAddTokenRecordWithModelUsesProviderUsage(t *testing.T) {
	rc := &RuntimeCore{}
	rc.AddTokenRecordWithModel(&schema.TokenUsage{
		PromptTokens:     1000,
		CompletionTokens: 200,
		TotalTokens:      1200,
	}, "qwen3.6-plus")

	stats := rc.GetTokenStats()
	if stats.Input == nil || *stats.Input != 1000 {
		t.Fatalf("stats.Input=%v, want 1000", stats.Input)
	}
	if stats.Reply == nil || *stats.Reply != 200 {
		t.Fatalf("stats.Reply=%v, want 200", stats.Reply)
	}
	if stats.Total == nil || *stats.Total != 1200 {
		t.Fatalf("stats.Total=%v, want 1200", stats.Total)
	}
	if stats.TotalCostUSD == nil || *stats.TotalCostUSD <= 0 {
		t.Fatalf("stats.TotalCostUSD=%v, want > 0", stats.TotalCostUSD)
	}
}
