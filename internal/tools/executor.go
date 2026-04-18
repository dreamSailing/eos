package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dreamSailing/eos/internal/pkg/utils"
)

// ToolCallKey 工具调用的缓存键
type ToolCallKey struct {
	Tool       string
	ParamsHash string
}

// ToolCallCacheEntry 工具调用缓存条目
type ToolCallCacheEntry struct {
	Result      ToolResult
	CreatedAt   time.Time
	AccessCount int
	TTL         time.Duration
}

// ToolCallTrace 工具调用追踪记录
type ToolCallTrace struct {
	ID         string                 // 唯一标识
	Tool       string                 // 工具名称
	Params     map[string]interface{} // 参数
	Result     ToolResult             // 执行结果
	StartTime  time.Time              // 开始时间
	EndTime    time.Time              // 结束时间
	Duration   time.Duration          // 执行时长
	Success    bool                   // 是否成功
	Cached     bool                   // 是否来自缓存
	RetryCount int                    // 重试次数
	ParentID   string                 // 父调用 ID（用于调用链）
}

// ToolCallStats 工具调用统计
type ToolCallStats struct {
	TotalCalls    int
	SuccessCalls  int
	FailureCalls  int
	CachedCalls   int
	RetriedCalls  int
	TotalDuration time.Duration
	AvgDuration   time.Duration
}

// ToolCallCache 工具调用缓存管理器
type ToolCallCache struct {
	mu         sync.RWMutex
	cache      map[ToolCallKey]*ToolCallCacheEntry
	maxSize    int
	defaultTTL time.Duration
}

// NewToolCallCache 创建工具调用缓存
func NewToolCallCache(maxSize int, defaultTTL time.Duration) *ToolCallCache {
	return &ToolCallCache{
		cache:      make(map[ToolCallKey]*ToolCallCacheEntry),
		maxSize:    maxSize,
		defaultTTL: defaultTTL,
	}
}

// Get 获取缓存结果
func (c *ToolCallCache) Get(tool string, params map[string]interface{}) (ToolResult, bool) {
	key := c.makeKey(tool, params)
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.cache[key]
	if !ok {
		return ToolResult{}, false
	}

	// 检查是否过期
	if time.Since(entry.CreatedAt) > entry.TTL {
		return ToolResult{}, false
	}

	entry.AccessCount++
	slog.Debug("tools.cache.hit", "component", utils.ComponentTool, "tool", tool, "access_count", entry.AccessCount)
	return entry.Result, true
}

// Set 设置缓存结果
func (c *ToolCallCache) Set(tool string, params map[string]interface{}, result ToolResult) {
	key := c.makeKey(tool, params)

	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果缓存已满，删除最旧的条目
	if len(c.cache) >= c.maxSize {
		var oldestKey ToolCallKey
		var oldestTime time.Time
		for k, v := range c.cache {
			if oldestTime.IsZero() || v.CreatedAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.CreatedAt
			}
		}
		if oldestKey.Tool != "" {
			delete(c.cache, oldestKey)
		}
	}

	c.cache[key] = &ToolCallCacheEntry{
		Result:    result,
		CreatedAt: time.Now(),
		TTL:       c.defaultTTL,
	}

	slog.Debug("tools.cache.set", "component", utils.ComponentTool, "tool", tool, "cache_size", len(c.cache))
}

// Clear 清除缓存
func (c *ToolCallCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[ToolCallKey]*ToolCallCacheEntry)
}

// makeKey 生成缓存键
func (c *ToolCallCache) makeKey(tool string, params map[string]interface{}) ToolCallKey {
	paramsHash := c.hashParams(params)
	return ToolCallKey{
		Tool:       tool,
		ParamsHash: paramsHash,
	}
}

// hashParams 对参数进行哈希
func (c *ToolCallCache) hashParams(params map[string]interface{}) string {
	// 使用 JSON 序列化参数，然后哈希
	// 排序键以确保一致性
	data, err := json.Marshal(params)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// ToolCallTracer 工具调用追踪器
type ToolCallTracer struct {
	mu     sync.RWMutex
	traces []ToolCallTrace
	stats  map[string]*ToolCallStats // 按工具名称统计
	maxLen int
}

// NewToolCallTracer 创建工具调用追踪器
func NewToolCallTracer(maxLen int) *ToolCallTracer {
	return &ToolCallTracer{
		traces: make([]ToolCallTrace, 0, maxLen),
		stats:  make(map[string]*ToolCallStats),
		maxLen: maxLen,
	}
}

// Record 记录工具调用
func (t *ToolCallTracer) Record(trace ToolCallTrace) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 分配 ID
	if trace.ID == "" {
		trace.ID = fmt.Sprintf("trace_%d", len(t.traces)+1)
	}

	// 限制追踪记录数量
	if len(t.traces) >= t.maxLen {
		// 移除最旧的记录
		t.traces = t.traces[1:]
	}

	t.traces = append(t.traces, trace)

	// 更新统计
	stats := t.getStatsLocked(trace.Tool)
	stats.TotalCalls++
	if trace.Success {
		stats.SuccessCalls++
	} else {
		stats.FailureCalls++
	}
	if trace.Cached {
		stats.CachedCalls++
	}
	if trace.RetryCount > 0 {
		stats.RetriedCalls++
	}
	stats.TotalDuration += trace.Duration
	if stats.TotalCalls > 0 {
		stats.AvgDuration = stats.TotalDuration / time.Duration(stats.TotalCalls)
	}

	slog.Debug("tools.trace.record", "component", utils.ComponentTool,
		"id", trace.ID,
		"tool", trace.Tool,
		"success", trace.Success,
		"cached", trace.Cached,
		"duration", trace.Duration.String(),
	)
}

// GetTraces 获取所有追踪记录
func (t *ToolCallTracer) GetTraces() []ToolCallTrace {
	t.mu.RLock()
	defer t.mu.RUnlock()

	traces := make([]ToolCallTrace, len(t.traces))
	copy(traces, t.traces)
	return traces
}

// GetTracesByTool 获取指定工具的追踪记录
func (t *ToolCallTracer) GetTracesByTool(tool string) []ToolCallTrace {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var traces []ToolCallTrace
	for _, trace := range t.traces {
		if trace.Tool == tool {
			traces = append(traces, trace)
		}
	}
	return traces
}

// GetStats 获取统计信息
func (t *ToolCallTracer) GetStats() map[string]*ToolCallStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// 返回副本
	stats := make(map[string]*ToolCallStats, len(t.stats))
	for k, v := range t.stats {
		stats[k] = &ToolCallStats{
			TotalCalls:    v.TotalCalls,
			SuccessCalls:  v.SuccessCalls,
			FailureCalls:  v.FailureCalls,
			CachedCalls:   v.CachedCalls,
			RetriedCalls:  v.RetriedCalls,
			TotalDuration: v.TotalDuration,
			AvgDuration:   v.AvgDuration,
		}
	}
	return stats
}

// Clear 清除追踪记录
func (t *ToolCallTracer) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.traces = make([]ToolCallTrace, 0, t.maxLen)
	t.stats = make(map[string]*ToolCallStats)
}

// getStatsLocked 获取统计信息（调用前需持有锁）
func (t *ToolCallTracer) getStatsLocked(tool string) *ToolCallStats {
	if t.stats[tool] == nil {
		t.stats[tool] = &ToolCallStats{}
	}
	return t.stats[tool]
}

// ToolCallExecutor 工具调用执行器，支持缓存、追踪和重试
type ToolCallExecutor struct {
	cache       *ToolCallCache
	tracer      *ToolCallTracer
	retryPolicy *utils.RetryPolicy
}

// NewToolCallExecutor 创建工具调用执行器
func NewToolCallExecutor() *ToolCallExecutor {
	return &ToolCallExecutor{
		cache:       NewToolCallCache(100, 5*time.Minute),
		tracer:      NewToolCallTracer(1000),
		retryPolicy: &utils.DefaultRetryPolicy,
	}
}

// Execute 执行工具调用（带缓存、追踪、重试）
func (e *ToolCallExecutor) Execute(
	ctx context.Context,
	tool string,
	params map[string]interface{},
	execFunc func(context.Context, map[string]interface{}) ToolResult,
	parentID string,
) ToolResult {
	startTime := time.Now()
	var result ToolResult
	var cached bool

	// 检查缓存
	cachedResult, hit := e.cache.Get(tool, params)
	if hit {
		cached = true
		result = cachedResult
		slog.Debug("tools.executor.cache_hit", "component", utils.ComponentTool, "tool", tool)
	} else {
		// 带重试的执行
		op := func() error {
			result = execFunc(ctx, params)
			if result.Status == "success" {
				return nil
			}
			return fmt.Errorf("%s", result.Error)
		}

		err := utils.DoRetry(ctx, op, *e.retryPolicy)
		if err != nil && result.Status != "success" {
			// 如果最终还是失败，result 已经保存了最后一次执行的结果
			slog.Warn("tools.executor.execute_failed",
				"component", utils.ComponentTool,
				"tool", tool,
				"error", err.Error(),
			)
		}

		// 成功则缓存结果
		if result.Status == "success" {
			e.cache.Set(tool, params, result)
		}
	}

	// 记录追踪
	trace := ToolCallTrace{
		Tool:       tool,
		Params:     params,
		Result:     result,
		StartTime:  startTime,
		EndTime:    time.Now(),
		Duration:   time.Since(startTime),
		Success:    result.Status == "success",
		Cached:     cached,
		RetryCount: 0, // 当前通过 utils.DoRetry 实现，此处追踪暂不记录详细重试次数
		ParentID:   parentID,
	}
	e.tracer.Record(trace)

	return result
}

// GetTraces 获取追踪记录
func (e *ToolCallExecutor) GetTraces() []ToolCallTrace {
	return e.tracer.GetTraces()
}

// GetStats 获取统计信息
func (e *ToolCallExecutor) GetStats() map[string]*ToolCallStats {
	return e.tracer.GetStats()
}

// ClearCache 清除缓存
func (e *ToolCallExecutor) ClearCache() {
	e.cache.Clear()
}

// ClearTraces 清除追踪记录
func (e *ToolCallExecutor) ClearTraces() {
	e.tracer.Clear()
}

// SetRetryPolicy 设置重试策略
func (e *ToolCallExecutor) SetRetryPolicy(policy *utils.RetryPolicy) {
	e.retryPolicy = policy
}
