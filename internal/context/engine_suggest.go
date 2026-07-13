package codectx

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Suggestion struct {
	Path    string   `json:"path"`
	Lang    string   `json:"lang"`
	Score   float64  `json:"score"`
	Symbols []string `json:"symbols"`
}

func (e *Engine) Suggest(query string, k int) []Suggestion {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if k <= 0 {
		k = 5
	}
	q := normalizeTokens(tokenizeQuery(query))
	if len(q) == 0 {
		return nil
	}
	N := float64(len(e.Index))
	const k1 = 1.5
	const b = 0.75
	sumdl := 0
	for _, fm := range e.Index {
		sumdl += len(fm.Tokens)
	}
	avgdl := 1.0
	if sumdl > 0 {
		avgdl = float64(sumdl) / float64(len(e.Index))
	}
	scores := map[string]float64{}
	for _, term := range q {
		df := float64(e.docFreq[term])
		if df <= 0 {
			continue
		}
		idf := mathLog((N - df + 0.5) / (df + 0.5))
		postings := e.inv[term]
		for path, tf := range postings {
			dl := float64(len(e.Index[path].Tokens))
			base := (float64(tf) * (k1 + 1.0)) / (float64(tf) + k1*(1.0-b+b*(dl/avgdl)))
			scores[path] += idf * base
		}
	}
	for p := range scores {
		lp := strings.ToLower(p)
		for _, raw := range pathHints(query) {
			if strings.Contains(lp, strings.ToLower(raw)) {
				scores[p] *= 1.3
			}
		}
		nb := 0
		if m := e.imports[p]; m != nil {
			nb += len(m)
		}
		if m := e.rimports[p]; m != nil {
			nb += len(m)
		}
		if nb > 0 {
			scores[p] *= 1.0 + 0.05*float64(nb)
		}
		sz := e.Index[p].Size
		if sz > 0 && sz < 64*1024 {
			scores[p] *= 1.1
		}
	}
	type pair struct {
		path  string
		score float64
	}
	var arr []pair
	for p, s := range scores {
		arr = append(arr, pair{p, s})
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].score > arr[j].score })
	if len(arr) > k {
		arr = arr[:k]
	}
	out := make([]Suggestion, 0, len(arr))
	for _, pr := range arr {
		fm := e.Index[pr.path]
		if fm == nil {
			continue
		}
		syms := fm.Symbols
		if len(syms) > 12 {
			syms = syms[:12]
		}
		out = append(out, Suggestion{Path: fm.Path, Lang: fm.Lang, Score: pr.score, Symbols: syms})
	}
	return out
}

func (e *Engine) Neighbors(path string, depth int) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if depth <= 0 {
		depth = 1
	}
	seen := map[string]struct{}{path: {}}
	cur := []string{path}
	var out []string
	for d := 0; d < depth; d++ {
		var next []string
		for _, p := range cur {
			for q := range e.imports[p] {
				if _, ok := seen[q]; !ok {
					seen[q] = struct{}{}
					next = append(next, q)
					out = append(out, q)
				}
			}
			for q := range e.rimports[p] {
				if _, ok := seen[q]; !ok {
					seen[q] = struct{}{}
					next = append(next, q)
					out = append(out, q)
				}
			}
		}
		cur = next
		if len(cur) == 0 {
			break
		}
	}
	return out
}

func (e *Engine) ImportsOf(path string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var out []string
	if m := e.imports[path]; m != nil {
		for k := range m {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func (e *Engine) ReverseImportsOf(path string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var out []string
	if m := e.rimports[path]; m != nil {
		for k := range m {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func tokenizeQuery(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	re := regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]{2,}`)
	return re.FindAllString(s, -1)
}

func pathHints(s string) []string {
	var out []string
	parts := strings.Split(s, " ")
	for _, p := range parts {
		i := strings.Index(p, "@")
		if i >= 0 {
			q := strings.TrimSpace(p[i+1:])
			if q != "" {
				out = append(out, filepath.ToSlash(q))
			}
		}
	}
	return out
}

func moduleGuess(path string) string {
	segs := strings.Split(filepath.ToSlash(path), "/")
	if len(segs) >= 2 {
		return strings.Join(segs[:2], "/")
	}
	if len(segs) >= 1 {
		return segs[0]
	}
	return path
}

func mathLog(x float64) float64 {
	return fastLog(x)
}

func fastLog(x float64) float64 {
	if x <= 0 {
		return 0
	}
	t := (x - 1.0) / (x + 1.0)
	t2 := t * t
	return 2.0 * (t + (t*t2)/3.0)
}
