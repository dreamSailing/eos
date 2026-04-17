package ai

import (
	"strings"
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
	ProviderOpenAI    ProviderType = "openai"    // OpenAI
	ProviderAnthropic ProviderType = "anthropic" // Anthropic Claude
	ProviderCustom    ProviderType = "custom"    // 自定义
)

// APIType API 类型
type APIType string

const (
	APITypeStandard APIType = "standard"  // 标准 API
	APITypeCodePlan APIType = "code-plan" // Code/Plan 套餐 API (OpenAI 兼容)
	APITypeClaude   APIType = "claude"    // Claude 兼容 API (Anthropic 协议)
)

// ProviderConfig 服务商配置
type ProviderConfig struct {
	ID              string       // 唯一标识
	Name            string       // 显示名称
	Type            ProviderType // 服务商类型
	DefaultAPIBase  string       // 默认 Base URL（标准 API）
	CodePlanAPIBase string       // Code/Plan API Base URL (OpenAI 兼容)
	ClaudeAPIBase   string       // Claude 兼容 API Base URL (Anthropic 协议)
	APIKeyEnv       string       // API Key 环境变量名
	Website         string       // 官方网站
	HasCodePlan     bool         // 是否支持 Code/Plan 套餐
	HasClaudeCode   bool         // 是否支持 Claude 兼容 API 模式
	EinoComponent   string       // Eino 组件名称（空则用 OpenAI 兼容）
	DefaultModels   []string     // 默认/推荐模型列表
}

// builtinProviders 内置服务商配置
var builtinProviders = []*ProviderConfig{
	// DeepSeek - OpenAI 兼容格式
	{
		ID:              "deepseek",
		Name:            "DeepSeek",
		Type:            ProviderDeepSeek,
		DefaultAPIBase:  "https://api.deepseek.com",
		CodePlanAPIBase: "",
		APIKeyEnv:       "DEEPSEEK_API_KEY",
		Website:         "https://platform.deepseek.com",
		HasCodePlan:     false,
		HasClaudeCode:   false,
		EinoComponent:   "deepseek",
		DefaultModels:   []string{"deepseek-chat", "deepseek-reasoner"},
	},
	// 阿里云通义千问 - OpenAI 兼容格式
	{
		ID:              "dashscope",
		Name:            "阿里云通义",
		Type:            ProviderDashScope,
		DefaultAPIBase:  "https://dashscope.aliyuncs.com/compatible-mode/v1",
		CodePlanAPIBase: "https://coding.dashscope.aliyuncs.com/v1",
		APIKeyEnv:       "DASHSCOPE_API_KEY",
		Website:         "https://tongyi.aliyun.com",
		HasCodePlan:     true,
		HasClaudeCode:   false,
		EinoComponent:   "dashscope",
		DefaultModels:   []string{"qwen3.6-plus", "dashscope-coding-plan-qwen3.6-plus", "dashscope-coding-plan-glm-5", "dashscope-coding-plan-kimi-k2.5"},
	},
	// 字节豆包 - 独立 Code/Plan URL
	{
		ID:              "bytedance",
		Name:            "字节豆包",
		Type:            ProviderByteDance,
		DefaultAPIBase:  "https://ark.cn-beijing.volces.com/api/v3",
		CodePlanAPIBase: "https://ark.cn-beijing.volces.com/api/coding/v3",
		ClaudeAPIBase:   "https://ark.cn-beijing.volces.com/api/coding",
		APIKeyEnv:       "VOLCENGINE_API_KEY",
		Website:         "https://www.volcengine.com",
		HasCodePlan:     true,
		HasClaudeCode:   true,
		EinoComponent:   "volcengine",
		DefaultModels:   []string{"doubao-seed-code", "doubao-seed-1.8", "ark-coding-plan-openai", "ark-coding-plan-claude"},
	},
	// 智谱 GLM - 独立 API 格式
	{
		ID:              "zhipu",
		Name:            "智谱 GLM",
		Type:            ProviderZhipu,
		DefaultAPIBase:  "https://open.bigmodel.cn/api/paas/v4",
		CodePlanAPIBase: "https://open.bigmodel.cn/api/coding/paas/v4",
		ClaudeAPIBase:   "https://open.bigmodel.cn/api/anthropic",
		APIKeyEnv:       "ZHIPU_API_KEY",
		Website:         "https://www.zhipuai.cn",
		HasCodePlan:     true,
		HasClaudeCode:   true,
		EinoComponent:   "zhipuai",
		DefaultModels:   []string{"glm-5", "glm-5-turbo", "zhipu-coding-plan-openai", "zhipu-coding-plan-claude"},
	},
	// Moonshot Kimi - OpenAI 兼容格式
	{
		ID:              "moonshot",
		Name:            "Moonshot",
		Type:            ProviderMoonshot,
		DefaultAPIBase:  "https://api.moonshot.cn/v1",
		CodePlanAPIBase: "",
		APIKeyEnv:       "MOONSHOT_API_KEY",
		Website:         "https://www.moonshot.cn",
		HasCodePlan:     false,
		HasClaudeCode:   false,
		EinoComponent:   "",
		DefaultModels:   []string{"kimi-k2.5"},
	},
	// MiniMax - Token Plan / OpenAI / Anthropic 兼容格式
	{
		ID:              "minimax",
		Name:            "MiniMax",
		Type:            ProviderMiniMax,
		DefaultAPIBase:  "https://api.minimaxi.com/v1",
		CodePlanAPIBase: "https://api.minimaxi.com/v1",
		ClaudeAPIBase:   "https://api.minimaxi.com/anthropic/v1",
		APIKeyEnv:       "MINIMAX_API_KEY",
		Website:         "https://platform.minimaxi.com",
		HasCodePlan:     true,
		HasClaudeCode:   true,
		EinoComponent:   "",
		DefaultModels:   []string{"minimax-token-plan-openai", "minimax-token-plan-claude"},
	},
	// Xiaomi MiMo - Token Plan / OpenAI / Anthropic 兼容格式
	{
		ID:              "mimo",
		Name:            "小米 MiMo",
		Type:            ProviderMiMo,
		DefaultAPIBase:  "https://token-plan-cn.xiaomimimo.com/v1",
		CodePlanAPIBase: "https://token-plan-cn.xiaomimimo.com/v1",
		ClaudeAPIBase:   "https://token-plan-cn.xiaomimimo.com/anthropic",
		APIKeyEnv:       "MIMO_API_KEY",
		Website:         "https://platform.xiaomimimo.com",
		HasCodePlan:     true,
		HasClaudeCode:   true,
		EinoComponent:   "",
		DefaultModels:   []string{"mimo-token-plan-openai-pro", "mimo-token-plan-openai-omni", "mimo-token-plan-claude-pro", "mimo-token-plan-claude-omni"},
	},
	// OpenAI - 标准 OpenAI 格式
	{
		ID:              "openai",
		Name:            "OpenAI",
		Type:            ProviderOpenAI,
		DefaultAPIBase:  "https://api.openai.com/v1",
		CodePlanAPIBase: "",
		APIKeyEnv:       "OPENAI_API_KEY",
		Website:         "https://openai.com",
		HasCodePlan:     false,
		HasClaudeCode:   false,
		EinoComponent:   "openai",
		DefaultModels:   []string{"gpt-5.3-codex", "gpt-4o", "o3-mini", "o3"},
	},
	// Anthropic Claude - 独立 API 格式
	{
		ID:              "anthropic",
		Name:            "Anthropic",
		Type:            ProviderAnthropic,
		DefaultAPIBase:  "https://api.anthropic.com/v1",
		CodePlanAPIBase: "",
		APIKeyEnv:       "ANTHROPIC_API_KEY",
		Website:         "https://www.anthropic.com",
		HasCodePlan:     false,
		HasClaudeCode:   false,
		EinoComponent:   "anthropic",
		DefaultModels:   []string{"claude-opus-4.6", "claude-4.5-sonnet", "claude-4.5-opus"},
	},
	// 自定义 - 用户配置
	{
		ID:              "custom",
		Name:            "自定义",
		Type:            ProviderCustom,
		DefaultAPIBase:  "",
		CodePlanAPIBase: "",
		APIKeyEnv:       "",
		Website:         "",
		HasCodePlan:     false,
		HasClaudeCode:   false,
		EinoComponent:   "",
		DefaultModels:   []string{},
	},
}

// ProviderRegistry 服务商注册表
type ProviderRegistry struct {
	providers map[ProviderType]*ProviderConfig
}

// NewProviderRegistry 创建服务商注册表
func NewProviderRegistry() *ProviderRegistry {
	pr := &ProviderRegistry{
		providers: make(map[ProviderType]*ProviderConfig),
	}
	for _, p := range builtinProviders {
		pr.providers[p.Type] = p
	}
	return pr
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
	for _, p := range builtinProviders {
		if p.Type != ProviderCustom {
			result = append(result, p)
		}
	}
	return result
}

// DetectProvider 根据 API Base URL 自动检测服务商
func (pr *ProviderRegistry) DetectProvider(baseURL string) *ProviderConfig {
	if baseURL == "" {
		return nil
	}
	b := strings.ToLower(strings.TrimSpace(baseURL))

	// 按特征字符串匹配
	for _, p := range builtinProviders {
		if p.Type == ProviderCustom {
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
	default:
		return cfg.DefaultAPIBase
	}
}

// GetCodePlanModelNames 返回需要使用 Code Plan API 的模型列表
func GetCodePlanModelNames() []string {
	return []string{
		"dashscope-coding-plan-qwen3.6-plus",
		"dashscope-coding-plan-glm-5",
		"dashscope-coding-plan-kimi-k2.5",
		"dashscope-coding-plan-minimax-m2.5",
		"ark-code-latest",
		"zhipu-coding-plan-openai",
		"zhipu-coding-plan-claude",
		"minimax-token-plan-openai",
		"minimax-token-plan-claude",
		"mimo-token-plan-openai-pro",
		"mimo-token-plan-openai-omni",
		"mimo-token-plan-claude-pro",
		"mimo-token-plan-claude-omni",
	}
}

// IsCodePlanModel 检查模型是否需要使用 Code Plan API
func IsCodePlanModel(modelName string) bool {
	name := strings.ToLower(strings.TrimSpace(modelName))
	for _, m := range GetCodePlanModelNames() {
		if strings.ToLower(m) == name {
			return true
		}
	}
	return false
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
	case "openai":
		return ProviderOpenAI
	case "anthropic", "claude":
		return ProviderAnthropic
	default:
		return ProviderCustom
	}
}
