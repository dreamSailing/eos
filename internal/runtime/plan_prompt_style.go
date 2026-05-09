package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"strings"
)

const defaultPlanPromptStyle = "concise"

type planPromptStyleContextKey struct{}

// NormalizePlanPromptStyle returns the persisted representation for planner style.
func NormalizePlanPromptStyle(raw string) string {
	style := strings.TrimSpace(raw)
	if style == "" {
		return defaultPlanPromptStyle
	}
	lower := strings.ToLower(style)
	switch lower {
	case "concise", "detailed":
		return lower
	}
	if strings.HasPrefix(lower, "custom:") {
		body := strings.TrimSpace(style[len("custom:"):])
		if body == "" {
			return defaultPlanPromptStyle
		}
		return "custom:" + body
	}
	return "custom:" + style
}

func WithPlanPromptStyle(ctx context.Context, style string) context.Context {
	return context.WithValue(ctx, planPromptStyleContextKey{}, NormalizePlanPromptStyle(style))
}

func planPromptStyleFromContext(ctx context.Context) string {
	if ctx == nil {
		return defaultPlanPromptStyle
	}
	if value, ok := ctx.Value(planPromptStyleContextKey{}).(string); ok {
		return NormalizePlanPromptStyle(value)
	}
	return defaultPlanPromptStyle
}

func BuildPlanPromptForStyle(style string) string {
	normalized := NormalizePlanPromptStyle(style)
	switch normalized {
	case "detailed":
		return PlanPrompt + `

**计划提示风格：详细**
- 现状分析需要覆盖关键依赖、调用链、数据流和潜在影响面。
- 实施计划需要说明阶段顺序、风险点、回退思路和边界条件。
- 验证方案需要包含单元测试、集成/手动验证入口，以及失败时的排查信号。`
	case "concise":
		return PlanPrompt
	default:
		if strings.HasPrefix(normalized, "custom:") {
			custom := strings.TrimSpace(strings.TrimPrefix(normalized, "custom:"))
			if custom != "" {
				return PlanPrompt + `

**计划提示风格：自定义**
` + custom
			}
		}
		return PlanPrompt
	}
}
