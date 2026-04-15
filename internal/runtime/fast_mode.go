package runtime

import (
	"sync"
)

// FastModeManager manages fast model switching
type FastModeManager struct {
	mu         sync.RWMutex
	fastModel  string
	mainModel  string
	activeFast bool
	onChange   func(fastEnabled bool, model string)
}

// NewFastModeManager creates a new fast mode manager
func NewFastModeManager(mainModel string) *FastModeManager {
	return &FastModeManager{
		mainModel: mainModel,
	}
}

// SetFastModel configures the fast model name
func (fmm *FastModeManager) SetFastModel(model string) {
	fmm.mu.Lock()
	defer fmm.mu.Unlock()
	fmm.fastModel = model
}

// FastModel returns the fast model name
func (fmm *FastModeManager) FastModel() string {
	fmm.mu.RLock()
	defer fmm.mu.RUnlock()
	return fmm.fastModel
}

// SetFastMode enables or disables fast mode
func (fmm *FastModeManager) SetFastMode(enabled bool) {
	fmm.mu.Lock()
	defer fmm.mu.Unlock()

	if fmm.activeFast == enabled {
		return
	}
	fmm.activeFast = enabled

	if fmm.onChange != nil {
		model := fmm.mainModel
		if enabled && fmm.fastModel != "" {
			model = fmm.fastModel
		}
		go fmm.onChange(enabled, model)
	}
}

// IsFastMode returns whether fast mode is currently active
func (fmm *FastModeManager) IsFastMode() bool {
	fmm.mu.RLock()
	defer fmm.mu.RUnlock()
	return fmm.activeFast
}

// ActiveModel returns the currently active model name
func (fmm *FastModeManager) ActiveModel() string {
	fmm.mu.RLock()
	defer fmm.mu.RUnlock()
	if fmm.activeFast && fmm.fastModel != "" {
		return fmm.fastModel
	}
	return fmm.mainModel
}

// SetOnChange sets the callback for mode changes
func (fmm *FastModeManager) SetOnChange(cb func(bool, string)) {
	fmm.mu.Lock()
	defer fmm.mu.Unlock()
	fmm.onChange = cb
}
