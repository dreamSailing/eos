package config

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	ModelRolePrimary          = "primary"
	ModelRoleImageGeneration  = "image_generation"
	ModelRoleVideoGeneration  = "video_generation"
	ModelRoleSpeechSynthesis  = "speech_synthesis"
	CapabilityImageGeneration = "image_generation"
	CapabilityVideoGeneration = "video_generation"
	CapabilitySpeechSynthesis = "speech_synthesis"
)

type CapabilityModelRefs struct {
	ImageGeneration string `json:"image_generation,omitempty"`
	VideoGeneration string `json:"video_generation,omitempty"`
	SpeechSynthesis string `json:"speech_synthesis,omitempty"`
}

type ModelEntry struct {
	Name                    string `json:"name"`
	APIBase                 string `json:"api_base"`
	APIKey                  string `json:"api_key"`
	Model                   string `json:"model"`
	Source                  string `json:"source,omitempty"`
	Provider                string `json:"provider,omitempty"`                  // 服务商类型 (deepseek, dashscope, etc.)
	APIType                 string `json:"api_type,omitempty"`                  // API 类型 (standard, code-plan)
	Role                    string `json:"role,omitempty"`                      // 模型角色: primary/image_generation/video_generation/speech_synthesis
	Enabled                 *bool  `json:"enabled,omitempty"`                   // 是否启用（旧配置留空时默认 true）
	ThinkingEnabled         bool   `json:"thinking_enabled,omitempty"`          // 是否为该模型启用思考
	ThinkingCapability      string `json:"thinking_capability,omitempty"`       // "none", "low", "medium", "high"
	SupportsReasoningEffort bool   `json:"supports_reasoning_effort,omitempty"` // 是否支持 ReasoningEffort 参数
	SupportsVision          bool   `json:"supports_vision,omitempty"`           // 是否支持视觉/多模态输入
	SupportsTools           bool   `json:"supports_tools,omitempty"`            // 是否支持工具调用
	SupportsImageGeneration bool   `json:"supports_image_generation,omitempty"`
	SupportsVideoGeneration bool   `json:"supports_video_generation,omitempty"`
	SupportsSpeechSynthesis bool   `json:"supports_speech_synthesis,omitempty"`
}

// ThinkingConfig 思考模式全局配置
type ThinkingConfig struct {
	Enabled         bool     `json:"enabled"`          // 是否启用思考模式
	AutoDetect      bool     `json:"auto_detect"`      // 是否自动检测模型思考能力
	ReasoningEffort string   `json:"reasoning_effort"` // 推理级别: "low", "medium", "high"
	CustomModels    []string `json:"custom_models"`    // 额外支持思考的自定义模型列表
}

type AgentConfig struct {
	MaxStep              int `json:"max_step,omitempty"`
	InvokeTimeoutSeconds int `json:"invoke_timeout_seconds,omitempty"`
	ToolTimeoutSeconds   int `json:"tool_timeout_seconds,omitempty"`
}

// SkillsDirEntry Skills 目录配置条目
type SkillsDirEntry struct {
	Path    string `json:"path"`    // Skills 目录路径
	Enabled bool   `json:"enabled"` // 是否启用
}

// PluginEntry 插件配置条目
type PluginEntry struct {
	Name    string `json:"name"`    // 插件名称
	Enabled bool   `json:"enabled"` // 是否启用
}

// MCPClientType MCP客户端类型
type MCPClientType string

const (
	MCPTypeStdio          MCPClientType = "stdio"           // 本地命令行工具
	MCPTypeSSE            MCPClientType = "sse"             // 远程SSE服务
	MCPTypeStreamableHTTP MCPClientType = "streamable-http" // Streamable HTTP MCP transport
)

// MCPEntry MCP服务配置条目
type MCPEntry struct {
	Name                 string            `json:"name"`                             // 服务名称（唯一标识）
	Type                 MCPClientType     `json:"type"`                             // 客户端类型: "stdio" 或 "sse"
	Command              string            `json:"command,omitempty"`                // stdio: 执行命令
	Args                 []string          `json:"args,omitempty"`                   // stdio: 命令参数
	Envs                 map[string]string `json:"envs,omitempty"`                   // stdio: 环境变量
	BaseURL              string            `json:"base_url,omitempty"`               // sse: 服务URL
	Enabled              bool              `json:"enabled"`                          // 是否启用
	Auth                 *MCPAuth          `json:"auth,omitempty"`                   // 认证配置
	ApprovalMode         string            `json:"approval_mode,omitempty"`          // 服务默认审批模式覆盖
	ToolApprovalOverride map[string]string `json:"tool_approval_override,omitempty"` // 单工具审批模式覆盖
}

// MCPAuth defines authentication configuration for MCP servers
type MCPAuth struct {
	Type       string            `json:"type"`                  // "bearer", "basic", "api_key"
	Token      string            `json:"token,omitempty"`       // Bearer token or API key value
	Headers    map[string]string `json:"headers,omitempty"`     // Custom headers to inject
	HeadersEnv map[string]string `json:"headers_env,omitempty"` // Header names whose values come from env vars
}

// LSPConfig LSP 配置
type LSPConfig struct {
	Enabled    *bool           `json:"enabled,omitempty"`     // 是否启用 LSP（默认 true）
	AutoDetect *bool           `json:"auto_detect,omitempty"` // 自动检测语言服务器（默认 true）
	Go         LSPServerConfig `json:"go,omitempty"`          // Go 语言配置
	Python     LSPServerConfig `json:"python,omitempty"`      // Python 语言配置
	TypeScript LSPServerConfig `json:"typescript,omitempty"`  // TypeScript 语言配置
}

func (c LSPConfig) EnabledValue() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

func (c LSPConfig) AutoDetectValue() bool {
	if c.AutoDetect == nil {
		return true
	}
	return *c.AutoDetect
}

// LSPServerConfig LSP 服务器配置
type LSPServerConfig struct {
	Enabled bool     `json:"enabled"`           // 是否启用
	Command string   `json:"command,omitempty"` // 自定义命令（留空使用自动检测）
	Args    []string `json:"args,omitempty"`    // 自定义参数
}

// PermissionsConfig defines tool permissions in config
type PermissionsConfig struct {
	AccessMode   string   `json:"access_mode,omitempty"`
	ApprovalMode string   `json:"approval_mode,omitempty"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
	DeniedTools  []string `json:"denied_tools,omitempty"`
}

// RemotePlatformType 远程仓库平台类型
type RemotePlatformType string

const (
	RemotePlatformGitHub RemotePlatformType = "github"
	RemotePlatformGitee  RemotePlatformType = "gitee"
)

// RemoteOAuthAppConfig 远程平台 OAuth 应用配置
type RemoteOAuthAppConfig struct {
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	RedirectURI  string `json:"redirect_uri,omitempty"`
}

// RemoteProviderConfig 远程平台配置
type RemoteProviderConfig struct {
	OAuth       RemoteOAuthAppConfig `json:"oauth,omitempty"`
	AccessToken string               `json:"access_token,omitempty"` // 可选：预置 token，便于服务端或 CI 场景
	Username    string               `json:"username,omitempty"`     // 可选：与预置 token 搭配使用
}

// RemoteAuthToken 持久化保存的授权令牌
type RemoteAuthToken struct {
	Platform     RemotePlatformType `json:"platform"`
	AccountID    string             `json:"account_id,omitempty"`
	AccountName  string             `json:"account_name,omitempty"`
	Login        string             `json:"login,omitempty"`
	AccessToken  string             `json:"access_token,omitempty"`
	RefreshToken string             `json:"refresh_token,omitempty"`
	TokenType    string             `json:"token_type,omitempty"`
	Scope        string             `json:"scope,omitempty"`
	ExpiryUnix   int64              `json:"expiry_unix,omitempty"`
}

// RemoteRepoEntry 远程仓库缓存信息
type RemoteRepoEntry struct {
	Platform      RemotePlatformType `json:"platform"`
	RepoURL       string             `json:"repo_url"`
	Owner         string             `json:"owner,omitempty"`
	Repo          string             `json:"repo,omitempty"`
	DefaultBranch string             `json:"default_branch,omitempty"`
	LocalPath     string             `json:"local_path,omitempty"`
	LastBranch    string             `json:"last_branch,omitempty"`
	LastUsedUnix  int64              `json:"last_used_unix,omitempty"`
}

type Config struct {
	Models                       []ModelEntry                    `json:"models,omitempty"`
	Active                       string                          `json:"active_model,omitempty"`
	CapabilityModels             CapabilityModelRefs             `json:"capability_models,omitempty"`
	Thinking                     ThinkingConfig                  `json:"thinking,omitempty"` // 思考模式配置
	NextMessagePredictionEnabled *bool                           `json:"next_message_prediction_enabled,omitempty"`
	Agent                        AgentConfig                     `json:"agent,omitempty"`
	MCP                          []MCPEntry                      `json:"mcp,omitempty"`                // MCP服务配置
	Skills                       []SkillsDirEntry                `json:"skills,omitempty"`             // Skills 目录配置
	DisabledSkills               []string                        `json:"disabled_skills,omitempty"`    // 被禁用的 skill 名称
	Plugins                      []PluginEntry                   `json:"plugins,omitempty"`            // 插件启停覆盖配置
	LSP                          LSPConfig                       `json:"lsp,omitempty"`                // LSP 配置
	KnownWorkspaces              []string                        `json:"known_workspaces,omitempty"`   // 已知工作区（绝对路径）
	LastWorkspace                string                          `json:"last_workspace,omitempty"`     // 上次前台工作区（绝对路径）
	TrustedWorkspaces            []string                        `json:"trusted_workspaces,omitempty"` // 已信任的工作区（绝对路径）
	Language                     string                          `json:"language,omitempty"`           // 语言设置 (zh, en)
	LogDir                       string                          `json:"log_dir,omitempty"`            // 全局日志目录
	FastModel                    string                          `json:"fast_model,omitempty"`         // Fast mode model name
	Permissions                  *PermissionsConfig              `json:"permissions,omitempty"`        // Tool permissions
	RemoteProviders              map[string]RemoteProviderConfig `json:"remote_providers,omitempty"`   // GitHub/Gitee OAuth/Token 配置
	RemoteAuth                   map[string]RemoteAuthToken      `json:"remote_auth,omitempty"`        // 已授权账号（按平台）
	RemoteRepos                  []RemoteRepoEntry               `json:"remote_repos,omitempty"`       // 最近访问的远程仓库
}

func boolPtr(v bool) *bool {
	return &v
}

func NormalizeModelRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "", ModelRolePrimary:
		return ModelRolePrimary
	case ModelRoleImageGeneration:
		return ModelRoleImageGeneration
	case ModelRoleVideoGeneration:
		return ModelRoleVideoGeneration
	case ModelRoleSpeechSynthesis:
		return ModelRoleSpeechSynthesis
	default:
		return strings.ToLower(strings.TrimSpace(role))
	}
}

func ModelRoleValue(entry ModelEntry) string {
	return NormalizeModelRole(entry.Role)
}

func ModelEnabled(entry ModelEntry) bool {
	if entry.Enabled == nil {
		return true
	}
	return *entry.Enabled
}

func SupportsCapability(entry ModelEntry, capability string) bool {
	switch strings.ToLower(strings.TrimSpace(capability)) {
	case CapabilityImageGeneration:
		return entry.SupportsImageGeneration || ModelRoleValue(entry) == ModelRoleImageGeneration
	case CapabilityVideoGeneration:
		return entry.SupportsVideoGeneration || ModelRoleValue(entry) == ModelRoleVideoGeneration
	case CapabilitySpeechSynthesis:
		return entry.SupportsSpeechSynthesis || ModelRoleValue(entry) == ModelRoleSpeechSynthesis
	default:
		return false
	}
}

func (r CapabilityModelRefs) Get(capability string) string {
	switch strings.ToLower(strings.TrimSpace(capability)) {
	case CapabilityImageGeneration:
		return strings.TrimSpace(r.ImageGeneration)
	case CapabilityVideoGeneration:
		return strings.TrimSpace(r.VideoGeneration)
	case CapabilitySpeechSynthesis:
		return strings.TrimSpace(r.SpeechSynthesis)
	default:
		return ""
	}
}

func (r *CapabilityModelRefs) Set(capability, modelName string) bool {
	if r == nil {
		return false
	}
	name := strings.TrimSpace(modelName)
	switch strings.ToLower(strings.TrimSpace(capability)) {
	case CapabilityImageGeneration:
		if r.ImageGeneration == name {
			return false
		}
		r.ImageGeneration = name
		return true
	case CapabilityVideoGeneration:
		if r.VideoGeneration == name {
			return false
		}
		r.VideoGeneration = name
		return true
	case CapabilitySpeechSynthesis:
		if r.SpeechSynthesis == name {
			return false
		}
		r.SpeechSynthesis = name
		return true
	default:
		return false
	}
}

func FindModelByName(cfg Config, name string) (ModelEntry, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ModelEntry{}, false
	}
	for _, m := range cfg.Models {
		if strings.EqualFold(strings.TrimSpace(m.Name), name) {
			return m, true
		}
	}
	return ModelEntry{}, false
}

func ResolveCapabilityModel(cfg Config, capability string) (ModelEntry, bool) {
	ref := cfg.CapabilityModels.Get(capability)
	if ref == "" {
		return ModelEntry{}, false
	}
	entry, ok := FindModelByName(cfg, ref)
	if !ok || !ModelEnabled(entry) {
		return ModelEntry{}, false
	}
	return entry, true
}

func ClearCapabilityModelRefsForModel(cfg *Config, name string) bool {
	if cfg == nil {
		return false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	changed := false
	if strings.EqualFold(strings.TrimSpace(cfg.CapabilityModels.ImageGeneration), name) {
		cfg.CapabilityModels.ImageGeneration = ""
		changed = true
	}
	if strings.EqualFold(strings.TrimSpace(cfg.CapabilityModels.VideoGeneration), name) {
		cfg.CapabilityModels.VideoGeneration = ""
		changed = true
	}
	if strings.EqualFold(strings.TrimSpace(cfg.CapabilityModels.SpeechSynthesis), name) {
		cfg.CapabilityModels.SpeechSynthesis = ""
		changed = true
	}
	return changed
}

func DefaultLogDir() string {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if strings.TrimSpace(base) == "" {
			base = os.Getenv("APPDATA")
		}
		if strings.TrimSpace(base) == "" {
			base, _ = os.UserHomeDir()
		}
		return filepath.Join(base, "EOS", "logs")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Logs", "EOS")
	default:
		base := os.Getenv("XDG_STATE_HOME")
		if strings.TrimSpace(base) == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, ".local", "state")
		}
		return filepath.Join(base, "eos", "logs")
	}
}

func ConfiguredLogDir() string {
	cfg, _ := Load()
	return ResolveLogDir(cfg.LogDir)
}

func ResolveLogDir(value string) string {
	trimmed := strings.TrimSpace(os.ExpandEnv(value))
	if trimmed == "" {
		return DefaultLogDir()
	}
	if strings.HasPrefix(trimmed, "~") {
		home, err := os.UserHomeDir()
		if err == nil && strings.TrimSpace(home) != "" {
			if trimmed == "~" {
				trimmed = home
			} else if strings.HasPrefix(trimmed, "~/") || strings.HasPrefix(trimmed, "~\\") {
				trimmed = filepath.Join(home, trimmed[2:])
			}
		}
	}
	if !filepath.IsAbs(trimmed) {
		if abs, err := filepath.Abs(trimmed); err == nil {
			trimmed = abs
		}
	}
	return filepath.Clean(trimmed)
}

func NextMessagePredictionEnabled(cfg *Config) bool {
	if cfg == nil || cfg.NextMessagePredictionEnabled == nil {
		return true
	}
	return *cfg.NextMessagePredictionEnabled
}

func Path() string {
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Error("config.path.user_home_dir.error",
			"error", err)
		return ".eos.json"
	}
	return filepath.Join(home, ".eos.json")
}

func Load() (Config, string) {
	p := Path()
	var cfg Config
	b, err := os.ReadFile(p)
	if err != nil {
		slog.Debug("config.load.file_not_found",
			"path", p,
			"error", err)
		return cfg, p
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		slog.Error("config.load.unmarshal.error",
			"path", p,
			"data_size", len(b),
			"error", err)
	}
	if NormalizeWorkspaceState(&cfg) {
		if err := Save(cfg, p); err != nil {
			slog.Warn("config.load.normalize_workspace_state.save.error", "path", p, "error", err.Error())
		}
	}

	if len(cfg.MCP) == 0 {
		if migrated, ok := tryMigrateLegacyMCPServers(b, &cfg); ok {
			if err := Save(cfg, p); err != nil {
				slog.Warn("config.load.migrate_mcp.save.error", "path", p, "error", err.Error())
			} else {
				slog.Info("config.load.migrate_mcp.save.success", "path", p, "mcp_count", len(migrated))
			}
		}
	}
	slog.Debug("config.load.success",
		"path", p,
		"models_count", len(cfg.Models),
		"active_model", cfg.Active)
	return cfg, p
}

func tryMigrateLegacyMCPServers(b []byte, cfg *Config) ([]MCPEntry, bool) {
	entries, err := ParseLegacyMCPServersJSON(b)
	if err != nil || len(entries) == 0 {
		return nil, false
	}
	cfg.MCP = entries
	return entries, true
}

func ParseLegacyMCPServersJSON(b []byte) ([]MCPEntry, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		return nil, err
	}
	raw, ok := top["mcpServers"]
	if !ok || len(raw) == 0 {
		return nil, errors.New("missing mcpServers")
	}
	return parseLegacyMCPServersRaw(raw)
}

func parseLegacyMCPServersRaw(raw json.RawMessage) ([]MCPEntry, error) {
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(raw, &servers); err != nil {
		return nil, err
	}
	if len(servers) == 0 {
		return nil, nil
	}

	entries := make([]MCPEntry, 0, len(servers))
	for name, body := range servers {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		var obj map[string]any
		_ = json.Unmarshal(body, &obj)

		entry := MCPEntry{
			Name:    name,
			Enabled: true,
		}

		if v, ok := obj["enabled"].(bool); ok {
			entry.Enabled = v
		}
		if v, ok := obj["type"].(string); ok {
			entry.Type = MCPClientType(strings.TrimSpace(v))
		}
		if v, ok := obj["command"].(string); ok {
			entry.Command = strings.TrimSpace(v)
		}
		if v, ok := obj["args"].([]any); ok {
			args := make([]string, 0, len(v))
			for _, a := range v {
				if s, ok := a.(string); ok {
					s = strings.TrimSpace(s)
					if s != "" {
						args = append(args, s)
					}
				}
			}
			entry.Args = args
		}
		if v, ok := obj["base_url"].(string); ok {
			entry.BaseURL = strings.TrimSpace(v)
		}
		if entry.BaseURL == "" {
			if v, ok := obj["url"].(string); ok {
				entry.BaseURL = strings.TrimSpace(v)
			}
		}

		if envs := parseLegacyEnvMap(obj["envs"]); len(envs) > 0 {
			entry.Envs = envs
		} else if env := parseLegacyEnvMap(obj["env"]); len(env) > 0 {
			entry.Envs = env
		}
		if v, ok := obj["approval_mode"].(string); ok {
			entry.ApprovalMode = strings.TrimSpace(v)
		}
		if rawOverrides, ok := obj["tool_approval_override"].(map[string]any); ok {
			overrides := make(map[string]string, len(rawOverrides))
			for toolName, value := range rawOverrides {
				toolName = strings.TrimSpace(toolName)
				text, _ := value.(string)
				text = strings.TrimSpace(text)
				if toolName == "" || text == "" {
					continue
				}
				overrides[toolName] = text
			}
			if len(overrides) > 0 {
				entry.ToolApprovalOverride = overrides
			}
		}

		if entry.Type == "" {
			if entry.BaseURL != "" {
				entry.Type = MCPTypeSSE
			} else {
				entry.Type = MCPTypeStdio
			}
		}

		entries = append(entries, entry)
	}
	return entries, nil
}

func parseLegacyEnvMap(v any) map[string]string {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, vv := range m {
		ks := strings.TrimSpace(k)
		if ks == "" {
			continue
		}
		if s, ok := vv.(string); ok {
			out[ks] = s
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func Save(cfg Config, p string) error {
	bs, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		slog.Error("config.save.marshal.error",
			"path", p,
			"models_count", len(cfg.Models),
			"active_model", cfg.Active,
			"error", err)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		slog.Error("config.save.mkdir_all.error",
			"path", p,
			"error", err)
		return err
	}
	if err := os.WriteFile(p, bs, 0600); err != nil {
		slog.Error("config.save.write_file.error",
			"path", p,
			"data_size", len(bs),
			"error", err)
		return err
	}
	slog.Debug("config.save.success",
		"path", p,
		"models_count", len(cfg.Models),
		"active_model", cfg.Active)
	return nil
}

func InferDefaultModel(base string) string {
	// 内置服务商的默认模型映射
	providerDefaults := map[string]string{
		"api.deepseek.com":                  "deepseek-v4-pro",
		"dashscope.aliyuncs.com":            "qwen3.6-plus",
		"ark.cn-beijing.volces.com":         "doubao-seed-code-preview-251028",
		"open.bigmodel.cn":                  "glm-5",
		"api.moonshot.cn":                   "kimi-k2.6",
		"api.minimaxi.com":                  "MiniMax-M2.7",
		"api.minimax.io":                    "MiniMax-M2.7",
		"token-plan-cn.xiaomimimo.com":      "mimo-v2.5-pro",
		"generativelanguage.googleapis.com": "gemini-3.1-pro-preview",
		"api.openai.com":                    "gpt-5.5",
		"api.anthropic.com":                 "claude-opus-4-7",
	}

	b := strings.ToLower(strings.TrimSpace(base))

	// 检查已知服务商
	for domain, model := range providerDefaults {
		if strings.Contains(b, domain) {
			// 特殊处理：字节豆包 Code Plan API
			if domain == "ark.cn-beijing.volces.com" {
				if strings.Contains(b, "coding") {
					if strings.Contains(b, "/v3") {
						return "ark-code-latest" // OpenAI 兼容版本
					}
					return "ark-code-latest-claude" // Anthropic 兼容版本
				}
			}
			return model
		}
	}

	// 回退到旧的检测逻辑
	if strings.Contains(b, "api.deepseek.com") {
		return "deepseek-v4-pro"
	}
	if strings.Contains(b, "dashscope.aliyuncs.com") && strings.Contains(b, "compatible-mode") {
		return "qwen3.6-plus"
	}
	if strings.Contains(b, "api.moonshot.cn") {
		return "kimi-k2.6"
	}
	if strings.Contains(b, "api.minimaxi.com") || strings.Contains(b, "api.minimax.io") {
		return "MiniMax-M2.7"
	}
	if strings.Contains(b, "xiaomimimo.com") {
		return "mimo-v2.5-pro"
	}
	if strings.Contains(b, "generativelanguage.googleapis.com") {
		return "gemini-3.1-pro-preview"
	}

	return ""
}

func ActiveModel(cfg Config) (ModelEntry, bool) {
	for _, m := range cfg.Models {
		if m.Name == cfg.Active {
			return m, true
		}
	}
	return ModelEntry{}, false
}

func SetActive(cfg *Config, name string) bool {
	found := false
	for _, m := range cfg.Models {
		if m.Name == name {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	cfg.Active = name
	return true
}

func AddModel(cfg *Config, entry ModelEntry) bool {
	entry.Role = ModelRoleValue(entry)
	if entry.Enabled == nil {
		entry.Enabled = boolPtr(true)
	}
	for _, m := range cfg.Models {
		if m.Name == entry.Name {
			return false
		}
	}
	cfg.Models = append(cfg.Models, entry)
	return true
}

func UpdateModel(cfg *Config, entry ModelEntry) bool {
	entry.Role = ModelRoleValue(entry)
	if entry.Enabled == nil {
		entry.Enabled = boolPtr(true)
	}
	for i, m := range cfg.Models {
		if m.Name == entry.Name {
			if strings.ToLower(m.Source) == "env" {
				return false
			}
			cfg.Models[i] = entry
			return true
		}
	}
	return false
}

func DeleteModel(cfg *Config, name string) bool {
	idx := -1
	for i, m := range cfg.Models {
		if m.Name == name {
			idx = i
			if strings.ToLower(m.Source) == "env" {
				return false
			}
		}
	}
	if idx < 0 {
		return false
	}
	if cfg.Active == name {
		return false
	}
	ClearCapabilityModelRefsForModel(cfg, name)
	cfg.Models = append(cfg.Models[:idx], cfg.Models[idx+1:]...)
	return true
}

// AddMCPServer 添加MCP服务
func AddMCPServer(cfg *Config, entry MCPEntry) bool {
	for _, m := range cfg.MCP {
		if m.Name == entry.Name {
			return false
		}
	}
	cfg.MCP = append(cfg.MCP, entry)
	return true
}

// UpdateMCPServer 更新MCP服务
func UpdateMCPServer(cfg *Config, entry MCPEntry) bool {
	for i, m := range cfg.MCP {
		if m.Name == entry.Name {
			cfg.MCP[i] = entry
			return true
		}
	}
	return false
}

// DeleteMCPServer 删除MCP服务
func DeleteMCPServer(cfg *Config, name string) bool {
	idx := -1
	for i, m := range cfg.MCP {
		if m.Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	cfg.MCP = append(cfg.MCP[:idx], cfg.MCP[idx+1:]...)
	return true
}

// GetEnabledMCPServers 获取所有启用的MCP服务
func GetEnabledMCPServers(cfg *Config) []MCPEntry {
	var enabled []MCPEntry
	for _, m := range cfg.MCP {
		if m.Enabled {
			enabled = append(enabled, m)
		}
	}
	return enabled
}

// ToggleMCPServer 切换MCP服务的启用状态
func ToggleMCPServer(cfg *Config, name string) bool {
	for i, m := range cfg.MCP {
		if m.Name == name {
			cfg.MCP[i].Enabled = !cfg.MCP[i].Enabled
			return true
		}
	}
	return false
}

// AddSkillsDir 添加 Skills 目录
func AddSkillsDir(cfg *Config, entry SkillsDirEntry) bool {
	for _, s := range cfg.Skills {
		if s.Path == entry.Path {
			return false
		}
	}
	cfg.Skills = append(cfg.Skills, entry)
	return true
}

// UpdateSkillsDir 更新 Skills 目录
func UpdateSkillsDir(cfg *Config, entry SkillsDirEntry) bool {
	for i, s := range cfg.Skills {
		if s.Path == entry.Path {
			cfg.Skills[i] = entry
			return true
		}
	}
	return false
}

// DeleteSkillsDir 删除 Skills 目录
func DeleteSkillsDir(cfg *Config, path string) bool {
	idx := -1
	for i, s := range cfg.Skills {
		if s.Path == path {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	cfg.Skills = append(cfg.Skills[:idx], cfg.Skills[idx+1:]...)
	return true
}

// GetEnabledSkillsDirs 获取所有启用的 Skills 目录
func GetEnabledSkillsDirs(cfg *Config) []string {
	var paths []string
	for _, s := range cfg.Skills {
		if s.Enabled {
			paths = append(paths, s.Path)
		}
	}
	return paths
}

// ToggleSkillsDir 切换 Skills 目录的启用状态
func ToggleSkillsDir(cfg *Config, path string) bool {
	for i, s := range cfg.Skills {
		if s.Path == path {
			cfg.Skills[i].Enabled = !cfg.Skills[i].Enabled
			return true
		}
	}
	return false
}

// IsSkillDisabled 检查 skill 是否被禁用
func IsSkillDisabled(cfg *Config, name string) bool {
	if cfg == nil {
		return false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, skillName := range cfg.DisabledSkills {
		if strings.EqualFold(strings.TrimSpace(skillName), name) {
			return true
		}
	}
	return false
}

// SetSkillDisabled 设置 skill 禁用状态
func SetSkillDisabled(cfg *Config, name string, disabled bool) bool {
	if cfg == nil {
		return false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	idx := -1
	for i, skillName := range cfg.DisabledSkills {
		if strings.EqualFold(strings.TrimSpace(skillName), name) {
			idx = i
			break
		}
	}
	if disabled {
		if idx >= 0 {
			return false
		}
		cfg.DisabledSkills = append(cfg.DisabledSkills, name)
		return true
	}
	if idx < 0 {
		return false
	}
	cfg.DisabledSkills = append(cfg.DisabledSkills[:idx], cfg.DisabledSkills[idx+1:]...)
	return true
}

// PluginEnabled 获取插件启用状态，未配置时默认启用。
func PluginEnabled(cfg *Config, name string) (bool, bool) {
	if cfg == nil {
		return true, false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return true, false
	}
	for _, plugin := range cfg.Plugins {
		if strings.EqualFold(strings.TrimSpace(plugin.Name), name) {
			return plugin.Enabled, true
		}
	}
	return true, false
}

// SetPluginEnabled 设置插件启用状态。默认状态为启用，因此启用时会清理显式覆盖。
func SetPluginEnabled(cfg *Config, name string, enabled bool) bool {
	if cfg == nil {
		return false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	idx := -1
	for i, plugin := range cfg.Plugins {
		if strings.EqualFold(strings.TrimSpace(plugin.Name), name) {
			idx = i
			break
		}
	}
	if enabled {
		if idx < 0 {
			return false
		}
		cfg.Plugins = append(cfg.Plugins[:idx], cfg.Plugins[idx+1:]...)
		return true
	}
	if idx >= 0 {
		if !cfg.Plugins[idx].Enabled {
			return false
		}
		cfg.Plugins[idx].Enabled = false
		return true
	}
	cfg.Plugins = append(cfg.Plugins, PluginEntry{Name: name, Enabled: false})
	return true
}
