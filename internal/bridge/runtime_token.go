//go:build legacy

package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/dreamSailing/eos/internal/ai"
)

// AddTokenRecord 添加 Token 记录
func (rc *RuntimeCore) AddTokenRecord(input, reply, total int) {
	rc.AddTokenRecordWithModel(&schema.TokenUsage{
		PromptTokens:     input,
		CompletionTokens: reply,
		TotalTokens:      total,
	}, "")
}

// AddTokenRecordWithModel 添加带模型名称的 Token 记录
func (rc *RuntimeCore) AddTokenRecordWithModel(usage *schema.TokenUsage, model string) {
	if rc.cm != nil && usage != nil {
		rc.cm.AddTokenRecord(usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
	}
	rc.tokenMu.Lock()
	defer rc.tokenMu.Unlock()

	if model == "" {
		model = rc.modelName
	}

	var input, reply, total *int
	var cachedInput *int
	var costUSD *float64
	if usage != nil {
		input = intPtr(usage.PromptTokens)
		reply = intPtr(usage.CompletionTokens)
		total = intPtr(usage.TotalTokens)
		if usage.PromptTokenDetails.CachedTokens > 0 {
			cachedInput = intPtr(usage.PromptTokenDetails.CachedTokens)
		}
		if costEst, ok := ai.EstimateUsageCost(model, usage); ok {
			costUSD = floatPtr(costEst.TotalCost)
		}
	}

	rc.tokenHistory = append(rc.tokenHistory, TokenRecord{
		Timestamp:   time.Now(),
		Model:       model,
		Input:       input,
		Reply:       reply,
		Total:       total,
		CachedInput: cachedInput,
		CostUSD:     costUSD,
	})
}

// GetTokenStats 获取 Token 统计
func (rc *RuntimeCore) GetTokenStats() TokenStats {
	rc.tokenMu.RLock()
	defer rc.tokenMu.RUnlock()
	var stats TokenStats
	stats.Rounds = len(rc.tokenHistory)
	for _, r := range rc.tokenHistory {
		stats.Input = addOptionalInt(stats.Input, r.Input)
		stats.Reply = addOptionalInt(stats.Reply, r.Reply)
		stats.Total = addOptionalInt(stats.Total, r.Total)
		stats.CachedInput = addOptionalInt(stats.CachedInput, r.CachedInput)
		stats.TotalCostUSD = addOptionalFloat(stats.TotalCostUSD, r.CostUSD)
		if r.Total == nil {
			stats.UnknownUsageRounds++
		}
		if r.CostUSD == nil {
			stats.UnknownCostRounds++
		}
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
		stats.Input = addOptionalInt(stats.Input, r.Input)
		stats.Reply = addOptionalInt(stats.Reply, r.Reply)
		stats.Total = addOptionalInt(stats.Total, r.Total)
		stats.CachedInput = addOptionalInt(stats.CachedInput, r.CachedInput)
		stats.TotalCostUSD = addOptionalFloat(stats.TotalCostUSD, r.CostUSD)
		if r.Total == nil {
			stats.UnknownUsageRounds++
		}
		if r.CostUSD == nil {
			stats.UnknownCostRounds++
		}
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

func intPtr(v int) *int {
	return &v
}

func floatPtr(v float64) *float64 {
	return &v
}

func addOptionalInt(total, value *int) *int {
	if value == nil {
		return total
	}
	if total == nil {
		return intPtr(*value)
	}
	next := *total + *value
	return intPtr(next)
}

func addOptionalFloat(total, value *float64) *float64 {
	if value == nil {
		return total
	}
	if total == nil {
		return floatPtr(*value)
	}
	next := *total + *value
	return floatPtr(next)
}

func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
