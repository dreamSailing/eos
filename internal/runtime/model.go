package runtime

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/dreamSailing/eos/internal/ai"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var (
	ErrMissingAPISettings = errors.New("missing API settings: please configure ~/.eos.json or environment variables")
	ErrMissingModelName   = errors.New("model not specified and provider unknown for base")
)

type RuntimeModel struct {
	cm       model.ToolCallingChatModel
	name     string
	base     string
	provider ai.ProviderType
}

type ToolCallingProvider interface {
	ToolCalling() model.ToolCallingChatModel
}

func NewChatModel(ctx context.Context) (AIModel, error) {
	return NewChatModelWithReasoning(ctx, "")
}

// NewChatModelWithReasoning 创建带有推理级别的模型
// reasoningLevel: "low", "medium", "high" 或 ""（不启用推理）
func NewChatModelWithReasoning(ctx context.Context, reasoningLevel string) (AIModel, error) {
	apiKey, base, name := ai.ResolveAPISettings()
	return NewChatModelWithSettings(ctx, apiKey, base, name, reasoningLevel)
}

func NewChatModelWithSettings(ctx context.Context, apiKey, base, name, reasoningLevel string) (AIModel, error) {
	if apiKey == "" || base == "" {
		return nil, ErrMissingAPISettings
	}
	if strings.TrimSpace(name) == "" {
		return nil, ErrMissingModelName
	}

	// 解析服务商信息
	providerInfo := ai.ResolveProviderAndModel(base, name)
	providerType := ai.ProviderCustom
	apiType := ai.APITypeStandard

	if providerInfo != nil {
		providerType = providerInfo.ProviderType
		if providerInfo.Model != nil {
			apiType = providerInfo.Model.APIType
		}
		// 如果检测到 Code Plan API 且模型需要，使用对应的 API Base
		if providerInfo.RequiresCodePlan || ai.IsCodePlanModel(name) {
			if providerInfo.APIBase != "" {
				base = providerInfo.APIBase
			}
		}
	}

	var cm model.ToolCallingChatModel
	var err error

	// 根据协议类型和提供商选择组件
	switch {
	case apiType == ai.APITypeClaude:
		// 使用 Claude (Anthropic) 协议组件
		slog.Debug("model.init.claude_protocol", "model", name, "base", base)
		window := ai.ContextWindowTokens(name)
		maxTokens := 4096
		if window > 0 {
			v := window / 10
			if v < 1024 {
				v = 1024
			}
			if v > 8192 {
				v = 8192
			}
			maxTokens = v
		}
		cm, err = claude.NewChatModel(ctx, &claude.Config{
			APIKey:    apiKey,
			BaseURL:   ptr(base),
			Model:     name,
			MaxTokens: maxTokens,
		})

	case providerType == ai.ProviderByteDance && (apiType == ai.APITypeCodePlan || apiType == ai.APITypeStandard):
		// 使用火山 Ark 专用组件（支持 OpenAI 兼容格式但针对火山优化）
		slog.Debug("model.init.ark_component", "model", name, "base", base)
		maxTokens := 16384
		cm, err = ark.NewChatModel(ctx, &ark.ChatModelConfig{
			APIKey:    apiKey,
			BaseURL:   base,
			Model:     name,
			MaxTokens: &maxTokens,
		})

	default:
		// 默认使用通用 OpenAI 组件
		slog.Debug("model.init.openai_component", "model", name, "base", base)
		cfg := &openai.ChatModelConfig{
			APIKey:  apiKey,
			BaseURL: base,
			Model:   name,
			ExtraFields: map[string]any{
				"max_tokens": 16384,
			},
		}

		// 如果指定了推理级别且模型支持，添加 ReasoningEffort
		if reasoningLevel != "" && ai.BuiltinSupportsReasoningEffort(name) {
			switch strings.ToLower(reasoningLevel) {
			case "low":
				cfg.ReasoningEffort = openai.ReasoningEffortLevelLow
			case "medium":
				cfg.ReasoningEffort = openai.ReasoningEffortLevelMedium
			case "high":
				cfg.ReasoningEffort = openai.ReasoningEffortLevelHigh
			default:
				cfg.ReasoningEffort = openai.ReasoningEffortLevelMedium
			}
			slog.Debug("model.reasoning_enabled", "model", name, "level", reasoningLevel)
		}

		cm, err = openai.NewChatModel(ctx, cfg)
	}

	if err != nil {
		return nil, err
	}
	return &RuntimeModel{cm: cm, name: name, base: base, provider: providerType}, nil
}

func (m *RuntimeModel) Chat(ctx context.Context, messages []ai.Message) (string, error) {
	msgs := m.convertMessages(messages)
	out, err := m.cm.Generate(ctx, msgs)
	if err != nil {
		slog.Error("chat.generate.error", "base", m.base, "model", m.name, "err", err)
		return "", err
	}
	return out.Content, nil
}

func (m *RuntimeModel) ChatStream(ctx context.Context, messages []ai.Message, onDelta func(string), onReasoning func(string)) (string, error) {
	msgs := m.convertMessages(messages)
	sr, err := m.cm.Stream(ctx, msgs)
	if err != nil {
		slog.Error("chat.stream.error", "base", m.base, "model", m.name, "err", err)
		return "", err
	}
	defer sr.Close()
	var full string
	for {
		chunk, e := sr.Recv()
		if e != nil {
			break
		}
		// 处理推理内容（o1 模型的思考过程）
		if chunk != nil && chunk.ReasoningContent != "" {
			if onReasoning != nil {
				onReasoning(chunk.ReasoningContent)
			}
			slog.Debug("chat.stream.reasoning", "length", len(chunk.ReasoningContent))
		}
		// 处理普通内容
		if chunk != nil && chunk.Content != "" {
			if onDelta != nil {
				onDelta(chunk.Content)
			}
			full += chunk.Content
		}
	}
	return full, nil
}

// convertMessages 将内部消息格式转换为 Eino schema 格式，支持多模态
func (m *RuntimeModel) convertMessages(messages []ai.Message) []*schema.Message {
	msgs := make([]*schema.Message, 0, len(messages))
	supportsVision := ai.SupportsVisionFromCatalog(m.name)

	for _, msg := range messages {
		s := strings.TrimSpace(msg.Content)
		if s == "" && len(msg.ImagePaths) == 0 {
			continue
		}

		var einoMsg *schema.Message
		switch msg.Role {
		case "system":
			einoMsg = schema.SystemMessage(s)
		case "user":
			if supportsVision && len(msg.ImagePaths) > 0 {
				// 构造多模态消息
				parts := make([]schema.MessageInputPart, 0, 1+len(msg.ImagePaths))
				if s != "" {
					parts = append(parts, schema.MessageInputPart{
						Type: schema.ChatMessagePartTypeText,
						Text: s,
					})
				}
				for _, path := range msg.ImagePaths {
					if data, err := os.ReadFile(path); err == nil {
						mime := getMimeType(path)
						b64 := base64.StdEncoding.EncodeToString(data)
						parts = append(parts, schema.MessageInputPart{
							Type: schema.ChatMessagePartTypeImageURL,
							Image: &schema.MessageInputImage{
								MessagePartCommon: schema.MessagePartCommon{
									URL: ptr("data:" + mime + ";base64," + b64),
								},
								Detail: schema.ImageURLDetailAuto,
							},
						})
						slog.Debug("model.convert_messages.add_image", "path", path, "mime", mime)
					} else {
						slog.Warn("model.convert_messages.read_image_failed", "path", path, "err", err)
					}
				}
				einoMsg = &schema.Message{
					Role:                  schema.User,
					UserInputMultiContent: parts,
				}
			} else {
				einoMsg = schema.UserMessage(s)
				if len(msg.ImagePaths) > 0 {
					slog.Info("model.convert_messages.vision_unsupported", "model", m.name, "images_count", len(msg.ImagePaths))
				}
			}
		default:
			einoMsg = schema.AssistantMessage(s, nil)
		}

		if einoMsg != nil {
			msgs = append(msgs, einoMsg)
		}
	}
	return msgs
}

func ptr[T any](v T) *T {
	return &v
}

func getMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

func (m *RuntimeModel) ToolCalling() model.ToolCallingChatModel {
	return m.cm
}

func (m *RuntimeModel) Name() string {
	return m.name
}

func (m *RuntimeModel) Base() string {
	return m.base
}

func (rt *EinoRuntime) Summarize(ctx context.Context, text string) (string, error) {
	if rt.model == nil {
		return "", nil
	}
	sys := ai.Message{Role: "system", Content: SummarizeToolOutputPrompt}
	usr := ai.Message{Role: "user", Content: text}
	out, err := rt.model.Chat(ctx, []ai.Message{sys, usr})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
