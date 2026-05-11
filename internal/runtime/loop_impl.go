package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
)

// 循环检测错误
var (
	// ErrLoopWarning 检测到循环，注入提示
	ErrLoopWarning = errors.New("loop_warning")

	// ErrLoopForceBreak 连续循环超过阈值，强制中断
	ErrLoopForceBreak = errors.New("loop_force_break")
)

const (
	LoopLevelWarning    = "warning"
	LoopLevelForceBreak = "force_break"
)

type LoopCheckResult struct {
	Level             string
	ToolName          string
	Reason            string
	Alternatives      string
	SameSignatureCount int
	PatternLength     int
	WarnCount         int
	MaxWarns          int
	WrapUpRequired    bool
}

func (r *LoopCheckResult) Error() error {
	if r == nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(r.Level)) {
	case LoopLevelForceBreak:
		if r.SameSignatureCount > 0 {
			return fmt.Errorf("%w: 检测到对 %s 的重复调用（%d 次相同参数）。替代方案：%s", ErrLoopForceBreak, r.ToolName, r.SameSignatureCount, r.Alternatives)
		}
		return fmt.Errorf("%w: 连续循环操作已达 %d 次，强制中断。请停止当前方法，直接基于已有信息完成任务或回答用户", ErrLoopForceBreak, r.WarnCount)
	case LoopLevelWarning:
		return fmt.Errorf("%w: 检测到重复操作模式（工具 %s）。建议：%s", ErrLoopWarning, r.ToolName, r.Alternatives)
	default:
		return nil
	}
}

// defaultWindowSize 滑动窗口大小
const defaultWindowSize = 10

// defaultMaxWarns 最大警告次数，超过后强制中断
const defaultMaxWarns = 5

// SlidingWindowLoopDetector 基于滑动窗口的工具调用循环检测器
type SlidingWindowLoopDetector struct {
	mu         sync.Mutex
	window     []string // 最近 N 个工具调用签名
	windowSize int      // 窗口大小
	warnCount  int      // 连续警告计数
	maxWarns   int      // 最大警告次数
}

// NewSlidingWindowLoopDetector 创建滑动窗口循环检测器
func NewSlidingWindowLoopDetector() *SlidingWindowLoopDetector {
	return &SlidingWindowLoopDetector{
		window:     make([]string, 0, defaultWindowSize),
		windowSize: defaultWindowSize,
		maxWarns:   defaultMaxWarns,
	}
}

// CheckLoop 检测工具调用是否形成循环
func (d *SlidingWindowLoopDetector) CheckLoop(toolName string, args map[string]interface{}) error {
	result := d.CheckLoopResult(toolName, args)
	if result == nil {
		return nil
	}
	return result.Error()
}

// CheckLoopResult 检测工具调用是否形成循环并返回结构化结果
func (d *SlidingWindowLoopDetector) CheckLoopResult(toolName string, args map[string]interface{}) *LoopCheckResult {
	sig := makeSignature(toolName, args)

	d.mu.Lock()
	defer d.mu.Unlock()

	// 追加到窗口
	d.window = append(d.window, sig)
	if len(d.window) > d.windowSize {
		d.window = d.window[len(d.window)-d.windowSize:]
	}

	// 窗口不满时不检测
	if len(d.window) < 4 {
		return nil
	}

	sameSigCount := 0
	for _, w := range d.window {
		if w == sig {
			sameSigCount++
		}
	}
	if (toolName == "read" || toolName == "search") && sameSigCount >= 3 {
		d.warnCount++
		slog.Warn("loop_detector.same_call_detected",
			"tool", toolName,
			"sig_count", sameSigCount,
			"warn_count", d.warnCount,
			"max_warns", d.maxWarns)
		alternatives := suggestAlternatives(toolName)
		return &LoopCheckResult{
			Level:              LoopLevelForceBreak,
			ToolName:           toolName,
			Reason:             "same_call_detected",
			Alternatives:       alternatives,
			SameSignatureCount: sameSigCount,
			WarnCount:          d.warnCount,
			MaxWarns:           d.maxWarns,
			WrapUpRequired:     true,
		}
	}

	// 检测重复模式
	if patternLen := d.detectPattern(); patternLen > 0 {
		d.warnCount++
		slog.Warn("loop_detector.pattern_detected",
			"tool", toolName,
			"pattern_length", patternLen,
			"warn_count", d.warnCount,
			"max_warns", d.maxWarns)

		alternatives := suggestAlternatives(toolName)
		if d.warnCount >= d.maxWarns {
			return &LoopCheckResult{
				Level:          LoopLevelForceBreak,
				ToolName:       toolName,
				Reason:         "pattern_warn_limit",
				Alternatives:   alternatives,
				PatternLength:  patternLen,
				WarnCount:      d.warnCount,
				MaxWarns:       d.maxWarns,
				WrapUpRequired: true,
			}
		}
		return &LoopCheckResult{
			Level:          LoopLevelWarning,
			ToolName:       toolName,
			Reason:         "pattern_detected",
			Alternatives:   alternatives,
			PatternLength:  patternLen,
			WarnCount:      d.warnCount,
			MaxWarns:       d.maxWarns,
			WrapUpRequired: false,
		}
	}

	// 没有检测到模式，重置警告计数
	d.warnCount = 0
	return nil
}

// suggestAlternatives 根据工具类型给出具体的替代操作建议
func suggestAlternatives(toolName string) string {
	switch toolName {
	case "read":
		return "1) 用 search {mode:\"text\"} 搜索关键代码片段 2) 用 read {mode:\"directory\"} 查看目录结构 3) 直接基于已读内容完成任务"
	case "search":
		return "1) 更换搜索关键词 2) 扩大搜索范围 3) 改用 search {mode:\"glob\"} 查找文件 4) 直接基于已有结果作答"
	case "edit":
		return "1) 检查目标文件是否正确 2) 重新 read 确认最新内容 3) 调整 old_text 匹配当前文件内容"
	case "bash":
		return "1) 检查命令语法 2) 改用 PowerShell 语法 3) 改用内建工具替代"
	default:
		return "1) 尝试不同的参数 2) 换用其他工具 3) 直接基于已有信息完成任务"
	}
}

// Reset 重置检测器状态
func (d *SlidingWindowLoopDetector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.window = d.window[:0]
	d.warnCount = 0
}

// detectPattern 检测窗口内的重复模式
// 返回模式长度（0 表示未检测到模式）
// 需持有锁
func (d *SlidingWindowLoopDetector) detectPattern() int {
	n := len(d.window)
	if n < 4 {
		return 0
	}

	// 检测长度 1 到 n/2 的重复模式
	for patLen := 1; patLen <= n/2; patLen++ {
		// 至少需要重复 2 次
		if n < patLen*2 {
			continue
		}

		// 从窗口末尾检查最后 patLen*2 个签名是否构成重复模式
		tail := d.window[n-patLen*2:]
		isPattern := true
		for i := 0; i < patLen; i++ {
			if tail[i] != tail[i+patLen] {
				isPattern = false
				break
			}
		}
		if isPattern {
			return patLen
		}
	}

	return 0
}

// makeSignature 生成工具调用签名
func makeSignature(toolName string, args map[string]interface{}) string {
	bs, _ := marshalCanonical(args)
	hash := sha256.Sum256(bs)
	return fmt.Sprintf("%s:%x", toolName, hash[:6])
}

type kv struct {
	K string `json:"k"`
	V any    `json:"v"`
}

func marshalCanonical(v any) ([]byte, error) {
	return json.Marshal(canonicalize(v))
}

func canonicalize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([]kv, 0, len(keys))
		for _, k := range keys {
			out = append(out, kv{K: k, V: canonicalize(x[k])})
		}
		return out
	case []any:
		out := make([]any, 0, len(x))
		for _, it := range x {
			out = append(out, canonicalize(it))
		}
		return out
	default:
		return v
	}
}
