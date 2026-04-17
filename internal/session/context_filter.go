package session

import (
	"strings"
	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/pkg/utils"
)

// ContextFilter 上下文过滤器配置
type ContextFilter struct {
	MaxRecent    int      // 保留最近消息数
	Keywords     []string // 相关关键词
	ExcludeMeta  bool     // 是否排除元消息 (IsMeta)
	SinceToken   int      // 起始 token 位置
	MaxTokens    int      // 最大 token 数量
	IncludeRoles []string // 包含的角色 (user, assistant, system)
	ExcludeRoles []string // 排除的角色
}

// DefaultContextFilter 创建默认过滤器
func DefaultContextFilter() *ContextFilter {
	return &ContextFilter{
		MaxRecent:    10,
		ExcludeMeta:  true,
		MaxTokens:    16000,
		IncludeRoles: []string{"user", "assistant"},
	}
}

// FilterForSubAgent 为子代理类型创建过滤器
func FilterForSubAgent(agentType string) *ContextFilter {
	switch agentType {
	case "explore":
		// 探索代理：只读，独立上下文，不需要历史
		return &ContextFilter{
			MaxRecent:    2,
			ExcludeMeta:  true,
			MaxTokens:    4000,
			IncludeRoles: []string{"user"},
		}
	case "test":
		// 测试代理：独立上下文，只关注测试相关
		return &ContextFilter{
			MaxRecent:    3,
			ExcludeMeta:  true,
			MaxTokens:    8000,
			IncludeRoles: []string{"user", "assistant"},
		}
	case "review":
		// 审查代理：混合上下文，需要项目状态和变更
		return &ContextFilter{
			MaxRecent:    8,
			ExcludeMeta:  true,
			MaxTokens:    12000,
			IncludeRoles: []string{"user", "assistant"},
		}
	case "security":
		// 安全审计：混合上下文，需要完整代码结构
		return &ContextFilter{
			MaxRecent:    6,
			ExcludeMeta:  true,
			MaxTokens:    16000,
			IncludeRoles: []string{"user", "assistant"},
		}
	case "architect":
		// 架构设计：混合上下文，需要完整项目结构
		return &ContextFilter{
			MaxRecent:    10,
			ExcludeMeta:  false,
			MaxTokens:    16000,
			IncludeRoles: []string{"user", "assistant", "system"},
		}
	default:
		return DefaultContextFilter()
	}
}

// Apply 过滤消息列表
func (f *ContextFilter) Apply(msgs []ai.Message) []ai.Message {
	var filtered []ai.Message
	tokenCount := 0

	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]

		// 排除元消息
		if f.ExcludeMeta && msg.IsMeta {
			continue
		}

		// 角色过滤
		if !f.shouldIncludeRole(msg.Role) {
			continue
		}

		// 关键词过滤
		if !f.matchesKeywords(msg.Content) {
			continue
		}

		// Token 限制
		msgTokens := utils.EstimateTokensWeighted("", msg.Content)
		if f.MaxTokens > 0 && tokenCount+msgTokens > f.MaxTokens {
			continue
		}

		filtered = append([]ai.Message{msg}, filtered...)
		tokenCount += msgTokens

		// 最近消息限制
		if f.MaxRecent > 0 && len(filtered) >= f.MaxRecent {
			break
		}
	}

	return filtered
}

// shouldIncludeRole 检查是否应该包含该角色
func (f *ContextFilter) shouldIncludeRole(role string) bool {
	// 优先排除
	if len(f.ExcludeRoles) > 0 {
		for _, r := range f.ExcludeRoles {
			if r == role {
				return false
			}
		}
	}
	// 包含检查
	if len(f.IncludeRoles) > 0 {
		for _, r := range f.IncludeRoles {
			if r == role {
				return true
			}
		}
		return false
	}
	return true
}

// matchesKeywords 检查内容是否匹配关键词
func (f *ContextFilter) matchesKeywords(content string) bool {
	if len(f.Keywords) == 0 {
		return true
	}
	lowerContent := strings.ToLower(content)
	for _, kw := range f.Keywords {
		if strings.Contains(lowerContent, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// WithKeywords 设置关键词过滤
func (f *ContextFilter) WithKeywords(keywords ...string) *ContextFilter {
	f.Keywords = keywords
	return f
}

// WithMaxRecent 设置最大最近消息数
func (f *ContextFilter) WithMaxRecent(max int) *ContextFilter {
	f.MaxRecent = max
	return f
}

// WithExcludeMeta 设置是否排除元消息
func (f *ContextFilter) WithExcludeMeta(exclude bool) *ContextFilter {
	f.ExcludeMeta = exclude
	return f
}

// WithMaxTokens 设置最大 token 数
func (f *ContextFilter) WithMaxTokens(max int) *ContextFilter {
	f.MaxTokens = max
	return f
}

// WithIncludeRoles 设置包含的角色
func (f *ContextFilter) WithIncludeRoles(roles ...string) *ContextFilter {
	f.IncludeRoles = roles
	return f
}

// WithExcludeRoles 设置排除的角色
func (f *ContextFilter) WithExcludeRoles(roles ...string) *ContextFilter {
	f.ExcludeRoles = roles
	return f
}

// ContextBuilder 上下文构建器
type ContextBuilder struct {
	messages []ai.Message
	filter   *ContextFilter
}

// NewContextBuilder 创建上下文构建器
func NewContextBuilder(msgs []ai.Message) *ContextBuilder {
	return &ContextBuilder{
		messages: msgs,
		filter:   DefaultContextFilter(),
	}
}

// WithFilter 设置过滤器
func (b *ContextBuilder) WithFilter(filter *ContextFilter) *ContextBuilder {
	b.filter = filter
	return b
}

// WithAgentType 根据代理类型设置过滤器
func (b *ContextBuilder) WithAgentType(agentType string) *ContextBuilder {
	b.filter = FilterForSubAgent(agentType)
	return b
}

// Build 构建过滤后的上下文
func (b *ContextBuilder) Build() []ai.Message {
	if b.filter == nil {
		return b.messages
	}
	return b.filter.Apply(b.messages)
}

// EstimateTokens 估算消息列表的 token 数
func (b *ContextBuilder) EstimateTokens() int {
	total := 0
	for _, msg := range b.messages {
		total += utils.EstimateTokensWeighted("", msg.Content)
	}
	return total
}

// ContextFilterManager 上下文过滤器管理器
type ContextFilterManager struct {
	filters map[string]*ContextFilter
}

// NewContextFilterManager 创建过滤器管理器
func NewContextFilterManager() *ContextFilterManager {
	return &ContextFilterManager{
		filters: make(map[string]*ContextFilter),
	}
}

// Register 注册过滤器
func (m *ContextFilterManager) Register(name string, filter *ContextFilter) {
	m.filters[name] = filter
}

// Get 获取过滤器
func (m *ContextFilterManager) Get(name string) (*ContextFilter, bool) {
	filter, ok := m.filters[name]
	return filter, ok
}

// Apply 应用指定过滤器
func (m *ContextFilterManager) Apply(name string, msgs []ai.Message) []ai.Message {
	filter, ok := m.filters[name]
	if !ok {
		return msgs
	}
	return filter.Apply(msgs)
}
