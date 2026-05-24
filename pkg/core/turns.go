package core

import (
	"context"
	"fmt"
	"strings"
)

func (r *Runtime) registerActiveTurn(turnID string, cancel context.CancelFunc) error {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return fmt.Errorf("turn id required")
	}
	if cancel == nil {
		return fmt.Errorf("turn cancel func required")
	}
	r.activeTurnsMu.Lock()
	defer r.activeTurnsMu.Unlock()
	if r.activeTurns == nil {
		r.activeTurns = map[string]context.CancelFunc{}
	}
	if _, exists := r.activeTurns[turnID]; exists {
		return fmt.Errorf("turn %q already running", turnID)
	}
	r.activeTurns[turnID] = cancel
	return nil
}

func (r *Runtime) finishActiveTurn(turnID string) {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" || r == nil {
		return
	}
	r.activeTurnsMu.Lock()
	delete(r.activeTurns, turnID)
	r.activeTurnsMu.Unlock()
}

func (r *Runtime) cancelActiveTurn(turnID string) bool {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" || r == nil {
		return false
	}
	r.activeTurnsMu.Lock()
	cancel := r.activeTurns[turnID]
	if cancel != nil {
		delete(r.activeTurns, turnID)
	}
	r.activeTurnsMu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func (r *Runtime) closeActiveTurns() {
	if r == nil {
		return
	}
	r.activeTurnsMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(r.activeTurns))
	for id, cancel := range r.activeTurns {
		delete(r.activeTurns, id)
		if cancel != nil {
			cancels = append(cancels, cancel)
		}
	}
	r.activeTurnsMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}
