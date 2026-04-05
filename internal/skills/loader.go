package skills

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dreamSailing/vb-coding/internal/hooks"
	pluginpkg "github.com/dreamSailing/vb-coding/internal/pkg/plugins"
	"github.com/dreamSailing/vb-coding/internal/pkg/utils"

	"gopkg.in/yaml.v3"
)

type AllowedToolsField []string

func (f *AllowedToolsField) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	var out []string
	switch value.Kind {
	case yaml.ScalarNode:
		out = parseAllowedToolsString(value.Value)
	case yaml.SequenceNode:
		for _, item := range value.Content {
			if item == nil {
				continue
			}
			out = append(out, parseAllowedToolsString(item.Value)...)
		}
	}
	*f = AllowedToolsField(uniqueStrings(out))
	return nil
}

func (f AllowedToolsField) Values() []string {
	return append([]string(nil), f...)
}

// Skill 表示一个 Agent Skill
type Skill struct {
	// Frontmatter 字段
	Name                   string                          `yaml:"name"`
	Description            string                          `yaml:"description"`
	License                string                          `yaml:"license,omitempty"`
	AllowedTools           AllowedToolsField               `yaml:"allowed-tools,omitempty"`
	Model                  string                          `yaml:"model,omitempty"`
	Version                string                          `yaml:"version,omitempty"`
	ArgumentHint           string                          `yaml:"argument-hint,omitempty"`
	DisableModelInvocation bool                            `yaml:"disable-model-invocation,omitempty"`
	UserInvocable          *bool                           `yaml:"user-invocable,omitempty"`
	Context                string                          `yaml:"context,omitempty"`
	Agent                  string                          `yaml:"agent,omitempty"`
	Keywords               []string                        `yaml:"keywords,omitempty"`
	Hooks                  map[string][]hooks.MatcherGroup `yaml:"hooks,omitempty"`

	// Content 字段
	Content string // SKILL.md 的 Markdown 内容

	// 路径字段
	BaseDir       string // Skill 目录路径
	ScriptsDir    string // scripts/ 路径
	ReferencesDir string // references/ 路径
	AssetsDir     string // assets/ 路径
	SkillMdPath   string // SKILL.md 文件路径
	PluginName    string // 所属插件名称
	PluginRoot    string // 所属插件根目录
	Kind          string // skill/command

	// 运行时字段
	IsActive bool // 是否当前激活

	Location string // project/user
}

// ContextModifier 执行上下文修改
type ContextModifier struct {
	AllowedTools  []string
	ModelOverride string
}

// Loader 管理 Agent Skills 的加载和扫描
type Loader struct {
	skillsDirs []string // 扫描目录列表
	skills     map[string]*Skill
	mu         sync.RWMutex
}

// NewLoader 创建 Skills 加载器
func NewLoader() *Loader {
	return &Loader{
		skills: make(map[string]*Skill),
	}
}

// SetSkillsDirs 设置要扫描的目录
func (l *Loader) SetSkillsDirs(dirs []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.skillsDirs = dirs
	slog.Debug("skills.loader.set_dirs",
		"component", utils.ComponentSystem,
		"dirs", dirs,
	)
}

// GetSkillsDirs 获取要扫描的目录
func (l *Loader) GetSkillsDirs() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.skillsDirs
}

// Scan 扫描所有 skills 目录
func (l *Loader) Scan() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, dir := range l.skillsDirs {
		if err := l.scanDir(dir); err != nil {
			slog.Warn("skills.loader.scan_dir.error",
				"component", utils.ComponentSystem,
				"dir", dir,
				"error", err,
			)
		}
	}

	slog.Info("skills.loader.scan.complete",
		"component", utils.ComponentSystem,
		"total_skills", len(l.skills),
	)

	return nil
}

// scanDir 扫描单个目录
func (l *Loader) scanDir(root string) error {
	if isCommandsRoot(root) {
		return l.scanCommandRoot(root)
	}
	return l.scanSkillRoot(root)
}

func (l *Loader) scanSkillRoot(root string) error {
	// 检查目录是否存在
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", root)
	}

	location := inferRootLocation(root)

	ignoreDirs := map[string]bool{
		".git":         true,
		"node_modules": true,
		"dist":         true,
		"build":        true,
		"vendor":       true,
		".vb":          true,
		".trae":        true,
	}
	maxDepth := 8
	maxSkills := 500
	rootClean := filepath.Clean(root)
	rootSepCount := strings.Count(rootClean, string(os.PathSeparator))
	loaded := 0

	err = filepath.WalkDir(rootClean, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") && d.Name() != ".claude" && d.Name() != ".vb" && d.Name() != ".trae" {
			return filepath.SkipDir
		}
		if ignoreDirs[strings.ToLower(d.Name())] && filepath.Clean(path) != rootClean {
			return filepath.SkipDir
		}

		depth := strings.Count(filepath.Clean(path), string(os.PathSeparator)) - rootSepCount
		if depth > maxDepth {
			return filepath.SkipDir
		}
		if loaded >= maxSkills {
			return filepath.SkipDir
		}

		skillMdPath := l.findSkillMd(path)
		if skillMdPath == "" {
			return nil
		}

		skill, err := l.loadSkill(path, skillMdPath)
		if err != nil {
			slog.Warn("skills.loader.load_skill.error",
				"component", utils.ComponentSystem,
				"path", path,
				"error", err,
			)
			return filepath.SkipDir
		}

		skill.Location = location
		if existing, ok := l.skills[skill.Name]; ok && existing != nil && !shouldReplaceSkill(existing, skill) {
			return filepath.SkipDir
		}
		l.skills[skill.Name] = skill
		loaded++
		slog.Debug("skills.loader.loaded",
			"component", utils.ComponentSystem,
			"name", skill.Name,
			"path", path,
		)
		return filepath.SkipDir
	})
	if err != nil {
		return err
	}

	return nil
}

func (l *Loader) scanCommandRoot(root string) error {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", root)
	}

	location := inferRootLocation(root)
	ignoreDirs := map[string]bool{
		".git":         true,
		"node_modules": true,
		"dist":         true,
		"build":        true,
		"vendor":       true,
	}
	maxDepth := 8
	maxCommands := 500
	rootClean := filepath.Clean(root)
	rootSepCount := strings.Count(rootClean, string(os.PathSeparator))
	loaded := 0

	err = filepath.WalkDir(rootClean, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && filepath.Clean(path) != rootClean {
				return filepath.SkipDir
			}
			if ignoreDirs[strings.ToLower(d.Name())] && filepath.Clean(path) != rootClean {
				return filepath.SkipDir
			}
			depth := strings.Count(filepath.Clean(path), string(os.PathSeparator)) - rootSepCount
			if depth > maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if loaded >= maxCommands {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".md") || strings.EqualFold(d.Name(), "SKILL.md") {
			return nil
		}

		commandName := strings.TrimSpace(strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())))
		skill, err := l.loadSkillDocument(filepath.Dir(path), path, commandName, "command")
		if err != nil {
			slog.Warn("skills.loader.load_command.error",
				"component", utils.ComponentSystem,
				"path", path,
				"error", err,
			)
			return nil
		}
		skill.Location = location
		if existing, ok := l.skills[skill.Name]; ok && existing != nil && !shouldReplaceSkill(existing, skill) {
			return nil
		}
		l.skills[skill.Name] = skill
		loaded++
		slog.Debug("skills.loader.command.loaded",
			"component", utils.ComponentSystem,
			"name", skill.Name,
			"path", path,
		)
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func locationPriority(loc string) int {
	switch strings.ToLower(strings.TrimSpace(loc)) {
	case "user":
		return 2
	case "project":
		return 1
	default:
		return 0
	}
}

func isUserSkillsDir(root, home string) bool {
	r := strings.ToLower(filepath.Clean(root))
	h := strings.ToLower(filepath.Clean(home))
	if !strings.HasPrefix(r, h) {
		return false
	}
	roots := []string{
		filepath.Join(home, ".vb", "skills"),
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".trae", "skills"),
	}
	for _, x := range roots {
		x = strings.ToLower(filepath.Clean(x))
		if strings.HasPrefix(r, x) {
			return true
		}
	}
	return false
}

func inferRootLocation(root string) string {
	location := "project"
	if home, err := os.UserHomeDir(); err == nil {
		root = filepath.Clean(root)
		home = filepath.Clean(home)
		if isUserSkillsDir(root, home) || isUserCommandsDir(root, home) {
			location = "user"
		}
	}
	return location
}

func isUserCommandsDir(root, home string) bool {
	r := strings.ToLower(filepath.Clean(root))
	h := strings.ToLower(filepath.Clean(home))
	if !strings.HasPrefix(r, h) {
		return false
	}
	roots := []string{
		filepath.Join(home, ".vb", "commands"),
		filepath.Join(home, ".claude", "commands"),
		filepath.Join(home, ".trae", "commands"),
	}
	for _, x := range roots {
		x = strings.ToLower(filepath.Clean(x))
		if strings.HasPrefix(r, x) {
			return true
		}
	}
	return false
}

func isCommandsRoot(root string) bool {
	return strings.EqualFold(strings.TrimSpace(filepath.Base(filepath.Clean(root))), "commands")
}

// findSkillMd 查找 SKILL.md 文件（不区分大小写）
func (l *Loader) findSkillMd(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), "SKILL.md") {
			return filepath.Join(dir, entry.Name())
		}
	}
	return ""
}

// loadSkill 加载单个 skill
func (l *Loader) loadSkill(baseDir, skillMdPath string) (*Skill, error) {
	return l.loadSkillDocument(baseDir, skillMdPath, filepath.Base(baseDir), "skill")
}

func (l *Loader) loadSkillDocument(baseDir, skillMdPath, fallbackName, kind string) (*Skill, error) {
	content, err := os.ReadFile(skillMdPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SKILL.md: %w", err)
	}

	body, frontmatter := splitFrontmatterContent(string(content))
	var skill Skill
	if frontmatter != "" {
		if err := yaml.Unmarshal([]byte(frontmatter), &skill); err != nil {
			return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
		}
	}
	skill.Kind = strings.TrimSpace(kind)
	skill.Content = strings.TrimSpace(body)
	skill.BaseDir = baseDir
	skill.SkillMdPath = skillMdPath
	skill.ScriptsDir = filepath.Join(baseDir, "scripts")
	skill.ReferencesDir = filepath.Join(baseDir, "references")
	skill.AssetsDir = filepath.Join(baseDir, "assets")
	if strings.TrimSpace(skill.Name) == "" {
		skill.Name = strings.TrimSpace(fallbackName)
	}
	if strings.TrimSpace(skill.Description) == "" {
		skill.Description = firstParagraph(skill.Content)
	}

	if plugin, ok := pluginpkg.FindOwningManifest(baseDir); ok {
		skill.PluginName = strings.TrimSpace(plugin.Name)
		skill.PluginRoot = strings.TrimSpace(plugin.RootDir)
		if name := strings.TrimSpace(skill.Name); name != "" && skill.PluginName != "" && !strings.Contains(name, ":") {
			skill.Name = skill.PluginName + ":" + name
		}
		if strings.TrimSpace(skill.Location) == "" {
			skill.Location = strings.TrimSpace(plugin.Location)
		}
	}

	if strings.TrimSpace(skill.Name) == "" {
		return nil, fmt.Errorf("missing skill name")
	}
	if strings.TrimSpace(skill.Content) == "" {
		return nil, fmt.Errorf("missing skill content")
	}

	return &skill, nil
}

func splitFrontmatterContent(raw string) (body string, frontmatter string) {
	text := strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return strings.TrimSpace(text), ""
	}
	rest := text[len("---\n"):]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return strings.TrimSpace(text), ""
	}
	frontmatter = strings.TrimSpace(rest[:idx])
	body = rest[idx+len("\n---"):]
	body = strings.TrimPrefix(body, "\n")
	return strings.TrimSpace(body), frontmatter
}

func firstParagraph(content string) string {
	content = strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	if content == "" {
		return ""
	}
	for _, block := range strings.Split(content, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		return strings.Join(strings.Fields(block), " ")
	}
	return ""
}

func shouldReplaceSkill(existing, incoming *Skill) bool {
	if existing == nil {
		return true
	}
	if incoming == nil {
		return false
	}
	existingLoc := locationPriority(existing.Location)
	incomingLoc := locationPriority(incoming.Location)
	if incomingLoc != existingLoc {
		return incomingLoc > existingLoc
	}
	return skillKindPriority(incoming.Kind) > skillKindPriority(existing.Kind)
}

func skillKindPriority(kind string) int {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "skill":
		return 2
	case "command":
		return 1
	default:
		return 0
	}
}

// List 列出所有可用 skills
func (l *Loader) List() []*Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	list := make([]*Skill, 0, len(l.skills))
	for _, skill := range l.skills {
		list = append(list, skill)
	}
	return list
}

// Get 获取指定 skill
func (l *Loader) Get(name string) (*Skill, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	skill, ok := l.skills[name]
	return skill, ok
}

// GetNames 获取所有 skill 名称
func (l *Loader) GetNames() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	names := make([]string, 0, len(l.skills))
	for name := range l.skills {
		names = append(names, name)
	}
	return names
}

// Reload 重新扫描所有目录
func (l *Loader) Reload() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 清空现有 skills
	l.skills = make(map[string]*Skill)

	// 重新扫描
	for _, dir := range l.skillsDirs {
		if err := l.scanDir(dir); err != nil {
			slog.Warn("skills.loader.scan_dir.error",
				"component", utils.ComponentSystem,
				"dir", dir,
				"error", err,
			)
		}
	}

	slog.Info("skills.loader.reload.complete",
		"component", utils.ComponentSystem,
		"total_skills", len(l.skills),
	)

	return nil
}

// GetStats 获取统计信息
func (l *Loader) GetStats() map[string]any {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return map[string]any{
		"total_skills": len(l.skills),
		"scan_dirs":    l.skillsDirs,
		"names":        l.GetNames(),
	}
}

// ParseAllowedTools 解析 allowed-tools 字符串
func ParseAllowedTools(toolsStr string) []string {
	return parseAllowedToolsString(toolsStr)
}

// FormatForTool 格式化 skill 用于 Skill tool 的描述
func (s *Skill) FormatForTool() string {
	return fmt.Sprintf(`"%s": %s`, s.Name, s.Description)
}

// GetContextModifier 获取上下文修改器
func (s *Skill) GetContextModifier() *ContextModifier {
	return &ContextModifier{
		AllowedTools:  s.AllowedTools.Values(),
		ModelOverride: s.Model,
	}
}

// GetPrompt 获取完整的 prompt 内容
func (s *Skill) GetPrompt() string {
	// 替换 {baseDir} 占位符
	prompt := strings.ReplaceAll(s.Content, "{baseDir}", s.BaseDir)
	return prompt
}

func (s *Skill) RenderPrompt(arguments string) string {
	prompt := s.GetPrompt()
	arguments = strings.TrimSpace(arguments)
	if arguments == "" {
		return prompt
	}
	if strings.Contains(prompt, "$ARGUMENTS") {
		prompt = strings.ReplaceAll(prompt, "$ARGUMENTS", arguments)
	} else {
		prompt = strings.TrimRight(prompt, "\n") + "\n\nARGUMENTS: " + arguments
	}
	return prompt
}

func parseAllowedToolsString(toolsStr string) []string {
	parts := strings.FieldsFunc(strings.TrimSpace(toolsStr), func(r rune) bool {
		switch r {
		case ',', '\n', '\r', '\t', ' ':
			return true
		default:
			return false
		}
	})
	return uniqueStrings(parts)
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}
