package runtime

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"github.com/dreamSailing/eos/internal/pkg/utils"
)

// MentionType 提及类型
type MentionType int

const (
	// MentionSubAgent 子代理提及
	MentionSubAgent MentionType = iota
	// MentionSkill Skill 提及
	MentionSkill
)

func (t MentionType) String() string {
	switch t {
	case MentionSubAgent:
		return "subagent"
	case MentionSkill:
		return "skill"
	default:
		return "unknown"
	}
}

// Mention 提及信息
type Mention struct {
	Type      MentionType
	Name      string
	Alias     string // 如 @explore 中的 "explore"
	Arguments string // 提及后的参数
	FullMatch string // 完整匹配的字符串
	StartPos  int    // 在原文中的位置
	EndPos    int    // 在原文中的位置
}

// MentionParser 提及解析器
type MentionParser struct {
	subAgentAliases map[string]SubAgentType
	skillAliases    map[string]string
	enabledSkills   map[string]bool
	mentionPattern  *regexp.Regexp
}

// NewMentionParser 创建提及解析器
func NewMentionParser() *MentionParser {
	// 匹配 @alias 或 @alias:arguments 格式
	// Go 不支持正向预查 (?=...)，改用直接匹配方式
	pattern := regexp.MustCompile(`@(\w+)(?::\s*(.+?))?`)
	return &MentionParser{
		subAgentAliases: map[string]SubAgentType{
			"explore":   SubAgentTypeExplore,
			"review":    SubAgentTypeReviewer,
			"test":      SubAgentTypeTester,
			"security":  SubAgentTypeSecurity,
			"architect": SubAgentTypeArchitect,
			"planner":   SubAgentTypePlanner,
			"dev":       SubAgentTypeSeniorDev,
		},
		skillAliases:   make(map[string]string),
		enabledSkills:  make(map[string]bool),
		mentionPattern: pattern,
	}
}

// RegisterSkill 注册 skill 别名
func (p *MentionParser) RegisterSkill(name, alias string) {
	p.skillAliases[alias] = name
}

// SetSkillEnabled 设置 skill 是否可用
func (p *MentionParser) SetSkillEnabled(name string, enabled bool) {
	p.enabledSkills[name] = enabled
}

// Parse 解析用户输入中的提及
func (p *MentionParser) Parse(input string) []Mention {
	var mentions []Mention
	matches := p.mentionPattern.FindAllStringSubmatchIndex(input, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		fullMatch := input[match[0]:match[1]]
		alias := input[match[2]:match[3]]
		var arguments string
		if match[4] >= 0 {
			arguments = strings.TrimSpace(input[match[4]:match[5]])
		}
		mention := Mention{
			Alias:     alias,
			Arguments: arguments,
			FullMatch: fullMatch,
			StartPos:  match[0],
			EndPos:    match[1],
		}
		if agentType, ok := p.subAgentAliases[alias]; ok {
			mention.Type = MentionSubAgent
			mention.Name = agentType.String()
			mentions = append(mentions, mention)
		} else if skillName, ok := p.skillAliases[alias]; ok {
			if p.enabledSkills[skillName] {
				mention.Type = MentionSkill
				mention.Name = skillName
				mentions = append(mentions, mention)
			}
		}
	}
	return mentions
}

// StripMentions 移除输入中的提及标记
func (p *MentionParser) StripMentions(input string) string {
	mentions := p.Parse(input)
	if len(mentions) == 0 {
		return input
	}
	var result strings.Builder
	lastPos := 0
	for _, m := range mentions {
		result.WriteString(input[lastPos:m.StartPos])
		lastPos = m.EndPos
	}
	result.WriteString(input[lastPos:])
	return strings.TrimSpace(result.String())
}

// ExtractQuery 提取清理后的查询文本
func (p *MentionParser) ExtractQuery(input string) string {
	return p.StripMentions(input)
}

// GetPrimaryMention 获取主要的提及（第一个 @mention）
func (p *MentionParser) GetPrimaryMention(input string) *Mention {
	mentions := p.Parse(input)
	if len(mentions) == 0 {
		return nil
	}
	return &mentions[0]
}

// HasMention 检查输入中是否包含提及
func (p *MentionParser) HasMention(input string) bool {
	return p.mentionPattern.MatchString(input)
}

// MentionToSubAgentType 将提及别名转换为子代理类型
func (p *MentionParser) MentionToSubAgentType(alias string) (SubAgentType, bool) {
	agentType, ok := p.subAgentAliases[alias]
	return agentType, ok
}

// FormatMention 格式化提及为可读字符串
func (m *Mention) FormatMention() string {
	if m.Arguments != "" {
		return fmt.Sprintf("@%s: %s", m.Alias, m.Arguments)
	}
	return fmt.Sprintf("@%s", m.Alias)
}

// SubAgentTypeFromAlias 从别名获取子代理类型
func SubAgentTypeFromAlias(alias string) (SubAgentType, bool) {
	switch strings.ToLower(alias) {
	case "explore":
		return SubAgentTypeExplore, true
	case "review":
		return SubAgentTypeReviewer, true
	case "test":
		return SubAgentTypeTester, true
	case "security":
		return SubAgentTypeSecurity, true
	case "architect":
		return SubAgentTypeArchitect, true
	case "planner":
		return SubAgentTypePlanner, true
	case "dev":
		return SubAgentTypeSeniorDev, true
	default:
		return 0, false
	}
}

// ParseMentionCommand 解析提及命令
// 返回：子代理类型、清理后的查询、是否是提及命令
func ParseMentionCommand(input string) (SubAgentType, string, bool) {
	parser := NewMentionParser()
	mention := parser.GetPrimaryMention(input)
	if mention == nil || mention.Type != MentionSubAgent {
		return 0, input, false
	}
	agentType, ok := parser.MentionToSubAgentType(mention.Alias)
	if !ok {
		return 0, input, false
	}
	query := parser.ExtractQuery(input)
	slog.Debug("mention.parse",
		"component", utils.ComponentAgent,
		"agent_type", agentType.String(),
		"alias", mention.Alias,
		"query", query,
	)
	return agentType, query, true
}

// DetectMention 检测输入是否为提及命令
func DetectMention(input string) bool {
	parser := NewMentionParser()
	return parser.HasMention(input)
}

// ListAvailableMentions 列出所有可用的提及
func ListAvailableMentions() []string {
	parser := NewMentionParser()
	var mentions []string
	for alias := range parser.subAgentAliases {
		mentions = append(mentions, "@"+alias)
	}
	return mentions
}
