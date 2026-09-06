package ai

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"strings"

	"github.com/eosaios/eos/pkg/coreapi"
)

// ProviderType 服务商类型
type ProviderType string

const (
	ProviderDeepSeek  ProviderType = "deepseek"  // DeepSeek
	ProviderDashScope ProviderType = "dashscope" // 阿里云通义
	ProviderByteDance ProviderType = "bytedance" // 字节豆包
	ProviderZhipu     ProviderType = "zhipu"     // 智谱 GLM
	ProviderMoonshot  ProviderType = "moonshot"  // Moonshot Kimi
	ProviderMiniMax   ProviderType = "minimax"   // MiniMax
	ProviderMiMo      ProviderType = "mimo"      // Xiaomi MiMo
	ProviderGemini    ProviderType = "gemini"    // Google Gemini
	ProviderOpenAI    ProviderType = "openai"    // OpenAI
	ProviderAnthropic ProviderType = "anthropic" // Anthropic Claude
	ProviderCustom    ProviderType = "custom"    // 自定义
)

// APIType API 类型
type APIType string

const (
	APITypeStandard        APIType = "standard"          // 标准 API
	APITypeCodePlan        APIType = "code-plan"         // Code/Plan 套餐 API (OpenAI 兼容)
	APITypeClaude          APIType = "claude"            // Claude 兼容 API (Anthropic 协议)
	APITypeTokenPlan       APIType = "token-plan"        // Token Plan 套餐 API (OpenAI 兼容)
	APITypeTokenPlanClaude APIType = "token-plan-claude" // Token Plan Claude 兼容 API
)

// ProviderConfig 服务商配置
type ProviderConfig struct {
	ID                     string                      // 唯一标识
	Name                   string                      // 显示名称
	Type                   ProviderType                // 服务商类型
	DefaultAPIBase         string                      // 默认 Base URL（标准 API）
	CodePlanAPIBase        string                      // Code/Plan API Base URL (OpenAI 兼容)
	ClaudeAPIBase          string                      // Claude 兼容 API Base URL (Anthropic 协议)
	AgentPlanAPIBase       string                      // Agent Plan API Base URL (OpenAI 兼容)
	AgentPlanClaudeAPIBase string                      // Agent Plan Claude 兼容 API Base URL
	APIKeyEnv              string                      // API Key 环境变量名
	Website                string                      // 官方网站
	HasCodePlan            bool                        // 是否支持 Code/Plan 套餐
	HasClaudeCode          bool                        // 是否支持 Claude 兼容 API 模式
	HasAgentPlan           bool                        // 是否支持 Agent Plan
	HasTokenPlan           bool                        // 是否支持 Token Plan
	TokenPlanAPIBase       string                      // Token Plan API Base URL (OpenAI 兼容)
	TokenPlanClaudeAPIBase string                      // Token Plan Claude 兼容 API Base URL
	DefaultModels          []string                    // 默认/推荐模型列表
	Endpoints              []coreapi.ProviderEndpoint // 内核原始端点向量，(plan, format) 查 base 的真相源
}

// ResolveAPIBase 按 (plan, format) 在服务商端点表里查 API Base，
// 与内核 resolve_api_base 同一套规则：内核 plan 是开放字符串（api/coding/token/agent），
// 查不到返回空串，由调用方决定回落。
func (p *ProviderConfig) ResolveAPIBase(plan, format string) string {
	if p == nil {
		return ""
	}
	plan = strings.ToLower(strings.TrimSpace(plan))
	format = strings.ToLower(strings.TrimSpace(format))
	for _, ep := range p.Endpoints {
		if strings.EqualFold(strings.TrimSpace(ep.Plan), plan) &&
			strings.EqualFold(strings.TrimSpace(ep.Format), format) {
			return strings.TrimSpace(ep.APIBase)
		}
	}
	return ""
}

// ProviderRegistry 服务商注册表
type ProviderRegistry struct {
	providers map[ProviderType]*ProviderConfig
	ordered   []*ProviderConfig
}

// NewProviderRegistry 创建服务商注册表
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[ProviderType]*ProviderConfig),
		ordered:   make([]*ProviderConfig, 0),
	}
}

// Get 获取服务商配置
func (pr *ProviderRegistry) Get(pt ProviderType) *ProviderConfig {
	return pr.providers[pt]
}

// GetByID 根据 ID 获取服务商配置
func (pr *ProviderRegistry) GetByID(id string) *ProviderConfig {
	for _, p := range pr.providers {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// GetAll 获取所有服务商配置
func (pr *ProviderRegistry) GetAll() []*ProviderConfig {
	var result []*ProviderConfig
	for _, p := range pr.ordered {
		if p.Type != ProviderCustom {
			result = append(result, p)
		}
	}
	return result
}

func (pr *ProviderRegistry) replaceAll(providers []*ProviderConfig) {
	pr.providers = make(map[ProviderType]*ProviderConfig, len(providers))
	pr.ordered = make([]*ProviderConfig, 0, len(providers))
	for _, p := range providers {
		if p == nil {
			continue
		}
		pr.providers[p.Type] = p
		pr.ordered = append(pr.ordered, p)
	}
}

// DetectProvider 根据 API Base URL 自动检测服务商
func (pr *ProviderRegistry) DetectProvider(baseURL string) *ProviderConfig {
	if baseURL == "" {
		return nil
	}
	b := strings.ToLower(strings.TrimSpace(baseURL))

	// 按当前运行时目录快照匹配
	for _, p := range pr.ordered {
		if p == nil || p.Type == ProviderCustom {
			continue
		}
		if p.Type == ProviderMiniMax && (strings.Contains(b, "api.minimaxi.com") || strings.Contains(b, "api.minimax.io")) {
			return p
		}
		if p.Type == ProviderMiMo && strings.Contains(b, "xiaomimimo.com") {
			return p
		}
		defaultBase := strings.ToLower(p.DefaultAPIBase)
		if defaultBase != "" && strings.Contains(b, extractDomain(defaultBase)) {
			return p
		}
		if p.CodePlanAPIBase != "" {
			codePlanBase := strings.ToLower(p.CodePlanAPIBase)
			if strings.Contains(b, extractDomain(codePlanBase)) {
				return p
			}
		}
	}

	return nil
}

// extractDomain 从 URL 中提取域名（用于匹配）
func extractDomain(url string) string {
	url = strings.TrimSpace(strings.ToLower(url))
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "ws://")
	url = strings.TrimPrefix(url, "wss://")
	if idx := strings.Index(url, "/"); idx >= 0 {
		url = url[:idx]
	}
	if idx := strings.Index(url, ":"); idx >= 0 {
		url = url[:idx]
	}
	return url
}

// globalRegistry 全局服务商注册表
var globalRegistry = NewProviderRegistry()

// GetProvider 获取服务商配置（使用全局注册表）
func GetProvider(pt ProviderType) *ProviderConfig {
	return globalRegistry.Get(pt)
}

// GetProviderByID 根据 ID 获取服务商配置（使用全局注册表）
func GetProviderByID(id string) *ProviderConfig {
	return globalRegistry.GetByID(id)
}

// GetAllProviders 获取所有服务商配置（使用全局注册表）
func GetAllProviders() []*ProviderConfig {
	return globalRegistry.GetAll()
}

// DetectProviderByBase 根据 API Base URL 自动检测服务商（使用全局注册表）
func DetectProviderByBase(baseURL string) *ProviderConfig {
	return globalRegistry.DetectProvider(baseURL)
}

// GetAPIBase 根据服务商和 API 类型返回正确的 Base URL
func GetAPIBase(provider ProviderType, apiType APIType, customBase string) string {
	if customBase != "" {
		return customBase
	}

	cfg := GetProvider(provider)
	if cfg == nil {
		return ""
	}

	switch apiType {
	case APITypeCodePlan:
		if cfg.CodePlanAPIBase != "" {
			return cfg.CodePlanAPIBase
		}
		return cfg.DefaultAPIBase
	case APITypeClaude:
		if cfg.ClaudeAPIBase != "" {
			return cfg.ClaudeAPIBase
		}
		// 如果没有专门的 Claude Base，尝试返回 CodePlan 或 Default
		if cfg.CodePlanAPIBase != "" {
			return cfg.CodePlanAPIBase
		}
		return cfg.DefaultAPIBase
	case APITypeTokenPlan:
		if cfg.TokenPlanAPIBase != "" {
			return cfg.TokenPlanAPIBase
		}
		return cfg.DefaultAPIBase
	case APITypeTokenPlanClaude:
		if cfg.TokenPlanClaudeAPIBase != "" {
			return cfg.TokenPlanClaudeAPIBase
		}
		if cfg.TokenPlanAPIBase != "" {
			return cfg.TokenPlanAPIBase
		}
		return cfg.DefaultAPIBase
	default:
		return cfg.DefaultAPIBase
	}
}

// GetCodePlanModelNames 返回当前目录快照中需要使用 Code Plan API 的模型列表。
func GetCodePlanModelNames() []string {
	result := make([]string, 0)
	seen := make(map[string]struct{})
	for _, entry := range globalCatalog.GetAll() {
		if entry == nil || (entry.APIType != APITypeCodePlan && entry.APIType != APITypeTokenPlan && entry.APIType != APITypeTokenPlanClaude) {
			continue
		}
		for _, key := range []string{entry.ID, entry.ModelName} {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			normalized := strings.ToLower(key)
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			result = append(result, key)
		}
	}
	return result
}

// IsCodePlanModel 检查模型是否需要使用 Code Plan API。
func IsCodePlanModel(modelName string) bool {
	entry := findCatalogEntryByKey(strings.ToLower(strings.TrimSpace(modelName)))
	return entry != nil && (entry.APIType == APITypeCodePlan || entry.APIType == APITypeTokenPlan || entry.APIType == APITypeTokenPlanClaude)
}

// ParseProviderType 从字符串解析服务商类型
func ParseProviderType(s string) ProviderType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "deepseek":
		return ProviderDeepSeek
	case "dashscope", "qwen", "aliyun", "alibaba":
		return ProviderDashScope
	case "bytedance", "volcengine", "volces", "doubao":
		return ProviderByteDance
	case "zhipu", "zhipuai", "glm", "chatglm":
		return ProviderZhipu
	case "moonshot", "kimi":
		return ProviderMoonshot
	case "minimax":
		return ProviderMiniMax
	case "mimo", "xiaomi", "xiaomimimo":
		return ProviderMiMo
	case "gemini", "google":
		return ProviderGemini
	case "openai":
		return ProviderOpenAI
	case "anthropic", "claude":
		return ProviderAnthropic
	default:
		return ProviderCustom
	}
}
