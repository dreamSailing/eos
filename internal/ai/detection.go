package ai

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"strings"

	"github.com/eosaios/eos/internal/config"
)

// DetectThinkingCapability 尝试从模型名称检测思考能力
// 作为字典的后备方案，当模型不在字典中时使用
func DetectThinkingCapability(modelName string) ThinkingCapability {
	name := strings.ToLower(strings.TrimSpace(modelName))

	// OpenAI o1 系列
	if strings.Contains(name, "o1") {
		if strings.Contains(name, "mini") {
			return ThinkingMedium
		}
		if strings.Contains(name, "preview") {
			return ThinkingMedium
		}
		return ThinkingHigh
	}

	// DeepSeek Reasoning 系列
	if strings.Contains(name, "deepseek") && (strings.Contains(name, "r1") || strings.Contains(name, "reasoning") || strings.Contains(name, "reasoner")) {
		return ThinkingHigh
	}

	// Kimi 系列
	if strings.Contains(name, "kimi") && (strings.Contains(name, "k2.5") || strings.Contains(name, "k2-5") || strings.Contains(name, "thinking")) {
		return ThinkingMedium
	}

	// GLM 系列
	if strings.Contains(name, "glm") && (strings.Contains(name, "4.7") || strings.Contains(name, "4.6") || strings.Contains(name, "thinking")) {
		return ThinkingMedium
	}

	// Qwen Reasoning 系列
	if strings.Contains(name, "qwen") && strings.Contains(name, "thinking") {
		return ThinkingMedium
	}
	if strings.Contains(name, "qwen") && (strings.Contains(name, "reasoning") || strings.Contains(name, "qwq")) {
		return ThinkingHigh
	}

	// Doubao / Ark 系列
	if strings.Contains(name, "doubao") || strings.Contains(name, "ark") {
		if strings.Contains(name, "thinking") || strings.Contains(name, "seed") {
			return ThinkingMedium
		}
		if strings.Contains(name, "code") {
			return ThinkingLow
		}
	}

	// 未来可以添加更多提供商的模式
	// 例如: Gemini Think, Meta COT 等

	return ThinkingNone
}

// ShouldEnableThinkingForModel 决定是否为指定模型启用思考模式
// 优先级: 模型配置 > 全局配置 > 模型目录 > 动态检测
func ShouldEnableThinkingForModel(modelName string, cfg *config.Config) bool {
	if cfg == nil {
		// 无配置时，仅检查模型目录
		return SupportsThinking(modelName)
	}

	// 检查全局思考开关
	if !cfg.Thinking.Enabled {
		return false
	}

	// 1. 检查模型级别的思考设置（优先级最高）
	for _, entry := range cfg.Models {
		if strings.EqualFold(entry.Name, modelName) {
			// 如果模型明确设置了 ThinkingEnabled，直接返回
			return entry.ThinkingEnabled
		}
	}

	// 2. 检查自定义模型列表
	for _, custom := range cfg.Thinking.CustomModels {
		if strings.EqualFold(custom, modelName) {
			return true
		}
	}

	// 3. 检查模型目录（Rust 内核推送）
	if info, ok := GetBuiltinModelInfo(modelName); ok {
		return info.Thinking > ThinkingNone
	}

	// 4. 动态检测（catalog 未覆盖的自定义模型，按模型名启发式推断）
	if cfg.Thinking.AutoDetect {
		return DetectThinkingCapability(modelName) > ThinkingNone
	}

	return false
}

// GetReasoningEffortLevel 获取推理级别配置
// 返回: "low", "medium", "high" 或 ""（不支持）
func GetReasoningEffortLevel(modelName string, cfg *config.Config) string {
	if cfg == nil || !cfg.Thinking.Enabled {
		return ""
	}

	// 如果模型不支持 ReasoningEffort，返回空
	if !SupportsReasoningEffort(modelName) {
		return ""
	}

	// 使用配置的级别
	level := strings.ToLower(strings.TrimSpace(cfg.Thinking.ReasoningEffort))
	switch level {
	case "low", "medium", "high":
		return level
	default:
		// 默认使用 medium
		return "medium"
	}
}

// GetModelCapabilitySummary 获取模型能力的完整摘要
// 用于调试和日志记录
func GetModelCapabilitySummary(modelName string, cfg *config.Config) map[string]interface{} {
	info, inCatalog := GetBuiltinModelInfo(modelName)
	detected := DetectThinkingCapability(modelName)
	enabled := ShouldEnableThinkingForModel(modelName, cfg)
	reasoningLevel := GetReasoningEffortLevel(modelName, cfg)

	return map[string]interface{}{
		"model":                     modelName,
		"in_catalog":                inCatalog,
		"catalog_capability":        info.Thinking.String(),
		"detected_capability":       detected.String(),
		"thinking_enabled":          enabled,
		"supports_reasoning_effort": info.SupportsReasoningEffort,
		"reasoning_effort_level":    reasoningLevel,
	}
}
