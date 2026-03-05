package tools

import (
	"github.com/dreamSailing/vb-coding/internal/mcp"
	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
)

// GetToolTraces returns tool call trace records
func (m *Manager) GetToolTraces() []ToolCallTrace {
	if m.executor != nil {
		return m.executor.GetTraces()
	}
	return nil
}

// GetToolStats returns tool call statistics
func (m *Manager) GetToolStats() map[string]*ToolCallStats {
	if m.executor != nil {
		return m.executor.GetStats()
	}
	return nil
}

// ClearToolCache clears tool call cache
func (m *Manager) ClearToolCache() {
	if m.executor != nil {
		m.executor.ClearCache()
	}
}

// ClearToolTraces clears tool call trace records
func (m *Manager) ClearToolTraces() {
	if m.executor != nil {
		m.executor.ClearTraces()
	}
}

// SetRetryPolicy sets tool call retry policy
func (m *Manager) SetRetryPolicy(policy *utils.RetryPolicy) {
	if m.executor != nil {
		m.executor.SetRetryPolicy(policy)
	}
}

// SetSkillManager sets the Skills manager
func (m *Manager) SetSkillManager(sm *SkillManager) {
	m.skillManager = sm
}

// GetSkillManager returns the Skills manager
func (m *Manager) GetSkillManager() *SkillManager {
	return m.skillManager
}

func (m *Manager) SetMCPManager(mm *mcp.Manager) {
	m.mcpManager = mm
}
