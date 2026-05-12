package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/ai"
	codectx "github.com/dreamSailing/eos/internal/context"
)

type injectedContextPackage struct {
	entries       []string
	summary       string
	usedBytes     int
	includedFiles int
	trimmedFiles  int
	omittedFiles  int
	omittedPaths  []string
}

func computeInjectBudgetBytes(rc *RuntimeCore, maxInjectKB int) int {
	explicitBudget := maxInjectKB > 0
	if maxInjectKB > 0 {
		maxInjectKB *= 1024
	}
	budget := maxInjectKB
	model := ""
	if rc != nil && rc.cm != nil {
		model = rc.cm.ExportState().ModelName
	}
	if budget <= 0 {
		window := ai.ContextWindowTokens(model)
		if window <= 0 {
			window = 128000
		}
		budget = clampInt(int(float64(window)*4*0.06), 16*1024, 128*1024)
	}
	if rc != nil && rc.cm != nil {
		_, _, remaining := rc.cm.PromptBudgetStatus()
		if remaining <= 0 {
			return 0
		}
		remainingBudget := int(float64(remaining*4) * 0.55)
		if remainingBudget <= 0 {
			return 0
		}
		if budget > remainingBudget {
			budget = remainingBudget
		}
	}
	if budget < 1024 {
		if explicitBudget {
			return budget
		}
		return 0
	}
	if explicitBudget {
		return budget
	}
	return clampInt(budget, 1024, 128*1024)
}

func buildInjectCandidates(rc *RuntimeCore, query string, sugg []codectx.Suggestion, limit int) []codectx.Suggestion {
	if limit <= 0 {
		limit = 4
	}

	root := ""
	versionsRoot := filepath.Join(".eos", "versions")
	if rc != nil {
		root = rc.workingRoot()
		versionsRoot = rc.versionsRoot()
	}

	seen := map[string]struct{}{}
	out := make([]codectx.Suggestion, 0, limit)
	add := func(p string, sym []string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		if !isSafeRelPath(p) {
			return
		}
		if !fileExistsUnderRoot(root, p) {
			return
		}
		seen[p] = struct{}{}
		out = append(out, codectx.Suggestion{Path: p, Symbols: sym})
	}

	for _, p := range extractMentionedPaths(query) {
		add(p, nil)
		if len(out) >= limit {
			return out
		}
	}
	for _, p := range recentVersionedFilesList(versionsRoot, 6) {
		add(p, nil)
		if len(out) >= limit {
			return out
		}
	}
	for _, s := range sugg {
		add(s.Path, s.Symbols)
		if len(out) >= limit {
			return out
		}
	}
	return out
}

func extractMentionedPaths(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	s := strings.ReplaceAll(text, "\\", "/")
	re := regexp.MustCompile(`(?i)(?:@)?([a-z0-9_./-]+\.(?:go|md|json|yaml|yml|toml|txt|js|ts|tsx|jsx|py|rs|java|c|cpp|h|hpp))`)
	ms := re.FindAllStringSubmatch(s, 20)
	if len(ms) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		if len(m) < 2 {
			continue
		}
		p := strings.TrimPrefix(strings.TrimSpace(m[1]), "/")
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func extractRelevantSnippet(content string, query string, symbols []string, maxBytes int) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if maxBytes <= 0 {
		maxBytes = 4096
	}
	if len(content) <= maxBytes {
		return content
	}

	ks := make([]string, 0, 12)
	for _, s := range symbols {
		s = strings.TrimSpace(s)
		if len(s) < 3 {
			continue
		}
		ks = append(ks, s)
		if len(ks) >= 6 {
			break
		}
	}
	for _, w := range tokenizeKeywords(query) {
		ks = append(ks, w)
		if len(ks) >= 12 {
			break
		}
	}

	lines := strings.Split(content, "\n")
	best := -1
	for i, l := range lines {
		ll := strings.ToLower(l)
		for _, k := range ks {
			if strings.Contains(ll, strings.ToLower(k)) {
				best = i
				break
			}
		}
		if best >= 0 {
			break
		}
	}
	if best < 0 {
		return content[:maxBytes] + "\n...truncated"
	}

	start := best - 80
	if start < 0 {
		start = 0
	}
	end := best + 160
	if end > len(lines) {
		end = len(lines)
	}
	out := strings.Join(lines[start:end], "\n")
	if len(out) > maxBytes {
		out = out[:maxBytes] + "\n...truncated"
	}
	return out
}

func tokenizeKeywords(text string) []string {
	s := strings.ToLower(text)
	re := regexp.MustCompile(`[a-z0-9_./-]{3,}`)
	ws := re.FindAllString(s, 50)
	out := make([]string, 0, len(ws))
	seen := map[string]struct{}{}
	for _, w := range ws {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		if strings.Contains(w, "/") && strings.Contains(w, ".") {
			continue
		}
		if _, ok := seen[w]; ok {
			continue
		}
		seen[w] = struct{}{}
		out = append(out, w)
	}
	return out
}

func (rc *RuntimeCore) buildInjectedContextPackage(query string, sugg []codectx.Suggestion, maxBytes int) injectedContextPackage {
	pkg := injectedContextPackage{
		omittedPaths: make([]string, 0, 4),
	}
	if maxBytes <= 0 || len(sugg) == 0 {
		return pkg
	}

	perFileMax := clampInt(maxBytes/clampInt(len(sugg), 1, 12), 1536, 12288)
	for idx, candidate := range sugg {
		left := maxBytes - pkg.usedBytes
		if left < 512 {
			pkg.omittedFiles += len(sugg) - idx
			pkg.omittedPaths = appendInjectedOmitted(pkg.omittedPaths, candidate.Path)
			for rest := idx + 1; rest < len(sugg); rest++ {
				pkg.omittedPaths = appendInjectedOmitted(pkg.omittedPaths, sugg[rest].Path)
			}
			break
		}

		remainingCandidates := len(sugg) - idx
		budget := left
		if remainingCandidates > 0 {
			fairShare := left / remainingCandidates
			if fairShare > 0 && fairShare < budget {
				budget = fairShare
			}
		}
		if budget > perFileMax {
			budget = perFileMax
		}
		if budget < 512 {
			pkg.omittedFiles++
			pkg.omittedPaths = appendInjectedOmitted(pkg.omittedPaths, candidate.Path)
			continue
		}

		entry, trimmed, err := rc.buildInjectedContextEntry(query, candidate, budget)
		if err != nil || strings.TrimSpace(entry) == "" {
			pkg.omittedFiles++
			pkg.omittedPaths = appendInjectedOmitted(pkg.omittedPaths, candidate.Path)
			continue
		}
		entryBytes := len(entry)
		if entryBytes > left {
			pkg.omittedFiles++
			pkg.omittedPaths = appendInjectedOmitted(pkg.omittedPaths, candidate.Path)
			continue
		}

		pkg.entries = append(pkg.entries, entry)
		pkg.usedBytes += entryBytes
		pkg.includedFiles++
		if trimmed {
			pkg.trimmedFiles++
		}
	}
	pkg.summary = formatInjectedContextSummary(pkg, maxBytes)
	return pkg
}

func (rc *RuntimeCore) buildInjectedContextEntry(query string, sugg codectx.Suggestion, maxBytes int) (string, bool, error) {
	if maxBytes <= 0 {
		return "", false, nil
	}
	contentBudget := maxBytes - 256
	if contentBudget < 256 {
		contentBudget = maxBytes
	}
	ap := rc.resolveWithinRoot(sugg.Path)
	bs, err := os.ReadFile(ap)
	if err != nil {
		return "", false, err
	}

	raw := strings.ReplaceAll(string(bs), "\r\n", "\n")
	snippet := extractRelevantSnippet(raw, query, sugg.Symbols, contentBudget)
	snippet = strings.TrimSpace(snippet)
	if snippet == "" {
		return "", false, nil
	}

	trimmed := len(snippet) < len(raw)
	meta := buildInjectedMetaLine(sugg, trimmed)
	entry := "@" + strings.TrimSpace(sugg.Path) + "\n" + meta + "\n" + snippet
	if len(entry) > maxBytes {
		snippet = extractRelevantSnippet(raw, query, sugg.Symbols, maxBytes/2)
		snippet = strings.TrimSpace(snippet)
		entry = "@" + strings.TrimSpace(sugg.Path) + "\n" + meta + "\n" + snippet
		trimmed = true
	}
	if len(entry) > maxBytes {
		entry = entry[:maxBytes]
		trimmed = true
	}
	return strings.TrimSpace(entry), trimmed, nil
}

func buildInjectedMetaLine(sugg codectx.Suggestion, trimmed bool) string {
	parts := []string{"[auto-context"}
	if len(sugg.Symbols) > 0 {
		symbols := sugg.Symbols
		if len(symbols) > 4 {
			symbols = symbols[:4]
		}
		parts = append(parts, "symbols="+strings.Join(symbols, ","))
	}
	if trimmed {
		parts = append(parts, "trimmed")
	}
	return strings.Join(parts, "; ") + "]"
}

func appendInjectedOmitted(paths []string, path string) []string {
	path = strings.TrimSpace(path)
	if path == "" || len(paths) >= 4 {
		return paths
	}
	return append(paths, path)
}

func formatInjectedContextSummary(pkg injectedContextPackage, maxBytes int) string {
	if pkg.includedFiles == 0 && pkg.omittedFiles == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("AutoContext package: used ")
	sb.WriteString(strconv.Itoa(pkg.usedBytes))
	sb.WriteString("/")
	sb.WriteString(strconv.Itoa(maxBytes))
	sb.WriteString(" bytes, included ")
	sb.WriteString(strconv.Itoa(pkg.includedFiles))
	sb.WriteString(" file(s)")
	if pkg.trimmedFiles > 0 {
		sb.WriteString(", trimmed ")
		sb.WriteString(strconv.Itoa(pkg.trimmedFiles))
	}
	if pkg.omittedFiles > 0 {
		sb.WriteString(", omitted ")
		sb.WriteString(strconv.Itoa(pkg.omittedFiles))
	}
	if len(pkg.omittedPaths) > 0 {
		sb.WriteString(" [")
		sb.WriteString(strings.Join(pkg.omittedPaths, ", "))
		if pkg.omittedFiles > len(pkg.omittedPaths) {
			sb.WriteString(", +")
			sb.WriteString(strconv.Itoa(pkg.omittedFiles - len(pkg.omittedPaths)))
		}
		sb.WriteString("]")
	}
	return sb.String()
}

func recentVersionedFilesList(root string, limit int) []string {
	if limit <= 0 {
		limit = 6
	}
	if _, err := os.Stat(root); err != nil {
		return nil
	}

	latest := map[string]time.Time{}
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
		name := d.Name()
		if !strings.HasSuffix(name, ".content") {
			return nil
		}
		dir := filepath.Dir(path)
		relDir, err := filepath.Rel(root, dir)
		relDir = filepath.ToSlash(relDir)
		if err != nil || strings.HasPrefix(relDir, "..") || relDir == "." || strings.HasPrefix(relDir, "_") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		ts := info.ModTime()
		existing, ok := latest[relDir]
		if !ok || ts.After(existing) {
			latest[relDir] = ts
		}
		return nil
	})

	type rec struct {
		pathRel string
		ts      time.Time
	}
	rs := make([]rec, 0, len(latest))
	for p, ts := range latest {
		rs = append(rs, rec{pathRel: filepath.ToSlash(p), ts: ts})
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i].ts.After(rs[j].ts) })
	if len(rs) > limit {
		rs = rs[:limit]
	}
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.pathRel)
	}
	return out
}

func fileExistsUnderRoot(root, rel string) bool {
	ap := filepath.Clean(filepath.FromSlash(rel))
	if root != "" {
		ap = filepath.Join(root, filepath.FromSlash(rel))
	}
	fi, err := os.Stat(ap)
	return err == nil && !fi.IsDir()
}

func isSafeRelPath(rel string) bool {
	p := filepath.Clean(filepath.FromSlash(rel))
	if p == "." || p == "" {
		return false
	}
	if strings.HasPrefix(p, "..") || filepath.IsAbs(p) {
		return false
	}
	return true
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
