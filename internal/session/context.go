package session

import (
	"github.com/dreamSailing/vb-coding/internal/ai"
	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
	"math"
	"strings"
	"sync"
)

// CompressionStrategy 压缩策略类型
type CompressionStrategy int

const (
	// CompressionConservative 保守策略：保留更多历史，压缩更温和
	CompressionConservative CompressionStrategy = iota
	// CompressionBalanced 平衡策略：默认策略
	CompressionBalanced
	// CompressionAggressive 激进策略：大幅压缩历史，保留最少信息
	CompressionAggressive
)

func (s CompressionStrategy) String() string {
	switch s {
	case CompressionConservative:
		return "conservative"
	case CompressionBalanced:
		return "balanced"
	case CompressionAggressive:
		return "aggressive"
	default:
		return "unknown"
	}
}

// CompressionStats 压缩统计信息
type CompressionStats struct {
	TotalCompressions int   // 总压缩次数
	LastCompressedAt  int64 // 最后压缩时间
	Strategy          CompressionStrategy
	OriginalChars     int     // 压缩前字符数
	CompressedChars   int     // 压缩后字符数
	OriginalTokens    int     // 压缩前 token 数
	CompressedTokens  int     // 压缩后 token 数
	SavedRatio        float64 // 节省比例
}

// ContextSnapshot 上下文快照，用于压缩历史保留
type ContextSnapshot struct {
	Timestamp int64            // 快照时间戳
	Messages  []ai.Message     // 快照消息
	Stats     CompressionStats // 压缩统计
}

// ContextManager 上下文管理器
type ContextManager struct {
	mu                  sync.RWMutex
	pinned              []ai.Message
	recent              []ai.Message
	tools               []string
	toolObs             []string
	ephem               []string
	currentFull         []ai.Message
	lastPlan            string
	recentRounds        int
	toolLimit           int
	maxChars            int
	modelName           string
	maxPromptTokens     int
	reservedReplyTokens int
	tokenCache          *utils.TokenEstimateCache
	onPlanUpdate        func(string)
	compressionStrategy CompressionStrategy // 压缩策略
	compressionStats    CompressionStats    // 压缩统计
	snapshots           []ContextSnapshot   // 压缩历史快照
	maxSnapshots        int                 // 最大快照数量
	autoCompressEnabled bool                // 是否启用自动压缩
	compressThreshold   float64             // 自动压缩阈值 (0-1)
	onPreCompact        func(trigger string, customInstructions string)
}

// NewContextManager 创建新的上下文管理器
func NewContextManager() *ContextManager {
	return &ContextManager{
		recentRounds:        6,
		toolLimit:           10,
		maxChars:            64000,
		modelName:           "",
		maxPromptTokens:     16000,
		reservedReplyTokens: 2048,
		tokenCache:          utils.NewTokenEstimateCache(2048),
		compressionStrategy: CompressionBalanced,
		maxSnapshots:        10,
		autoCompressEnabled: true,
		compressThreshold:   0.90, // 90% 阈值（从 95% 降低，提前压缩以避免急剧信息丢失）
	}
}

func (c *ContextManager) SetModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setModelLocked(model)
}

func (c *ContextManager) setModelLocked(model string) {
	m := strings.TrimSpace(model)
	c.modelName = m

	window := ai.ContextWindowTokens(m)
	reserved := c.reservedReplyTokens
	if window > 0 {
		dynamicReserved := int(math.Round(float64(window) * 0.15))
		if dynamicReserved < 1024 {
			dynamicReserved = 1024
		}
		if dynamicReserved > 8192 {
			dynamicReserved = 8192
		}
		reserved = dynamicReserved
		if reserved >= window {
			reserved = window / 5
		}
	}
	if reserved < 512 {
		reserved = 512
	}
	c.reservedReplyTokens = reserved

	maxPrompt := window - reserved
	if maxPrompt < 2048 {
		maxPrompt = 2048
	}
	// 根据模型上下文窗口动态设定上限
	// 128K 模型：最多使用 80K
	// 200K+ 模型：最多使用 120K
	maxLimit := 16000
	if window >= 200000 {
		maxLimit = 120000
	} else if window >= 128000 {
		maxLimit = 80000
	} else if window >= 32000 {
		maxLimit = 24000
	}
	if maxPrompt > maxLimit {
		maxPrompt = maxLimit
	}
	c.maxPromptTokens = maxPrompt
	c.maxChars = maxPrompt * 4
}

// SetOnPlanUpdate 设置计划更新回调
func (c *ContextManager) SetOnPlanUpdate(cb func(string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onPlanUpdate = cb
}

func (c *ContextManager) SetOnPreCompact(cb func(trigger string, customInstructions string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onPreCompact = cb
}

// SetMaxChars 设置上下文最大字符数
func (c *ContextManager) SetMaxChars(maxChars int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxChars = maxChars
	if maxChars > 0 {
		c.maxPromptTokens = maxChars / 4
	}
}

// SetCompressionStrategy 设置压缩策略
func (c *ContextManager) SetCompressionStrategy(strategy CompressionStrategy) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.compressionStrategy = strategy
	c.applyCompressionStrategyLocked()
}

// CompressionStrategy 获取当前压缩策略
func (c *ContextManager) CompressionStrategy() CompressionStrategy {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.compressionStrategy
}

// GetCompressionStats 获取压缩统计信息
func (c *ContextManager) GetCompressionStats() CompressionStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.compressionStats
}

// SetAutoCompressEnabled 设置是否启用自动压缩
func (c *ContextManager) SetAutoCompressEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.autoCompressEnabled = enabled
}

// SetCompressThreshold 设置自动压缩阈值
func (c *ContextManager) SetCompressThreshold(threshold float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if threshold >= 0 && threshold <= 1 {
		c.compressThreshold = threshold
	}
}

// GetAutoCompressEnabled 获取自动压缩状态
func (c *ContextManager) GetAutoCompressEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.autoCompressEnabled
}

// GetCompressThreshold 获取自动压缩阈值
func (c *ContextManager) GetCompressThreshold() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.compressThreshold
}

// applyCompressionStrategyLocked 应用压缩策略（调用前需持有锁）
func (c *ContextManager) applyCompressionStrategyLocked() {
	switch c.compressionStrategy {
	case CompressionConservative:
		c.recentRounds = 10
		c.toolLimit = 15
	case CompressionBalanced:
		c.recentRounds = 6
		c.toolLimit = 10
	case CompressionAggressive:
		c.recentRounds = 3
		c.toolLimit = 5
	}
}
