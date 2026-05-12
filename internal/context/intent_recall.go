package codectx

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type RecallSource string

const (
	RecallSourceIntentRecall  RecallSource = "intent_recall"
	RecallSourceExplicitPath  RecallSource = "explicit_path"
	RecallSourceRecentChanges RecallSource = "recent_changes"
	RecallSourceRecentFocus   RecallSource = "recent_focus"
)

type RecallEvidence struct {
	Path     string
	Source   RecallSource
	Weight   float64
	Priority int
	Reason   string
	Symbols  []string
}

type RecallOptions struct {
	Limit         int
	ExplicitPaths []string
	PathHints     []string
	Evidence      []RecallEvidence
}

type RecallCandidate struct {
	Path          string
	Lang          string
	Symbols       []string
	Sources       []RecallSource
	Score         float64
	TextScore     float64
	EvidenceScore float64
	Priority      int
	Reason        string
}

type RecallResult struct {
	Terms      []string
	Candidates []RecallCandidate
}

type recallAccumulator struct {
	path          string
	lang          string
	symbols       []string
	sources       map[RecallSource]struct{}
	reasons       []string
	priority      int
	textScore     float64
	evidenceScore float64
}

func LoadOrBuild(root string) (*Engine, error) {
	e := NewEngine(root)
	idxPath := filepath.Join(root, ".eos", "index.json")
	if _, err := os.Stat(idxPath); err == nil {
		if err := e.LoadIndex(idxPath); err == nil {
			return e, nil
		}
	}
	if err := e.BuildIndex(); err != nil {
		return nil, err
	}
	_ = os.MkdirAll(filepath.Dir(idxPath), 0o755)
	_ = e.SaveIndex(idxPath)
	return e, nil
}

func (e *Engine) RecallIntent(query string, opts RecallOptions) RecallResult {
	limit := opts.Limit
	if limit <= 0 {
		limit = 6
	}
	terms := collectIntentTerms(query, opts.ExplicitPaths, opts.PathHints)
	result := RecallResult{Terms: terms}

	e.mu.RLock()
	indexCount := len(e.Index)
	e.mu.RUnlock()
	if indexCount == 0 && len(opts.ExplicitPaths) == 0 && len(opts.Evidence) == 0 {
		return result
	}

	acc := map[string]*recallAccumulator{}
	if len(terms) > 0 {
		suggestions := e.Suggest(strings.Join(terms, " "), maxInt(limit*4, 12))
		topSuggestion := 0.0
		if len(suggestions) > 0 {
			topSuggestion = suggestions[0].Score
		}
		for _, suggestion := range suggestions {
			fm := e.lookupFileMeta(suggestion.Path)
			score, reason := scoreIntentMatch(suggestion.Path, fm, terms)
			score = maxFloat64(score, normalizeSuggestionScore(suggestion.Score, topSuggestion))
			if score < 0.32 {
				continue
			}
			if strings.TrimSpace(reason) == "" {
				reason = "自然语言意图召回命中"
			}
			addRecallSignal(acc, suggestion.Path, fm, RecallSourceIntentRecall, score, 160, reason, suggestion.Symbols)
		}

		e.mu.RLock()
		for path, fm := range e.Index {
			score, reason := scoreIntentMatch(path, fm, terms)
			if score < 0.62 {
				continue
			}
			addRecallSignal(acc, path, fm, RecallSourceIntentRecall, score, 140, reason, fm.Symbols)
		}
		e.mu.RUnlock()
	}

	for _, path := range opts.ExplicitPaths {
		path = normalizeRecallPath(path)
		if path == "" {
			continue
		}
		addRecallSignal(acc, path, e.lookupFileMeta(path), RecallSourceExplicitPath, 1.0, 300, "用户显式指定路径", nil)
	}
	for _, evidence := range opts.Evidence {
		path := normalizeRecallPath(evidence.Path)
		if path == "" {
			continue
		}
		source := evidence.Source
		if source == "" {
			source = RecallSourceRecentChanges
		}
		weight := evidence.Weight
		if weight <= 0 {
			weight = defaultEvidenceWeight(source)
		}
		priority := evidence.Priority
		if priority <= 0 {
			priority = defaultEvidencePriority(source)
		}
		reason := strings.TrimSpace(evidence.Reason)
		if reason == "" {
			reason = defaultEvidenceReason(source)
		}
		addRecallSignal(acc, path, e.lookupFileMeta(path), source, clampScore(weight), priority, reason, evidence.Symbols)
	}

	result.Candidates = finalizeRecallCandidates(acc)
	if len(result.Candidates) > limit {
		result.Candidates = result.Candidates[:limit]
	}
	return result
}

func collectIntentTerms(query string, explicitPaths []string, pathHints []string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(term string) {
		term = strings.ToLower(strings.TrimSpace(term))
		if len(term) < 3 {
			return
		}
		if isStopWord(term) {
			return
		}
		if _, ok := seen[term]; ok {
			return
		}
		seen[term] = struct{}{}
		out = append(out, term)
	}

	for _, token := range tokenizeQuery(query) {
		add(token)
		for _, part := range splitIntentToken(token) {
			add(part)
		}
	}
	for _, hint := range pathHints {
		for _, part := range splitIntentToken(hint) {
			add(part)
		}
	}
	for _, path := range explicitPaths {
		for _, part := range splitIntentToken(path) {
			add(part)
		}
	}
	for key, aliases := range naturalLanguageIntentAliases() {
		if !strings.Contains(query, key) {
			continue
		}
		for _, alias := range aliases {
			add(alias)
		}
	}

	if len(out) > 16 {
		out = out[:16]
	}
	return out
}

func splitIntentToken(token string) []string {
	token = strings.TrimSpace(strings.ReplaceAll(token, "\\", "/"))
	if token == "" {
		return nil
	}
	boundaryRe := regexp.MustCompile(`[._/\-]+`)
	camelRe := regexp.MustCompile(`([a-z0-9])([A-Z])`)
	normalized := camelRe.ReplaceAllString(token, `${1} ${2}`)
	normalized = boundaryRe.ReplaceAllString(normalized, " ")
	parts := strings.Fields(normalized)
	if len(parts) == 0 {
		return nil
	}
	out := make([]string, 0, len(parts)+1)
	out = append(out, parts...)
	joined := strings.ToLower(strings.Join(parts, ""))
	if len(joined) >= 3 {
		out = append(out, joined)
	}
	return out
}

func naturalLanguageIntentAliases() map[string][]string {
	return map[string][]string{
		"登录":  {"login", "signin", "account"},
		"鉴权":  {"auth", "token", "session", "oauth", "permission"},
		"认证":  {"auth", "login", "credential", "token"},
		"权限":  {"auth", "permission", "policy", "access"},
		"调度":  {"dispatch", "orchestration", "scheduler", "schedule"},
		"上下文": {"context"},
		"循环":  {"loop", "cycle"},
		"检测":  {"detect", "check", "guard"},
		"配置":  {"config", "settings"},
		"提示":  {"prompt"},
		"工具":  {"tool"},
		"插件":  {"plugin", "hook"},
		"会话":  {"session"},
		"运行时": {"runtime"},
		"桥接":  {"bridge"},
		"注入":  {"inject"},
		"搜索":  {"search", "query", "retrieve"},
		"排序":  {"rank", "sort", "order"},
		"证据":  {"evidence", "signal"},
		"报错":  {"error", "panic", "failure"},
		"错误":  {"error", "panic", "failure"},
		"测试":  {"test", "spec"},
	}
}

func scoreIntentMatch(path string, fm *FileMeta, terms []string) (float64, string) {
	path = strings.ToLower(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	var matched []string
	score := 0.0

	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if len(term) < 3 {
			continue
		}
		switch {
		case path == term || base == term:
			score = maxFloat64(score, 0.9)
			matched = appendUniqueString(matched, term)
		case strings.Contains(path, term):
			score = maxFloat64(score, 0.82)
			matched = appendUniqueString(matched, term)
		case strings.Contains(base, term):
			score = maxFloat64(score, 0.78)
			matched = appendUniqueString(matched, term)
		case strings.Contains(strings.ReplaceAll(base, "_", ""), strings.ReplaceAll(term, "_", "")):
			score = maxFloat64(score, 0.72)
			matched = appendUniqueString(matched, term)
		}
	}

	if fm != nil {
		for _, symbol := range fm.Symbols {
			lowerSymbol := strings.ToLower(symbol)
			for _, term := range terms {
				term = strings.ToLower(strings.TrimSpace(term))
				if len(term) < 3 {
					continue
				}
				if strings.Contains(lowerSymbol, term) {
					score = maxFloat64(score, 0.76)
					matched = appendUniqueString(matched, term)
				}
			}
		}
	}

	if len(matched) > 1 {
		score = clampScore(score + minFloat64(0.12, 0.04*float64(len(matched)-1)))
	}
	if score == 0 {
		return 0, ""
	}
	return score, "自然语言意图与路径/符号匹配: " + strings.Join(matched, ",")
}

func addRecallSignal(acc map[string]*recallAccumulator, path string, fm *FileMeta, source RecallSource, weight float64, priority int, reason string, symbols []string) {
	path = normalizeRecallPath(path)
	if path == "" {
		return
	}
	item, ok := acc[path]
	if !ok {
		item = &recallAccumulator{
			path:    path,
			sources: map[RecallSource]struct{}{},
		}
		acc[path] = item
	}
	item.sources[source] = struct{}{}
	if fm != nil {
		item.lang = fm.Lang
		if len(symbols) == 0 {
			symbols = fm.Symbols
		}
	}
	if source == RecallSourceIntentRecall {
		item.textScore = maxFloat64(item.textScore, clampScore(weight))
	} else {
		item.evidenceScore = maxFloat64(item.evidenceScore, clampScore(weight))
	}
	item.priority = maxInt(item.priority, priority)
	if reason = strings.TrimSpace(reason); reason != "" {
		item.reasons = appendUniqueString(item.reasons, reason)
	}
	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}
		item.symbols = appendUniqueString(item.symbols, symbol)
	}
}

func finalizeRecallCandidates(acc map[string]*recallAccumulator) []RecallCandidate {
	if len(acc) == 0 {
		return nil
	}
	out := make([]RecallCandidate, 0, len(acc))
	for _, item := range acc {
		sources := make([]RecallSource, 0, len(item.sources))
		for source := range item.sources {
			sources = append(sources, source)
		}
		sort.Slice(sources, func(i, j int) bool { return sources[i] < sources[j] })
		symbols := append([]string{}, item.symbols...)
		if len(symbols) > 12 {
			symbols = symbols[:12]
		}
		out = append(out, RecallCandidate{
			Path:          item.path,
			Lang:          item.lang,
			Symbols:       symbols,
			Sources:       sources,
			Score:         fuseRecallScore(item),
			TextScore:     item.textScore,
			EvidenceScore: item.evidenceScore,
			Priority:      item.priority,
			Reason:        strings.Join(item.reasons, "；"),
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

func fuseRecallScore(item *recallAccumulator) float64 {
	if item == nil {
		return 0
	}
	if _, ok := item.sources[RecallSourceExplicitPath]; ok {
		return 1.0
	}
	score := maxFloat64(item.textScore, item.evidenceScore)
	if item.textScore > 0 && item.evidenceScore > 0 {
		score += 0.12 * minFloat64(item.textScore, item.evidenceScore)
	}
	if len(item.sources) > 1 {
		score += minFloat64(0.12, 0.04*float64(len(item.sources)-1))
	}
	return clampScore(score)
}

func normalizeRecallPath(path string) string {
	path = strings.TrimSpace(strings.TrimPrefix(path, "@"))
	if path == "" {
		return ""
	}
	path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if path == "." || path == ".." || strings.HasPrefix(path, "../") || filepath.IsAbs(path) {
		return ""
	}
	return path
}

func normalizeSuggestionScore(raw float64, max float64) float64 {
	if raw <= 0 || max <= 0 {
		return 0
	}
	return clampScore(raw / max)
}

func defaultEvidenceWeight(source RecallSource) float64 {
	switch source {
	case RecallSourceExplicitPath:
		return 1.0
	case RecallSourceRecentChanges:
		return 0.82
	case RecallSourceRecentFocus:
		return 0.74
	default:
		return 0.58
	}
}

func defaultEvidencePriority(source RecallSource) int {
	switch source {
	case RecallSourceExplicitPath:
		return 300
	case RecallSourceRecentChanges:
		return 180
	case RecallSourceRecentFocus:
		return 120
	default:
		return 100
	}
}

func defaultEvidenceReason(source RecallSource) string {
	switch source {
	case RecallSourceExplicitPath:
		return "用户显式指定路径"
	case RecallSourceRecentChanges:
		return "最近改动证据"
	case RecallSourceRecentFocus:
		return "最近焦点证据"
	default:
		return "外部证据"
	}
}

func (e *Engine) lookupFileMeta(path string) *FileMeta {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.Index[normalizeRecallPath(path)]
}

func appendUniqueString(values []string, target string) []string {
	for _, value := range values {
		if value == target {
			return values
		}
	}
	return append(values, target)
}

func clampScore(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
