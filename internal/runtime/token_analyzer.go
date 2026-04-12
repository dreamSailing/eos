package runtime

import (
	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"log/slog"
)

// TokenMetrics Token 使用指标
type TokenMetrics struct {
	InputTokens   int64     // 输入 token 数
	OutputTokens  int64     // 输出 token 数
	TotalTokens   int64     // 总 token 数
	EstimatedCost float64   // 预估成本（美元）
	Timestamp     time.Time // 记录时间
	Component     string    // 组件名称（user, tool, assistant, system）
	Stage         string    // 阶段（prompt, tool_call, response）
}

// TokenAnalysis Token 使用分析结果
type TokenAnalysis struct {
	SessionID          string                    // 会话 ID
	StartTime          time.Time                 // 会话开始时间
	EndTime            time.Time                 // 会话结束时间
	TotalMessages      int                       // 总消息数
	TotalInput         int64                     // 总输入 token
	TotalOutput        int64                     // 总输出 token
	TotalTokens        int64                     // 总 token
	EstimatedCost      float64                   // 预估成本
	MetricsByStage     map[string][]TokenMetrics // 按阶段分类的指标
	MetricsByComponent map[string][]TokenMetrics // 按组件分类的指标
	ToolUsage          map[string]int64          // 工具调用 token 消耗
}

// TokenAnalyzer Token 使用分析器
type TokenAnalyzer struct {
	mu           sync.RWMutex
	sessionID    string
	startTime    time.Time
	metrics      []TokenMetrics
	toolUsage    map[string]int64
	roundMetrics map[int][]TokenMetrics // 按轮次分类
	currentRound int
	enabled      bool
}

// NewTokenAnalyzer 创建 Token 分析器
func NewTokenAnalyzer(sessionID string) *TokenAnalyzer {
	return &TokenAnalyzer{
		sessionID:    sessionID,
		startTime:    time.Now(),
		metrics:      make([]TokenMetrics, 0, 1000),
		toolUsage:    make(map[string]int64),
		roundMetrics: make(map[int][]TokenMetrics),
		currentRound: 0,
		enabled:      true,
	}
}

// Record 记录 Token 使用
func (a *TokenAnalyzer) Record(metrics TokenMetrics) {
	if !a.enabled {
		return
	}
	if metrics.Timestamp.IsZero() {
		metrics.Timestamp = time.Now()
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.metrics = append(a.metrics, metrics)
	a.roundMetrics[a.currentRound] = append(a.roundMetrics[a.currentRound], metrics)

	if metrics.Component == "tool" {
		a.toolUsage[metrics.Stage] += metrics.TotalTokens
	}

	slog.Debug("token_analyzer.record",
		"component", utils.ComponentSystem,
		"session", a.sessionID,
		"round", a.currentRound,
		"input", metrics.InputTokens,
		"output", metrics.OutputTokens,
		"stage", metrics.Stage,
	)
}

// NextRound 进入下一轮对话
func (a *TokenAnalyzer) NextRound() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.currentRound++
	slog.Debug("token_analyzer.next_round",
		"component", utils.ComponentSystem,
		"session", a.sessionID,
		"round", a.currentRound,
	)
}

// Analyze 执行 Token 使用分析
func (a *TokenAnalyzer) Analyze() *TokenAnalysis {
	a.mu.RLock()
	defer a.mu.RUnlock()

	analysis := &TokenAnalysis{
		SessionID:          a.sessionID,
		StartTime:          a.startTime,
		EndTime:            time.Now(),
		TotalMessages:      len(a.metrics),
		MetricsByStage:     make(map[string][]TokenMetrics),
		MetricsByComponent: make(map[string][]TokenMetrics),
		ToolUsage:          make(map[string]int64),
	}

	// 复制工具使用数据
	for k, v := range a.toolUsage {
		analysis.ToolUsage[k] = v
	}

	// 按阶段和组件分类
	for _, m := range a.metrics {
		analysis.TotalInput += m.InputTokens
		analysis.TotalOutput += m.OutputTokens
		analysis.TotalTokens += m.TotalTokens
		analysis.EstimatedCost += m.EstimatedCost

		analysis.MetricsByStage[m.Stage] = append(analysis.MetricsByStage[m.Stage], m)
		analysis.MetricsByComponent[m.Component] = append(analysis.MetricsByComponent[m.Component], m)
	}

	return analysis
}

// AnalyzeRound 分析指定轮次的 Token 使用
func (a *TokenAnalyzer) AnalyzeRound(round int) *RoundTokenAnalysis {
	a.mu.RLock()
	defer a.mu.RUnlock()

	metrics, ok := a.roundMetrics[round]
	if !ok {
		return nil
	}

	analysis := &RoundTokenAnalysis{
		Round:        round,
		InputTokens:  0,
		OutputTokens: 0,
		TotalTokens:  0,
		ByStage:      make(map[string]int64),
	}

	for _, m := range metrics {
		analysis.InputTokens += m.InputTokens
		analysis.OutputTokens += m.OutputTokens
		analysis.TotalTokens += m.TotalTokens
		analysis.EstimatedCost += m.EstimatedCost
		analysis.ByStage[m.Stage] += m.TotalTokens
	}

	return analysis
}

// RoundTokenAnalysis 单轮 Token 分析结果
type RoundTokenAnalysis struct {
	Round         int              // 轮次
	InputTokens   int64            // 输入 token
	OutputTokens  int64            // 输出 token
	TotalTokens   int64            // 总 token
	EstimatedCost float64          // 预估成本
	ByStage       map[string]int64 // 按阶段统计
}

// EstimateTokens 估算文本的 token 数量
// 粗略估算：中文约 1 字符 = 1 token，英文约 4 字符 = 1 token
func EstimateTokens(text string) int64 {
	if strings.TrimSpace(text) == "" {
		return 0
	}
	return int64(utils.EstimateTokensWeighted("", text))
}

// EstimateMessageTokens 估算消息的 token 数量
func EstimateMessageTokens(msg *schema.Message) (input, output int64) {
	if msg == nil {
		return 0, 0
	}

	switch msg.Role {
	case schema.System, schema.User:
		input = EstimateTokens(msg.Content)
	case schema.Assistant:
		input = EstimateTokens(msg.Content)
		output = EstimateTokens(msg.Content)
	}

	// 估算工具调用的 token 消耗
	if len(msg.ToolCalls) > 0 {
		// 每个工具调用约 50 tokens（参数 + 名称）
		input += int64(len(msg.ToolCalls) * 50)
	}

	return input, output
}

func EstimateCost(modelName string, inputTokens, outputTokens int64) float64 {
	inputPricePerMillion := 3.0
	outputPricePerMillion := 15.0

	switch {
	case strings.HasPrefix(strings.ToLower(strings.TrimSpace(modelName)), "qwen3.6-plus"),
		strings.HasPrefix(strings.ToLower(strings.TrimSpace(modelName)), "qwen3.5-plus"):
		inputPricePerMillion = 800.0
		outputPricePerMillion = 4800.0
	}

	inputCost := float64(inputTokens) / 1_000_000 * inputPricePerMillion
	outputCost := float64(outputTokens) / 1_000_000 * outputPricePerMillion

	return inputCost + outputCost
}

// GetTopTokenConsumers 获取 token 消耗最多的阶段/工具
func (a *TokenAnalyzer) GetTopTokenConsumers(n int) []TokenConsumer {
	a.mu.RLock()
	defer a.mu.RUnlock()

	consumers := make(map[string]int64)

	// 聚合各阶段/工具的 token 消耗
	for _, m := range a.metrics {
		key := m.Stage
		if m.Component == "tool" {
			key = "tool:" + m.Stage
		}
		consumers[key] += m.TotalTokens
	}

	// 转换为切片并排序
	var result []TokenConsumer
	for k, v := range consumers {
		result = append(result, TokenConsumer{
			Name:   k,
			Tokens: v,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Tokens > result[j].Tokens
	})

	if n > 0 && len(result) > n {
		result = result[:n]
	}

	return result
}

// TokenConsumer Token 消费者
type TokenConsumer struct {
	Name   string // 阶段/工具名称
	Tokens int64  // token 消耗量
}

// GetSummary 获取摘要统计
func (a *TokenAnalyzer) GetSummary() map[string]any {
	a.mu.RLock()
	defer a.mu.RUnlock()

	analysis := a.Analyze()
	topConsumers := a.GetTopTokenConsumers(5)

	return map[string]any{
		"session_id":       a.sessionID,
		"duration_seconds": time.Since(a.startTime).Seconds(),
		"current_round":    a.currentRound,
		"total_messages":   analysis.TotalMessages,
		"total_input":      analysis.TotalInput,
		"total_output":     analysis.TotalOutput,
		"total_tokens":     analysis.TotalTokens,
		"estimated_cost":   analysis.EstimatedCost,
		"avg_per_round": func() int64 {
			if a.currentRound > 0 {
				return analysis.TotalTokens / int64(a.currentRound)
			}
			return 0
		}(),
		"top_consumers": topConsumers,
	}
}

// Clear 清除所有记录
func (a *TokenAnalyzer) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.metrics = make([]TokenMetrics, 0, 1000)
	a.toolUsage = make(map[string]int64)
	a.roundMetrics = make(map[int][]TokenMetrics)
	a.startTime = time.Now()
	a.currentRound = 0

	slog.Debug("token_analyzer.clear",
		"component", utils.ComponentSystem,
		"session", a.sessionID,
	)
}

// Enable 启用分析
func (a *TokenAnalyzer) Enable() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = true
}

// Disable 禁用分析
func (a *TokenAnalyzer) Disable() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = false
}

// IsEnabled 检查分析是否启用
func (a *TokenAnalyzer) IsEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.enabled
}
