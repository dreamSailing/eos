package bridge

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/ai"
	codectx "github.com/dreamSailing/eos/internal/context"
)

func computeInjectBudgetBytes(rc *RuntimeCore, maxInjectKB int) int {
	if maxInjectKB > 0 {
		return maxInjectKB * 1024
	}
	model := ""
	if rc != nil && rc.cm != nil {
		model = rc.cm.ExportState().ModelName
	}
	window := ai.ContextWindowTokens(model)
	if window <= 0 {
		window = 128000
	}
	budget := int(float64(window) * 4 * 0.06)
	return clampInt(budget, 16*1024, 128*1024)
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
			return nil
		}
		seen++
		if seen > 4000 {
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
