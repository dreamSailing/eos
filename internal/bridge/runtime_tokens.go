package bridge

import (
	"log/slog"
	"sync"
	"time"

	"github.com/dreamSailing/eos/internal/pkg/utils"
)

// TokenBudget manages per-turn and session-level token budgets
type TokenBudget struct {
	mu sync.RWMutex

	// Configuration
	MaxTurnTokens    int     // Maximum tokens per turn (0 = unlimited)
	MaxSessionTokens int     // Maximum tokens per session (0 = unlimited)
	WarnThreshold    float64 // Warning threshold ratio (default 0.80)

	// Current state
	TurnInputTokens  int
	TurnOutputTokens int
	SessionTotal     int
	SessionStart     time.Time

	// Callbacks
	OnWarn  func(usageRatio float64, turnUsed, sessionUsed int)
	OnExceed func(reason string, totalUsed int)
}

// TokenBudgetStatus represents the current budget check result
type TokenBudgetStatus string

const (
	BudgetOK       TokenBudgetStatus = "ok"
	BudgetWarn     TokenBudgetStatus = "warn"
	BudgetExceeded TokenBudgetStatus = "exceeded"
)

// NewTokenBudget creates a new TokenBudget with defaults
func NewTokenBudget() *TokenBudget {
	return &TokenBudget{
		WarnThreshold: 0.80,
		SessionStart:  time.Now(),
	}
}

// Check checks the current budget status before an operation
func (tb *TokenBudget) Check() TokenBudgetStatus {
	tb.mu.RLock()
	defer tb.mu.RUnlock()

	if tb.MaxSessionTokens > 0 {
		ratio := float64(tb.SessionTotal) / float64(tb.MaxSessionTokens)
		if ratio >= 1.0 {
			return BudgetExceeded
		}
		if ratio >= tb.WarnThreshold {
			return BudgetWarn
		}
	}

	if tb.MaxTurnTokens > 0 {
		turnTotal := tb.TurnInputTokens + tb.TurnOutputTokens
		ratio := float64(turnTotal) / float64(tb.MaxTurnTokens)
		if ratio >= 1.0 {
			return BudgetExceeded
		}
		if ratio >= tb.WarnThreshold {
			return BudgetWarn
		}
	}

	return BudgetOK
}

// RecordUsage records token usage after an operation
func (tb *TokenBudget) RecordUsage(inputTokens, outputTokens int) TokenBudgetStatus {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.TurnInputTokens += inputTokens
	tb.TurnOutputTokens += outputTokens
	tb.SessionTotal += inputTokens + outputTokens

	// Check thresholds
	status := BudgetOK

	if tb.MaxSessionTokens > 0 {
		ratio := float64(tb.SessionTotal) / float64(tb.MaxSessionTokens)
		if ratio >= 1.0 {
			status = BudgetExceeded
			if tb.OnExceed != nil {
				go tb.OnExceed("session budget exceeded", tb.SessionTotal)
			}
		} else if ratio >= tb.WarnThreshold && status == BudgetOK {
			status = BudgetWarn
			if tb.OnWarn != nil {
				go tb.OnWarn(ratio, tb.TurnInputTokens+tb.TurnOutputTokens, tb.SessionTotal)
			}
		}
	}

	if tb.MaxTurnTokens > 0 && status != BudgetExceeded {
		turnTotal := tb.TurnInputTokens + tb.TurnOutputTokens
		ratio := float64(turnTotal) / float64(tb.MaxTurnTokens)
		if ratio >= 1.0 {
			status = BudgetExceeded
			if tb.OnExceed != nil {
				go tb.OnExceed("turn budget exceeded", tb.SessionTotal)
			}
		} else if ratio >= tb.WarnThreshold && status == BudgetOK {
			status = BudgetWarn
			if tb.OnWarn != nil {
				go tb.OnWarn(ratio, turnTotal, tb.SessionTotal)
			}
		}
	}

	return status
}

// ResetTurn resets per-turn counters (called at the start of each turn)
func (tb *TokenBudget) ResetTurn() {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.TurnInputTokens = 0
	tb.TurnOutputTokens = 0
}

// SetBudgets configures the token budgets
func (tb *TokenBudget) SetBudgets(maxTurn, maxSession int) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.MaxTurnTokens = maxTurn
	tb.MaxSessionTokens = maxSession
}

// Snapshot returns a read-only copy of the budget state
func (tb *TokenBudget) Snapshot() (turnIn, turnOut, sessionTotal int, maxTurn, maxSession int) {
	tb.mu.RLock()
	defer tb.mu.RUnlock()
	return tb.TurnInputTokens, tb.TurnOutputTokens, tb.SessionTotal,
		tb.MaxTurnTokens, tb.MaxSessionTokens
}

// UsageRatio returns the session token usage ratio
func (tb *TokenBudget) UsageRatio() float64 {
	tb.mu.RLock()
	defer tb.mu.RUnlock()
	if tb.MaxSessionTokens <= 0 {
		return 0
	}
	return float64(tb.SessionTotal) / float64(tb.MaxSessionTokens)
}

// InitializeTokenBudget sets up the token budget for RuntimeCore
func (rc *RuntimeCore) InitializeTokenBudget(maxTurn, maxSession int) {
	rc.tokenMu.Lock()
	defer rc.tokenMu.Unlock()

	if rc.tokenBudget != nil {
		rc.tokenBudget.SetBudgets(maxTurn, maxSession)
		return
	}

	tb := NewTokenBudget()
	tb.SetBudgets(maxTurn, maxSession)
	tb.OnWarn = func(ratio float64, turnUsed, sessionUsed int) {
		slog.Warn("token_budget.warn",
			"component", utils.ComponentSystem,
			"usage_ratio", ratio,
			"turn_used", turnUsed,
			"session_used", sessionUsed,
		)
		rc.eventsCh <- Event{
			Type:    "budget.updated",
			Content: "token budget warning",
			Data: map[string]any{
				"status":       "warn",
				"usage_ratio":  ratio,
				"turn_used":    turnUsed,
				"session_used": sessionUsed,
			},
		}
	}
	tb.OnExceed = func(reason string, totalUsed int) {
		slog.Error("token_budget.exceeded",
			"component", utils.ComponentSystem,
			"reason", reason,
			"session_used", totalUsed,
		)
		rc.eventsCh <- Event{
			Type:    "budget.updated",
			Content: reason,
			Data: map[string]any{
				"status":       "exceeded",
				"reason":       reason,
				"session_used": totalUsed,
			},
		}
	}
	rc.tokenBudget = tb
}

// CheckTokenBudget checks the token budget before an operation
func (rc *RuntimeCore) CheckTokenBudget() TokenBudgetStatus {
	rc.tokenMu.RLock()
	tb := rc.tokenBudget
	rc.tokenMu.RUnlock()
	if tb == nil {
		return BudgetOK
	}
	return tb.Check()
}

// RecordTokenUsage records token usage after a graph invoke
func (rc *RuntimeCore) RecordTokenUsage(inputTokens, outputTokens int) TokenBudgetStatus {
	rc.tokenMu.Lock()
	tb := rc.tokenBudget
	rc.tokenMu.RUnlock()
	if tb == nil {
		return BudgetOK
	}
	return tb.RecordUsage(inputTokens, outputTokens)
}

// ResetTurnBudget resets the per-turn token budget
func (rc *RuntimeCore) ResetTurnBudget() {
	rc.tokenMu.Lock()
	tb := rc.tokenBudget
	rc.tokenMu.RUnlock()
	if tb != nil {
		tb.ResetTurn()
	}
}

// tokenBudget is stored in RuntimeCore (add field)
// Declared here to avoid circular reference with runtime_core.go field addition
