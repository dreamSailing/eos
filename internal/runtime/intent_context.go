package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/cloudwego/eino/schema"
	codectx "github.com/dreamSailing/eos/internal/context"
)

type IntentContextSource string

const (
	IntentContextSourceIntentRecall  IntentContextSource = "intent_recall"
	IntentContextSourceExplicitPath  IntentContextSource = "explicit_path"
	IntentContextSourceRecentChanges IntentContextSource = "recent_changes"
	IntentContextSourceRecentFocus   IntentContextSource = "recent_focus"
)

type IntentContextDecision string

const (
	IntentContextDecisionDirectAnswer   IntentContextDecision = "direct_answer"
	IntentContextDecisionPreferExplicit IntentContextDecision = "prefer_explicit"
	IntentContextDecisionAutoLocate     IntentContextDecision = "auto_locate"
	IntentContextDecisionClarify        IntentContextDecision = "clarify"
	IntentContextDecisionBroadSearch    IntentContextDecision = "broad_search"
)

type IntentContextFallback string

const (
	IntentContextFallbackNone        IntentContextFallback = "none"
	IntentContextFallbackClarify     IntentContextFallback = "clarify"
	IntentContextFallbackBroadSearch IntentContextFallback = "broad_search"
)

type IntentContextCandidate struct {
	Path     string
	Symbols  []string
	Sources  []IntentContextSource
	Score    float64
	Priority int
	Reason   string
}

type IntentContextPlan struct {
	Query              string
	Intent             string
	Decision           IntentContextDecision
	Fallback           IntentContextFallback
	Confidence         float64
	ExplicitSignals    []string
	SearchHints        []string
	Candidates         []IntentContextCandidate
	TrimmedCandidates  []IntentContextCandidate
	Explanations       []string
	InjectBudgetBytes  int
	HasExplicitSignals bool
}

type intentCandidateAccumulator struct {
	path     string
	symbols  []string
	sources  map[IntentContextSource]struct{}
	score    float64
	priority int
	reasons  []string
}

const (
	intentContextKeepCandidates = 3
	intentContextMaxCandidates  = 6
)

func buildIntentContextPlan(cwd string, history []*schema.Message) IntentContextPlan {
	query := strings.TrimSpace(lastUserText(history))
	return planIntentContext(cwd, query)
}

func planIntentContext(cwd string, query string) IntentContextPlan {
	plan := IntentContextPlan{
		Query:             strings.TrimSpace(query),
		Intent:            detectIntent(query),
		Decision:          IntentContextDecisionDirectAnswer,
		Fallback:          IntentContextFallbackNone,
		InjectBudgetBytes: 12 * 1024,
	}
	if plan.Query == "" {
		return plan
	}

	searchHints := extractCodeKeywords(plan.Query)
	explicitPaths := extractExplicitContextPaths(plan.Query)
	plan.ExplicitSignals = append([]string{}, explicitPaths...)
	plan.SearchHints = append([]string{}, searchHints...)
	plan.HasExplicitSignals = len(plan.ExplicitSignals) > 0

	accumulators := map[string]*intentCandidateAccumulator{}
	for _, path := range explicitPaths {
		addIntentCandidate(accumulators, path, searchHints, IntentContextSourceExplicitPath, 1.0, 300, "用户已显式指定路径")
	}

	for _, path := range CollectRecentFiles(cwd) {
		score, reason, matched := scoreRecentFileCandidate(path, searchHints)
		if !matched {
			continue
		}
		addIntentCandidate(accumulators, path, searchHints, IntentContextSourceRecentChanges, score, 180, reason)
	}

	for _, path := range recentVersionedFilesList(cwd, 6) {
		score, reason, matched := scoreRecentFileCandidate(path, searchHints)
		if !matched {
			continue
		}
		addIntentCandidate(accumulators, path, searchHints, IntentContextSourceRecentFocus, score, 120, reason)
	}

	recall := recallIntentContextCandidates(cwd, plan.Query, explicitPaths, searchHints)
	plan.SearchHints = uniqueStrings(append(plan.SearchHints, recall.Terms...))
	for _, candidate := range recall.Candidates {
		addIntentCandidate(accumulators, candidate.Path, candidate.Symbols, IntentContextSourceIntentRecall, candidate.Score, candidate.Priority, candidate.Reason)
		for _, source := range candidate.Sources {
			mapped := mapRecallSource(source)
			if mapped == "" || mapped == IntentContextSourceIntentRecall {
				continue
			}
			addIntentCandidate(accumulators, candidate.Path, candidate.Symbols, mapped, candidate.Score, candidate.Priority, "")
		}
	}

	if len(accumulators) == 0 && len(explicitPaths) == 0 {
		for idx, path := range CollectRecentFiles(cwd) {
			if idx >= 2 {
				break
			}
			addIntentCandidate(accumulators, path, nil, IntentContextSourceRecentChanges, 0.42, 80, "缺少显式命中，回退到最近改动文件")
		}
		for idx, path := range recentVersionedFilesList(cwd, 6) {
			if idx >= 2 {
				break
			}
			addIntentCandidate(accumulators, path, nil, IntentContextSourceRecentFocus, 0.36, 60, "缺少显式命中，回退到最近焦点文件")
		}
	}

	plan.Candidates = finalizeIntentCandidates(accumulators)
	if len(plan.Candidates) > intentContextMaxCandidates {
		plan.Candidates = plan.Candidates[:intentContextMaxCandidates]
	}

	if len(plan.Candidates) > intentContextKeepCandidates {
		plan.TrimmedCandidates = append(plan.TrimmedCandidates, plan.Candidates[intentContextKeepCandidates:]...)
		plan.Candidates = plan.Candidates[:intentContextKeepCandidates]
	}
	plan.InjectBudgetBytes = clampIntentBudget(8*1024+len(plan.Candidates)*4*1024, 8*1024, 32*1024)
	plan.Decision, plan.Fallback, plan.Confidence, plan.Explanations = decideIntentContextPlan(plan)
	return plan
}

func addIntentCandidate(acc map[string]*intentCandidateAccumulator, path string, symbols []string, source IntentContextSource, score float64, priority int, reason string) {
	path = normalizeIntentContextPath(path)
	if path == "" {
		return
	}
	item, ok := acc[path]
	if !ok {
		item = &intentCandidateAccumulator{
			path:    path,
			symbols: append([]string{}, symbols...),
			sources: map[IntentContextSource]struct{}{},
		}
		acc[path] = item
	}
	item.sources[source] = struct{}{}
	if score > item.score {
		item.score = score
	}
	if priority > item.priority {
		item.priority = priority
	}
	if reason = strings.TrimSpace(reason); reason != "" && !containsString(item.reasons, reason) {
		item.reasons = append(item.reasons, reason)
	}
	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" || containsString(item.symbols, symbol) {
			continue
		}
		item.symbols = append(item.symbols, symbol)
	}
}

func finalizeIntentCandidates(acc map[string]*intentCandidateAccumulator) []IntentContextCandidate {
	if len(acc) == 0 {
		return nil
	}
	out := make([]IntentContextCandidate, 0, len(acc))
	for _, item := range acc {
		sources := make([]IntentContextSource, 0, len(item.sources))
		for source := range item.sources {
			sources = append(sources, source)
		}
		sort.Slice(sources, func(i, j int) bool { return sources[i] < sources[j] })
		out = append(out, IntentContextCandidate{
			Path:     item.path,
			Symbols:  append([]string{}, item.symbols...),
			Sources:  sources,
			Score:    item.score,
			Priority: item.priority,
			Reason:   strings.Join(item.reasons, "；"),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func decideIntentContextPlan(plan IntentContextPlan) (IntentContextDecision, IntentContextFallback, float64, []string) {
	if plan.HasExplicitSignals {
		return IntentContextDecisionPreferExplicit, IntentContextFallbackNone, 1.0, []string{
			"显式路径优先于自动定位，自动候选仅作为补充线索。",
			fmt.Sprintf("保留 %d 个显式信号并直接纳入上下文规划。", len(plan.ExplicitSignals)),
		}
	}

	if len(plan.Candidates) == 0 {
		return IntentContextDecisionBroadSearch, IntentContextFallbackBroadSearch, 0.18, []string{
			"未找到足够可靠的候选文件。",
			"优先扩大搜索范围，而不是假设已经理解用户指向的代码区域。",
		}
	}

	top := plan.Candidates[0]
	secondScore := 0.0
	if len(plan.Candidates) > 1 {
		secondScore = plan.Candidates[1].Score
	}
	gap := top.Score - secondScore

	if top.Score >= 0.85 && gap >= 0.15 {
		return IntentContextDecisionAutoLocate, IntentContextFallbackNone, top.Score, []string{
			"最近活跃文件与当前查询高度一致，可以先注入高置信候选。",
			fmt.Sprintf("首选候选 `%s` 相比次选结果有明显优势。", top.Path),
		}
	}

	if len(plan.Candidates) > 1 && gap <= 0.08 {
		return IntentContextDecisionClarify, IntentContextFallbackClarify, clampConfidence(top.Score-0.15, 0.22, 0.55), []string{
			"前几名候选得分接近，现有证据不足以安全地区分具体目标。",
			"应优先澄清候选方向，或在更宽范围内继续搜索。",
		}
	}

	if top.Score < 0.55 {
		return IntentContextDecisionBroadSearch, IntentContextFallbackBroadSearch, clampConfidence(top.Score, 0.18, 0.5), []string{
			"现有候选分数偏低，直接修改风险较高。",
			"优先执行更广域的文本/符号搜索，再决定是否进入修改。",
		}
	}

	return IntentContextDecisionAutoLocate, IntentContextFallbackNone, clampConfidence(top.Score, 0.56, 0.84), []string{
		"已找到可用候选，但仍需在执行前结合搜索结果进一步确认。",
		"优先围绕首选候选及其符号线索展开读取和验证。",
	}
}

func formatIntentContextPlan(plan IntentContextPlan) string {
	if strings.TrimSpace(plan.Query) == "" {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("**本轮意图**：")
	sb.WriteString(plan.Intent)
	sb.WriteString("\n")
	switch plan.Intent {
	case "调试/修复":
		sb.WriteString("- 优先读取报错与相关文件，定位根因后再改代码\n")
	case "编码/实现":
		sb.WriteString("- 先了解现状与约束，再进行最小可验证改动\n")
	case "重构/优化":
		sb.WriteString("- 先扫描结构与依赖，再按小步提交式修改并验证\n")
	default:
		sb.WriteString("- 直接给出简洁答案；必要时再调用工具补充证据\n")
	}

	sb.WriteString("\n**意图感知上下文编排**：\n")
	sb.WriteString("- 决策: " + string(plan.Decision) + "\n")
	sb.WriteString("- 回退: " + string(plan.Fallback) + "\n")
	sb.WriteString(fmt.Sprintf("- 置信度: %.2f\n", plan.Confidence))
	sb.WriteString(fmt.Sprintf("- 注入预算: %d bytes\n", plan.InjectBudgetBytes))
	if len(plan.ExplicitSignals) > 0 {
		sb.WriteString("- 显式信号: " + strings.Join(plan.ExplicitSignals, ", ") + "\n")
	}
	if len(plan.SearchHints) > 0 {
		sb.WriteString("- 搜索线索: " + strings.Join(plan.SearchHints, ", ") + "\n")
	}
	for idx, cand := range plan.Candidates {
		sb.WriteString(fmt.Sprintf("- 候选 %d: %s | score=%.2f | source=%s", idx+1, cand.Path, cand.Score, joinIntentSources(cand.Sources)))
		if strings.TrimSpace(cand.Reason) != "" {
			sb.WriteString(" | reason=" + cand.Reason)
		}
		sb.WriteString("\n")
	}
	if len(plan.TrimmedCandidates) > 0 {
		trimmed := make([]string, 0, len(plan.TrimmedCandidates))
		for _, cand := range plan.TrimmedCandidates {
			trimmed = append(trimmed, cand.Path)
		}
		sb.WriteString("- 已裁切候选: " + strings.Join(trimmed, ", ") + "\n")
	}
	for _, explanation := range plan.Explanations {
		sb.WriteString("- 规则说明: " + explanation + "\n")
	}
	return strings.TrimSpace(sb.String())
}

func extractExplicitContextPaths(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	s := strings.ReplaceAll(text, "\\", "/")
	re := regexp.MustCompile(`(?i)(?:@)?([a-z0-9_./-]+\.(?:go|md|json|yaml|yml|toml|txt|js|ts|tsx|jsx|py|rs|java|c|cpp|h|hpp))`)
	matches := re.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		path := normalizeIntentContextPath(match[1])
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

func scoreRecentFileCandidate(path string, searchHints []string) (float64, string, bool) {
	path = normalizeIntentContextPath(path)
	if path == "" {
		return 0, "", false
	}
	lowerPath := strings.ToLower(path)
	base := strings.TrimSuffix(filepath.Base(lowerPath), filepath.Ext(lowerPath))
	best := 0.0
	var matched []string
	for _, hint := range searchHints {
		hint = strings.ToLower(strings.TrimSpace(hint))
		if hint == "" || len(hint) < 3 {
			continue
		}
		if strings.Contains(lowerPath, hint) {
			best = maxFloat(best, 0.82)
			matched = appendIfMissing(matched, hint)
			continue
		}
		if strings.Contains(base, hint) {
			best = maxFloat(best, 0.74)
			matched = appendIfMissing(matched, hint)
			continue
		}
		if strings.Contains(hint, base) || strings.Contains(base, strings.ReplaceAll(hint, "_", "")) {
			best = maxFloat(best, 0.66)
			matched = appendIfMissing(matched, hint)
		}
	}
	if best == 0 {
		return 0, "", false
	}
	return best, "路径与查询关键词匹配: " + strings.Join(matched, ","), true
}

func recentVersionedFilesList(cwd string, limit int) []string {
	if limit <= 0 {
		limit = 6
	}
	root := filepath.Join(cwd, ".eos", "versions")
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil
	}

	type rec struct {
		path string
		ts   int64
	}
	latest := map[string]int64{}
	seen := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if path != root {
				rel, err := filepath.Rel(root, path)
				if err == nil && strings.HasPrefix(filepath.ToSlash(rel), "_") {
					return filepath.SkipDir
				}
			}
			return nil
		}
		seen++
		if seen > 4000 {
			return fs.SkipAll
		}
		if !strings.HasSuffix(d.Name(), ".content") {
			return nil
		}
		dir := filepath.Dir(path)
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." || strings.HasPrefix(rel, "..") || strings.HasPrefix(rel, "_") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		ts := info.ModTime().UnixNano()
		if ts > latest[rel] {
			latest[rel] = ts
		}
		return nil
	})

	if len(latest) == 0 {
		return nil
	}
	rs := make([]rec, 0, len(latest))
	for path, ts := range latest {
		rs = append(rs, rec{path: path, ts: ts})
	}
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].ts != rs[j].ts {
			return rs[i].ts > rs[j].ts
		}
		return rs[i].path < rs[j].path
	})
	if len(rs) > limit {
		rs = rs[:limit]
	}
	out := make([]string, 0, len(rs))
	for _, item := range rs {
		out = append(out, item.path)
	}
	return out
}

func normalizeIntentContextPath(path string) string {
	path = strings.TrimSpace(strings.TrimPrefix(path, "@"))
	if path == "" {
		return ""
	}
	path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if path == "." || path == "" || strings.HasPrefix(path, "../") || path == ".." || filepath.IsAbs(path) {
		return ""
	}
	return path
}

func joinIntentSources(sources []IntentContextSource) string {
	if len(sources) == 0 {
		return ""
	}
	parts := make([]string, 0, len(sources))
	for _, source := range sources {
		parts = append(parts, string(source))
	}
	return strings.Join(parts, ",")
}

func clampIntentBudget(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampConfidence(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func appendIfMissing(values []string, target string) []string {
	if containsString(values, target) {
		return values
	}
	return append(values, target)
}

func recallIntentContextCandidates(cwd string, query string, explicitPaths []string, searchHints []string) codectx.RecallResult {
	engine, err := codectx.LoadOrBuild(cwd)
	if err != nil || engine == nil {
		return codectx.RecallResult{}
	}

	evidence := make([]codectx.RecallEvidence, 0, 12)
	for _, path := range CollectRecentFiles(cwd) {
		score, reason, matched := scoreRecentFileCandidate(path, searchHints)
		if !matched {
			score = 0.42
			reason = "缺少显式命中，回退到最近改动文件"
		}
		evidence = append(evidence, codectx.RecallEvidence{
			Path:     path,
			Source:   codectx.RecallSourceRecentChanges,
			Weight:   score,
			Priority: 180,
			Reason:   reason,
		})
	}
	for _, path := range recentVersionedFilesList(cwd, 6) {
		score, reason, matched := scoreRecentFileCandidate(path, searchHints)
		if !matched {
			score = 0.36
			reason = "缺少显式命中，回退到最近焦点文件"
		}
		evidence = append(evidence, codectx.RecallEvidence{
			Path:     path,
			Source:   codectx.RecallSourceRecentFocus,
			Weight:   score,
			Priority: 120,
			Reason:   reason,
		})
	}

	return engine.RecallIntent(query, codectx.RecallOptions{
		Limit:         intentContextMaxCandidates,
		ExplicitPaths: explicitPaths,
		PathHints:     searchHints,
		Evidence:      evidence,
	})
}

func mapRecallSource(source codectx.RecallSource) IntentContextSource {
	switch source {
	case codectx.RecallSourceIntentRecall:
		return IntentContextSourceIntentRecall
	case codectx.RecallSourceExplicitPath:
		return IntentContextSourceExplicitPath
	case codectx.RecallSourceRecentChanges:
		return IntentContextSourceRecentChanges
	case codectx.RecallSourceRecentFocus:
		return IntentContextSourceRecentFocus
	default:
		return ""
	}
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
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
