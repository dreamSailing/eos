package bridge

import (
	"time"
)

// AddTokenRecord 添加 Token 记录
func (rc *RuntimeCore) AddTokenRecord(input, reply, total int) {
	rc.AddTokenRecordWithModel(input, reply, total, "")
}

// AddTokenRecordWithModel 添加带模型名称的 Token 记录
func (rc *RuntimeCore) AddTokenRecordWithModel(input, reply, total int, model string) {
	rc.cm.AddTokenRecord(input, reply, total)
	rc.tokenMu.Lock()
	defer rc.tokenMu.Unlock()

	if model == "" {
		model = rc.modelName
	}

	rc.tokenHistory = append(rc.tokenHistory, TokenRecord{
		Timestamp: time.Now(),
		Model:     model,
		Input:     input,
		Reply:     reply,
		Total:     total,
	})
}

// GetTokenStats 获取 Token 统计
func (rc *RuntimeCore) GetTokenStats() TokenStats {
	rc.tokenMu.RLock()
	defer rc.tokenMu.RUnlock()
	var stats TokenStats
	stats.Rounds = len(rc.tokenHistory)
	for _, r := range rc.tokenHistory {
		stats.Input += r.Input
		stats.Reply += r.Reply
		stats.Total += r.Total
	}
	return stats
}

// GetTokenHistory 获取 Token 历史
func (rc *RuntimeCore) GetTokenHistory() []TokenRecord {
	rc.tokenMu.RLock()
	defer rc.tokenMu.RUnlock()
	out := make([]TokenRecord, len(rc.tokenHistory))
	copy(out, rc.tokenHistory)
	return out
}

// GetModelTokenStats 获取模型 Token 统计
func (rc *RuntimeCore) GetModelTokenStats() []ModelTokenStats {
	rc.tokenMu.RLock()
	defer rc.tokenMu.RUnlock()

	modelMap := make(map[string]*ModelTokenStats)
	for _, r := range rc.tokenHistory {
		model := r.Model
		if model == "" {
			model = "unknown"
		}
		if _, exists := modelMap[model]; !exists {
			modelMap[model] = &ModelTokenStats{
				Model: model,
			}
		}
		stats := modelMap[model]
		stats.Rounds++
		stats.Input += r.Input
		stats.Reply += r.Reply
		stats.Total += r.Total
	}

	result := make([]ModelTokenStats, 0, len(modelMap))
	for _, stats := range modelMap {
		result = append(result, *stats)
	}

	return result
}

// ClearTokenHistory 清除 Token 历史
func (rc *RuntimeCore) ClearTokenHistory() {
	rc.tokenMu.Lock()
	defer rc.tokenMu.Unlock()
	rc.tokenHistory = nil
}

// EstimateTokens 估算 Token 数
func (rc *RuntimeCore) EstimateTokens(text string) (int, int, int) {
	return rc.cm.EstimateTokens(text)
}
