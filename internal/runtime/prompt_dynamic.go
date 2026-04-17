package runtime

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
)

func BuildProjectPromptAdditions(cwd string) string {
	var sb strings.Builder

	sb.WriteString("**项目约定**：\n")
	addSnippetPath := func(label string, p string, max int) {
		b, ok := readTextFileBestEffort(p, max)
		if !ok {
			return
		}
		sb.WriteString("- @" + label + "\n")
		sb.WriteString("```\n")
		sb.WriteString(strings.TrimSpace(b))
		sb.WriteString("\n```\n")
	}
	addSnippet := func(rel string, max int) {
		addSnippetPath(rel, filepath.Join(cwd, rel), max)
	}

	addSnippet("EOS.md", 8000)

	// Fix 4.5: Inject session memory content if available
	sessionMemPath := filepath.Join(cwd, ".eos", "session-memory", "session.md")
	if memContent, ok := readTextFileBestEffort(sessionMemPath, 4000); ok {
		sb.WriteString("\n\n**会话记忆**：\n```\n")
		sb.WriteString(strings.TrimSpace(memContent))
		sb.WriteString("\n```\n")
	}

	sb.WriteString("\n\n## 自动记忆指南\n当你在对话中发现重要的用户偏好、项目约定或反复出现的模式时，使用 suggest_memory 工具将它们建议添加到 EOS.md 或 .eos/Rules.md 中。")

	addSnippet(filepath.Join(".eos", "Rules.md"), 8000)
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		addSnippetPath(filepath.Join("~", ".eos", "Rules.md"), filepath.Join(home, ".eos", "Rules.md"), 8000)
	}
	addSnippet(filepath.Join(".eos", "prompt.md"), 4000)

	sb.WriteString("\n**规范文件约定**：\n")
	sb.WriteString("- 项目规范文件：.eos/Rules.md（默认写这里）\n")
	sb.WriteString("- 全局规范文件：~/.eos/Rules.md（仅用户明确要求“全局规则”时写这里）\n")
	sb.WriteString("- 项目指导文件只使用 VB.md，不使用 CLAUDE.md\n")
	sb.WriteString("- 用户要求“写/更新规则”时，直接更新对应 Rules.md 文件内容\n")
	sb.WriteString("- 生成/更新规范时使用固定模板；若文件已存在，更新其对应章节，不要整文件重写或重复追加标题\n")
	sb.WriteString("```\n")
	sb.WriteString(strings.TrimSpace(RulesMdTemplate()))
	sb.WriteString("\n```\n")

	if recent := recentVersionedFiles(cwd, 8); recent != "" {
		sb.WriteString("\n**最近修改（基于 .eos/versions）**：\n")
		sb.WriteString(recent)
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}

func buildIntentPromptAdditions(history []*schema.Message) string {
	lastUser := lastUserText(history)
	if strings.TrimSpace(lastUser) == "" {
		return ""
	}
	intent := detectIntent(lastUser)

	var sb strings.Builder
	sb.WriteString("**本轮意图**：")
	sb.WriteString(intent)
	sb.WriteString("\n")
	switch intent {
	case "调试/修复":
		sb.WriteString("- 优先读取报错与相关文件，定位根因后再改代码\n")
	case "编码/实现":
		sb.WriteString("- 先了解现状与约束，再进行最小可验证改动\n")
	case "重构/优化":
		sb.WriteString("- 先扫描结构与依赖，再按小步提交式修改并验证\n")
	default:
		sb.WriteString("- 直接给出简洁答案；必要时再调用工具补充证据\n")
	}
	return strings.TrimSpace(sb.String())
}

// detectIntent 基于多维度评分检测用户意图
func detectIntent(text string) string {
	s := strings.ToLower(text)

	// 多维度评分
	scores := map[string]int{
		"调试/修复": 0,
		"编码/实现": 0,
		"重构/优化": 0,
		"问答/解释": 0,
		"分析/审查": 0,
	}

	// 调试关键词
	for _, k := range []string{"bug", "error", "panic", "崩溃", "报错", "修复", "不工作", "失败", "异常", "crash"} {
		if strings.Contains(s, k) {
			scores["调试/修复"]++
		}
	}
	// 实现关键词
	for _, k := range []string{"实现", "新增", "添加", "支持", "功能", "创建", "写", "做"} {
		if strings.Contains(s, k) {
			scores["编码/实现"]++
		}
	}
	// 重构关键词
	for _, k := range []string{"重构", "优化", "性能", "整理", "拆分", "迭代", "改进", "简化"} {
		if strings.Contains(s, k) {
			scores["重构/优化"]++
		}
	}
	// 问答关键词
	for _, k := range []string{"解释", "为什么", "怎么", "原理", "什么是", "区别", "如何"} {
		if strings.Contains(s, k) {
			scores["问答/解释"]++
		}
	}
	// 分析关键词
	for _, k := range []string{"看看", "分析", "审查", "检查", "评估", "差距", "对比", "质量"} {
		if strings.Contains(s, k) {
			scores["分析/审查"]++
		}
	}

	// 返回得分最高的意图
	best, bestScore := "通用", 0
	for intent, score := range scores {
		if score > bestScore {
			best, bestScore = intent, score
		}
	}
	return best
}

func lastUserText(history []*schema.Message) string {
	for i := len(history) - 1; i >= 0; i-- {
		m := history[i]
		if m == nil {
			continue
		}
		if m.Role != schema.User {
			continue
		}
		if strings.TrimSpace(m.Content) != "" {
			return m.Content
		}
		if len(m.UserInputMultiContent) > 0 {
			for _, p := range m.UserInputMultiContent {
				if p.Type == schema.ChatMessagePartTypeText && strings.TrimSpace(p.Text) != "" {
					return p.Text
				}
			}
		}
	}
	return ""
}

func readTextFileBestEffort(path string, max int) (string, bool) {
	if max <= 0 {
		max = 2000
	}
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return "", false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", false
	}
	if len(s) > max {
		s = s[:max] + "\n…trimmed"
	}
	return s, true
}

func recentVersionedFiles(cwd string, limit int) string {
	if limit <= 0 {
		limit = 8
	}
	root := filepath.Join(cwd, ".eos", "versions")
	_, err := os.Stat(root)
	if err != nil {
		return ""
	}

	type rec struct {
		pathRel string
		ts      time.Time
	}
	latest := map[string]time.Time{}

	seen := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		seen++
		if seen > 3000 {
			return fs.SkipAll
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".meta") && !strings.HasSuffix(name, ".content") {
			return nil
		}
		id := strings.TrimSuffix(strings.TrimSuffix(name, ".meta"), ".content")
		ts, err := time.Parse("20060102-150405", id)
		if err != nil {
			return nil
		}
		dir := filepath.Dir(path)
		relDir, err := filepath.Rel(root, dir)
		if err != nil || strings.HasPrefix(relDir, "..") {
			return nil
		}
		existing, ok := latest[relDir]
		if !ok || ts.After(existing) {
			latest[relDir] = ts
		}
		return nil
	})

	if len(latest) == 0 {
		return ""
	}
	rs := make([]rec, 0, len(latest))
	for p, ts := range latest {
		rs = append(rs, rec{pathRel: filepath.ToSlash(p), ts: ts})
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i].ts.After(rs[j].ts) })
	if len(rs) > limit {
		rs = rs[:limit]
	}
	var sb strings.Builder
	for _, r := range rs {
		sb.WriteString("- ")
		sb.WriteString(r.pathRel)
		sb.WriteString(" (")
		sb.WriteString(r.ts.Format("2006-01-02 15:04:05"))
		sb.WriteString(")\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}
