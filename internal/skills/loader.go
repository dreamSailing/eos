package skills

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"io/fs"

	"github.com/dreamSailing/vb-coding/internal/hooks"
	"github.com/dreamSailing/vb-coding/internal/pkg/utils"

	"gopkg.in/yaml.v3"
)

// Skill 表示一个 Agent Skill
type Skill struct {
	// Frontmatter 字段
	Name         string `yaml:"name"`
	Description  string `yaml:"description"`
	License      string `yaml:"license,omitempty"`
	AllowedTools string `yaml:"allowed-tools,omitempty"`
	Model        string `yaml:"model,omitempty"`
	Version      string `yaml:"version,omitempty"`
	ArgumentHint          string   `yaml:"argument-hint,omitempty"`
	DisableModelInvocation bool     `yaml:"disable-model-invocation,omitempty"`
	UserInvocable          *bool    `yaml:"user-invocable,omitempty"`
	Context                string   `yaml:"context,omitempty"`
	Agent                  string   `yaml:"agent,omitempty"`
	Keywords               []string `yaml:"keywords,omitempty"`
	Hooks                  map[string][]hooks.MatcherGroup `yaml:"hooks,omitempty"`

	// Content 字段
	Content string // SKILL.md 的 Markdown 内容

	// 路径字段
	BaseDir       string // Skill 目录路径
	ScriptsDir    string // scripts/ 路径
	ReferencesDir string // references/ 路径
	AssetsDir     string // assets/ 路径
	SkillMdPath   string // SKILL.md 文件路径

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

	location := "project"
	if home, err := os.UserHomeDir(); err == nil {
		if isUserSkillsDir(filepath.Clean(root), filepath.Clean(home)) {
			location = "user"
		}
	}

	ignoreDirs := map[string]bool{
		".git":        true,
		"node_modules": true,
		"dist":        true,
		"build":       true,
		"vendor":      true,
		".vb":         true,
		".trae":       true,
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
		if existing, ok := l.skills[skill.Name]; ok && existing != nil {
			if locationPriority(existing.Location) >= locationPriority(skill.Location) {
				return filepath.SkipDir
			}
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
	content, err := os.ReadFile(skillMdPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SKILL.md: %w", err)
	}

	// 分离 frontmatter 和 markdown
	// 格式：---\nYAML\n---\nMarkdown
	parts := strings.SplitN(string(content), "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid SKILL.md format: missing frontmatter or content")
	}

	// 解析 YAML frontmatter
	var skill Skill
	if err := yaml.Unmarshal([]byte(parts[1]), &skill); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// 验证必需字段
	if skill.Name == "" {
		return nil, fmt.Errorf("missing required field: name")
	}
	if skill.Description == "" {
		return nil, fmt.Errorf("missing required field: description")
	}

	// 提取 markdown 内容
	skill.Content = strings.TrimSpace(parts[2])
	skill.BaseDir = baseDir
	skill.SkillMdPath = skillMdPath
	skill.ScriptsDir = filepath.Join(baseDir, "scripts")
	skill.ReferencesDir = filepath.Join(baseDir, "references")
	skill.AssetsDir = filepath.Join(baseDir, "assets")

	return &skill, nil
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
	if toolsStr == "" {
		return nil
	}

	tools := strings.Split(toolsStr, ",")
	result := make([]string, 0, len(tools))

	for _, tool := range tools {
		tool = strings.TrimSpace(tool)
		if tool != "" {
			result = append(result, tool)
		}
	}

	return result
}

// FormatForTool 格式化 skill 用于 Skill tool 的描述
func (s *Skill) FormatForTool() string {
	return fmt.Sprintf(`"%s": %s`, s.Name, s.Description)
}

// GetContextModifier 获取上下文修改器
func (s *Skill) GetContextModifier() *ContextModifier {
	return &ContextModifier{
		AllowedTools:  ParseAllowedTools(s.AllowedTools),
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
