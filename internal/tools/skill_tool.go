package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
	"github.com/dreamSailing/vb-coding/internal/skills"
)

// SkillManager 管理 Agent Skills
type SkillManager struct {
	loader     *skills.Loader
	manager    *Manager
	mu         sync.RWMutex
	active     map[string]*skills.Skill // 当前激活的 skills
	disabled   map[string]bool          // 被禁用的 skills
	onActivate func(*skills.Skill)      // skill 激活回调
}

// NewSkillManager 创建 Skill 管理器
func NewSkillManager(loader *skills.Loader, mgr *Manager) *SkillManager {
	return &SkillManager{
		loader:   loader,
		manager:  mgr,
		active:   make(map[string]*skills.Skill),
		disabled: make(map[string]bool),
	}
}

// SetOnActivate 设置 skill 激活回调
func (sm *SkillManager) SetOnActivate(fn func(*skills.Skill)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onActivate = fn
}

// GetLoader 获取 skills 加载器
func (sm *SkillManager) GetLoader() *skills.Loader {
	return sm.loader
}

// List 列出所有可用 skills
func (sm *SkillManager) List() []*skills.Skill {
	if sm.loader == nil {
		return nil
	}
	return sm.loader.List()
}

// Get 获取指定 skill
func (sm *SkillManager) Get(name string) (*skills.Skill, bool) {
	if sm.loader == nil {
		return nil, false
	}
	if sm.IsDisabled(name) {
		return nil, false
	}
	return sm.loader.Get(name)
}

// InjectSkill 注入 skill 到对话上下文
// 返回：消息列表、上下文修改器、错误
func (sm *SkillManager) InjectSkill(ctx context.Context, skillName string) ([]MessagePart, *skills.ContextModifier, error) {
	return sm.InjectSkillWithArguments(ctx, skillName, "")
}

func (sm *SkillManager) InjectSkillWithArguments(ctx context.Context, skillName string, arguments string) ([]MessagePart, *skills.ContextModifier, error) {
	if sm.loader == nil {
		return nil, nil, fmt.Errorf("skill loader not initialized")
	}

	// 加载 skill
	skill, ok := sm.Get(skillName)
	if !ok {
		if sm.IsDisabled(skillName) {
			return nil, nil, fmt.Errorf("skill disabled: %s", skillName)
		}
		return nil, nil, fmt.Errorf("skill not found: %s", skillName)
	}

	slog.Debug("skills.manager.inject",
		"component", utils.ComponentTool,
		"skill", skillName,
		"description", skill.Description,
	)

	// 构建消息
	messages, err := sm.buildSkillMessages(ctx, skill, arguments)
	if err != nil {
		return nil, nil, err
	}
	ctxMod := skill.GetContextModifier()

	// 标记为激活
	sm.mu.Lock()
	sm.active[skillName] = skill
	sm.mu.Unlock()

	// 触发回调
	if sm.onActivate != nil {
		sm.onActivate(skill)
	}

	return messages, ctxMod, nil
}

// MessagePart 表示消息的一部分（用于返回给上层）
type MessagePart struct {
	Role    string
	Content string
	IsMeta  bool
}

// buildSkillMessages 构建技能相关的消息
func (sm *SkillManager) buildSkillMessages(ctx context.Context, skill *skills.Skill, arguments string) ([]MessagePart, error) {
	// 消息 1：用户可见的元数据
	metadataContent := fmt.Sprintf(
		"<command-message>The \"%s\" skill is loading</command-message>\n<command-name>%s</command-name>",
		skill.Name, skill.Name,
	)

	// 消息 2：隐藏的 skill prompt
	skillPrompt := skill.RenderPrompt(arguments)
	rendered, err := sm.preprocessBangCommands(ctx, skillPrompt)
	if err != nil {
		return nil, err
	}

	return []MessagePart{
		{
			Role:    "user",
			Content: metadataContent,
			IsMeta:  false, // 用户可见
		},
		{
			Role:    "user",
			Content: rendered,
			IsMeta:  true, // 用户不可见
		},
	}, nil
}

var bangCmdRe = regexp.MustCompile("!`([\\s\\S]*?)`")

func (sm *SkillManager) preprocessBangCommands(ctx context.Context, s string) (string, error) {
	if !strings.Contains(s, "!`") {
		return s, nil
	}
	if sm == nil || sm.manager == nil {
		return s, nil
	}
	matches := bangCmdRe.FindAllStringSubmatchIndex(s, -1)
	if len(matches) == 0 {
		return s, nil
	}
	type rep struct {
		start int
		end   int
		text  string
	}
	reps := make([]rep, 0, len(matches))
	for _, m := range matches {
		if len(m) < 4 {
			continue
		}
		fullStart, fullEnd := m[0], m[1]
		cmdStart, cmdEnd := m[2], m[3]
		cmd := strings.TrimSpace(s[cmdStart:cmdEnd])
		if cmd == "" {
			reps = append(reps, rep{start: fullStart, end: fullEnd, text: ""})
			continue
		}

		call := ToolCall{Tool: "bash", Parameters: map[string]any{"command": cmd}}
		if SafetyGateClassify != nil {
			category, _, summary, dangerous := SafetyGateClassify(call)
			if dangerous && (SafetyGateSessionAllowed == nil || !SafetyGateSessionAllowed(category)) {
				if SafetyGatePrompt == nil {
					return "", fmt.Errorf("permission required for: %s", summary)
				}
				dec := SafetyGatePrompt(ctx, category, summary)
				if dec == "deny" {
					return "", fmt.Errorf("operation denied by user: %s", summary)
				}
				if dec == "session" && SafetyGateAllowSession != nil {
					SafetyGateAllowSession(category)
				}
			}
		}

		res, err := sm.manager.ExecuteBashDirect(ctx, cmd)
		if err != nil {
			reps = append(reps, rep{start: fullStart, end: fullEnd, text: "Error: " + err.Error()})
			continue
		}
		res = strings.TrimSpace(res)
		if len(res) > 4000 {
			res = TruncateOutput(res, 4000)
		}
		reps = append(reps, rep{start: fullStart, end: fullEnd, text: res})
	}

	out := s
	for i := len(reps) - 1; i >= 0; i-- {
		r := reps[i]
		if r.start < 0 || r.end < 0 || r.start > len(out) || r.end > len(out) || r.start > r.end {
			continue
		}
		out = out[:r.start] + r.text + out[r.end:]
	}
	return out, nil
}

// Deactivate 停用 skill
func (sm *SkillManager) Deactivate(skillName string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.active, skillName)

	slog.Debug("skills.manager.deactivate",
		"component", utils.ComponentTool,
		"skill", skillName,
	)
}

// GetActive 获取当前激活的 skills
func (sm *SkillManager) GetActive() map[string]*skills.Skill {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]*skills.Skill, len(sm.active))
	for k, v := range sm.active {
		result[k] = v
	}
	return result
}

// IsActive 检查 skill 是否激活
func (sm *SkillManager) IsActive(skillName string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	_, ok := sm.active[skillName]
	return ok
}

// SetDisabled 设置 skill 禁用状态。被禁用后会自动取消激活。
func (sm *SkillManager) SetDisabled(skillName string, disabled bool) {
	key := skillStateKey(skillName)
	if key == "" {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if disabled {
		sm.disabled[key] = true
		for name := range sm.active {
			if skillStateKey(name) == key {
				delete(sm.active, name)
			}
		}
		return
	}
	delete(sm.disabled, key)
}

// SetDisabledSkills 使用新的禁用列表覆盖当前状态。
func (sm *SkillManager) SetDisabledSkills(skillNames []string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	disabled := make(map[string]bool, len(skillNames))
	for _, name := range skillNames {
		key := skillStateKey(name)
		if key == "" {
			continue
		}
		disabled[key] = true
	}
	sm.disabled = disabled
	for name := range sm.active {
		if disabled[skillStateKey(name)] {
			delete(sm.active, name)
		}
	}
}

// IsDisabled 检查 skill 是否被禁用。
func (sm *SkillManager) IsDisabled(skillName string) bool {
	key := skillStateKey(skillName)
	if key == "" {
		return false
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.disabled[key]
}

// GetDisabled 返回当前禁用的 skill 名称集合。
func (sm *SkillManager) GetDisabled() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	names := make([]string, 0, len(sm.disabled))
	for name := range sm.disabled {
		names = append(names, name)
	}
	return names
}

// FormatSkillsForPrompt 格式化 skills 列表用于提示词
// 这用于在 Skill tool 的描述中展示可用 skills
func (sm *SkillManager) FormatSkillsForPrompt() string {
	if sm.loader == nil {
		return ""
	}

	skills := sm.loader.List()
	if len(skills) == 0 {
		return ""
	}

	maxChars := 16000
	if v := strings.TrimSpace(os.Getenv("SLASH_COMMAND_TOOL_CHAR_BUDGET")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxChars = n
		}
	}

	var b strings.Builder
	b.WriteString("<available_skills>\n")
	total := 0
	omitted := 0
	for _, s := range skills {
		if s == nil {
			continue
		}
		if s.DisableModelInvocation {
			continue
		}
		if sm.IsDisabled(s.Name) {
			continue
		}
		name := strings.TrimSpace(s.Name)
		desc := strings.TrimSpace(s.Description)
		if name == "" || desc == "" {
			continue
		}
		loc := strings.TrimSpace(s.Location)
		if loc == "" {
			loc = "project"
		}

		ah := strings.TrimSpace(s.ArgumentHint)
		kws := make([]string, 0, len(s.Keywords))
		for _, kw := range s.Keywords {
			kw = strings.TrimSpace(kw)
			if kw != "" {
				kws = append(kws, kw)
			}
		}

		var item strings.Builder
		item.WriteString("  <skill>\n")
		item.WriteString("    <name>" + name + "</name>\n")
		item.WriteString("    <description>" + desc + "</description>\n")
		if ah != "" {
			item.WriteString("    <argument-hint>" + ah + "</argument-hint>\n")
		}
		if len(kws) > 0 {
			item.WriteString("    <keywords>" + strings.Join(kws, ", ") + "</keywords>\n")
		}
		item.WriteString("    <location>" + loc + "</location>\n")
		item.WriteString("  </skill>\n")
		itemStr := item.String()
		if total+len(itemStr) > maxChars {
			omitted++
			continue
		}
		b.WriteString(itemStr)
		total += len(itemStr)
	}
	if omitted > 0 {
		b.WriteString(fmt.Sprintf("  <note>Some skills omitted due to budget (%d). Use skills_list to view all.</note>\n", omitted))
	}
	b.WriteString("</available_skills>")
	return b.String()
}

// Reload 重新加载 skills
func (sm *SkillManager) Reload() error {
	if sm.loader == nil {
		return fmt.Errorf("skill loader not initialized")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 清除激活状态
	sm.active = make(map[string]*skills.Skill)

	// 重新加载
	return sm.loader.Reload()
}

func (sm *SkillManager) ReloadPreserveActive() error {
	if sm.loader == nil {
		return fmt.Errorf("skill loader not initialized")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	activeNames := make([]string, 0, len(sm.active))
	for name := range sm.active {
		activeNames = append(activeNames, name)
	}

	if err := sm.loader.Reload(); err != nil {
		return err
	}

	for _, name := range activeNames {
		if sm.disabled[skillStateKey(name)] {
			delete(sm.active, name)
			continue
		}
		if s, ok := sm.loader.Get(name); ok && s != nil {
			sm.active[name] = s
		} else {
			delete(sm.active, name)
		}
	}
	return nil
}

// GetStats 获取统计信息
func (sm *SkillManager) GetStats() map[string]any {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := map[string]any{
		"active_count":   len(sm.active),
		"disabled_count": len(sm.disabled),
		"active_names": func() []string {
			names := make([]string, 0, len(sm.active))
			for name := range sm.active {
				names = append(names, name)
			}
			return names
		}(),
		"disabled_names": func() []string {
			names := make([]string, 0, len(sm.disabled))
			for name := range sm.disabled {
				names = append(names, name)
			}
			return names
		}(),
	}

	if sm.loader != nil {
		loaderStats := sm.loader.GetStats()
		for k, v := range loaderStats {
			stats[k] = v
		}
	}

	return stats
}

func skillStateKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
