package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	codectx "github.com/dreamSailing/eos/internal/context"
	"github.com/dreamSailing/eos/internal/pkg/utils"
	"github.com/dreamSailing/eos/internal/search"

	"github.com/bmatcuk/doublestar/v4"
)

// SearchOptions holds options for the unified search tool
type SearchOptions struct {
	Mode            string   // "glob", "regex", "text", "code", "deps", "graph"
	Pattern         string   // Search pattern
	Root            string   // Search root directory
	MaxDepth        int      // Maximum recursion depth
	Exclude         []string // Exclude patterns
	MinSize         int64    // Minimum file size
	MaxSize         int64    // Maximum file size
	Limit           int      // Max results limit
	Context         int      // Context lines for text/regex search
	CaseInsensitive bool
	Flags           string // Regex flags
	K               int    // For code graph: number of suggestions
	Depth           int    // For deps: dependency depth
}

func (m *Manager) searchStructured(ctx context.Context, params map[string]any) ToolResult {
	mode, _ := params["mode"].(string)
	pattern, _ := params["pattern"].(string)
	root, _ := params["root"].(string)
	maxDepth := toInt(params["max_depth"], 0)
	exclude := toStringSlice(params["exclude"])
	minSize := toInt64(params["min_size"], 0)
	maxSize := toInt64(params["max_size"], 0)
	limit := toInt(params["limit"], 1000)
	contextLines := toInt(params["context"], 0)
	caseInsensitive, _ := params["case_insensitive"].(bool)
	flags, _ := params["flags"].(string)
	k := toInt(params["k"], 5)
	depth := toInt(params["depth"], 1)

	if strings.TrimSpace(mode) == "" {
		mode = "glob"
	}
	if strings.TrimSpace(pattern) == "" {
		return ToolResult{Type: "tool_result", Tool: "search", Status: "error", Error: "pattern required"}
	}

	if strings.TrimSpace(root) == "" {
		root = "."
	}
	root = normalizePathPlaceholder(root)
	res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), root)
	if !res.IsValid {
		slog.Error("search.out_of_root", "component", utils.ComponentTool, "mode", mode, "pattern", pattern, "path", root, "error", res.ErrMsg)
		return ToolResult{Type: "tool_result", Tool: "search", Status: "error", Error: "path outside working directory"}
	}
	ap := res.AbsPath
	relRoot := res.RelPath

	switch mode {
	case "glob":
		return m.searchGlob(ctx, ap, relRoot, pattern, maxDepth, exclude, minSize, maxSize)
	case "regex":
		return m.searchRegex(ctx, ap, relRoot, pattern, maxDepth, exclude, limit, contextLines, flags)
	case "text":
		return m.searchText(ctx, ap, relRoot, pattern, maxDepth, exclude, limit, contextLines, caseInsensitive)
	case "code":
		return m.searchCode(ctx, ap, relRoot, pattern, k)
	case "deps":
		return m.searchDeps(ctx, ap, relRoot, pattern, depth, limit)
	case "graph":
		return m.searchGraph(ctx, ap, limit)
	default:
		return ToolResult{Type: "tool_result", Tool: "search", Status: "error", Error: fmt.Sprintf("unknown mode: %s (valid: glob, regex, text, code, deps, graph)", mode)}
	}
}

func (m *Manager) searchGlob(_ context.Context, root, relRoot, pattern string, maxDepth int, exclude []string, minSize, maxSize int64) ToolResult {
	var matches []string

	// 默认排除的目录（VCS、构建产物等）
	defaultExcludeDirs := map[string]bool{
		".git": true, ".svn": true, ".hg": true,
		"node_modules": true, ".eos": true, "__pycache__": true,
		".idea": true, ".vscode": true, "vendor": true,
	}

	// Load .eosignore patterns.
	di := NewDotIgnore(root)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 自动跳过 VCS 和构建产物目录
		if info.IsDir() && path != root {
			base := filepath.Base(path)
			if defaultExcludeDirs[base] {
				return filepath.SkipDir
			}
			// Check .eosignore for directories.
			if di.Match(path) {
				return filepath.SkipDir
			}
		}

		// Calculate depth
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		depth := len(strings.Split(filepath.ToSlash(relPath), "/")) - 1

		// Check max depth
		if maxDepth > 0 && depth > maxDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check exclude patterns
		if info.IsDir() {
			base := filepath.Base(path)
			for _, ex := range exclude {
				matched, _ := doublestar.Match(ex, base)
				if matched {
					return filepath.SkipDir
				}
			}
		}

		// Skip directories by default
		if info.IsDir() {
			return nil
		}

		// Check .eosignore for files.
		if di.Match(path) {
			return nil
		}

		// Check file size
		if minSize > 0 && info.Size() < minSize {
			return nil
		}
		if maxSize > 0 && info.Size() > maxSize {
			return nil
		}

		// Use doublestar for pattern matching (supports **)
		matched, err := doublestar.Match(pattern, filepath.ToSlash(relPath))
		if err != nil {
			return err
		}
		if matched {
			matches = append(matches, filepath.ToSlash(relPath))
		}

		return nil
	})

	if err != nil {
		slog.Error("search.glob.error", "component", utils.ComponentTool, "pattern", pattern, "root", root, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "search", Status: "error", Error: fmt.Sprintf("%v", err)}
	}

	return ToolResult{Type: "tool_result", Tool: "search", Status: "success", Data: map[string]any{"mode": "glob", "root": filepath.ToSlash(relRoot), "matches": matches}, Display: fmt.Sprintf("%d match(es)", len(matches))}
}

func (m *Manager) searchRegex(_ context.Context, root, relRoot, pattern string, _ int, exclude []string, limit, contextLines int, flags string) ToolResult {
	opts := search.Options{
		Root:         root,
		Includes:     exclude,    // used as allowlist
		Excludes:     []string{}, // not used in regex mode
		Limit:        limit,
		ContextLines: contextLines,
		Flags:        flags,
		MaxFileSize:  2 * 1024 * 1024,
	}
	res, trunc, err := search.DirRegex(pattern, opts)
	res = filterIgnoredResults(root, res)
	return searchResult("regex", relRoot, res, trunc, err)
}

func (m *Manager) searchText(_ context.Context, root, relRoot, pattern string, _ int, exclude []string, limit, contextLines int, caseInsensitive bool) ToolResult {
	opts := search.Options{
		Root:            root,
		Includes:        exclude,
		Excludes:        []string{},
		Limit:           limit,
		ContextLines:    contextLines,
		CaseInsensitive: caseInsensitive,
		MaxFileSize:     2 * 1024 * 1024,
	}
	res, trunc, err := search.DirText(pattern, opts)
	res = filterIgnoredResults(root, res)
	return searchResult("text", relRoot, res, trunc, err)
}

func searchResult(mode, relRoot string, res []search.Result, trunc bool, err error) ToolResult {
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: "search", Status: "error", Error: fmt.Sprintf("%v", err)}
	}
	return ToolResult{
		Type:   "tool_result",
		Tool:   "search",
		Status: "success",
		Data: map[string]any{
			"mode":      mode,
			"root":      filepath.ToSlash(relRoot),
			"results":   mapResults(res),
			"truncated": trunc,
		},
		Display: fmt.Sprintf("%d match(es)%s", len(res), func() string {
			if trunc {
				return " (truncated)"
			}
			return ""
		}()),
	}
}

func filterIgnoredResults(root string, results []search.Result) []search.Result {
	di := NewDotIgnore(root)
	if len(di.Load()) == 0 || len(results) == 0 {
		return results
	}
	filtered := make([]search.Result, 0, len(results))
	for _, item := range results {
		path := item.File
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		if di.Match(path) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func (m *Manager) searchCode(_ context.Context, root, relRoot, query string, k int) ToolResult {
	e := codectx.NewEngine(root)
	idxp := filepath.Join(root, ".eos", "index.json")
	if _, err := os.Stat(idxp); err == nil {
		_ = e.LoadIndex(idxp)
	} else {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			_ = e.BuildIndex()
			_ = os.MkdirAll(filepath.Dir(idxp), 0755)
			_ = e.SaveIndex(idxp)
			wg.Done()
		}()
		// Don't wait for index build to complete if query can be answered from partial results
	}
	sugg := e.Suggest(query, k)
	out := make([]map[string]any, 0, len(sugg))
	for _, s := range sugg {
		out = append(out, map[string]any{"path": s.Path, "lang": s.Lang, "score": s.Score, "symbols": s.Symbols})
	}
	return ToolResult{Type: "tool_result", Tool: "search", Status: "success", Data: map[string]any{"mode": "code", "root": filepath.ToSlash(relRoot), "suggestions": out}, Display: fmt.Sprintf("%d suggestion(s)", len(out))}
}

func (m *Manager) searchDeps(_ context.Context, root string, _ string, file string, depth, limit int) ToolResult {
	if strings.TrimSpace(file) == "" {
		return ToolResult{Type: "tool_result", Tool: "search", Status: "error", Error: "file required"}
	}
	e := codectx.NewEngine(root)
	idxp := filepath.Join(root, ".eos", "index.json")
	if _, err := os.Stat(idxp); err == nil {
		_ = e.LoadIndex(idxp)
	} else {
		_ = e.BuildIndex()
		_ = os.MkdirAll(filepath.Dir(idxp), 0755)
		_ = e.SaveIndex(idxp)
	}
	imports := e.ImportsOf(filepath.ToSlash(file))
	rimports := e.ReverseImportsOf(filepath.ToSlash(file))
	neigh := e.Neighbors(filepath.ToSlash(file), depth)
	if limit > 0 && len(neigh) > limit {
		neigh = neigh[:limit]
	}
	deg := len(imports) + len(rimports)
	lang := ""
	if fm := e.Index[filepath.ToSlash(file)]; fm != nil {
		lang = fm.Lang
	}
	return ToolResult{Type: "tool_result", Tool: "search", Status: "success", Data: map[string]any{"mode": "deps", "file": filepath.ToSlash(file), "lang": lang, "imports": imports, "reverse_imports": rimports, "neighbors": neigh, "degree": deg}, Display: fmt.Sprintf("Dependencies: imports=%d, reverse=%d, degree=%d", len(imports), len(rimports), deg)}
}

func (m *Manager) searchGraph(_ context.Context, root string, limit int) ToolResult {
	e := codectx.NewEngine(root)
	if err := e.BuildIndex(); err != nil {
		slog.Error("search.graph.error", "component", utils.ComponentTool, "root", root, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "search", Status: "error", Error: fmt.Sprintf("%v", err)}
	}
	type node struct {
		Path, Lang string
		Degree     int
	}
	nodes := make([]node, 0, len(e.Index))
	langCounts := map[string]int{}
	for p, fm := range e.Index {
		d := len(e.ImportsOf(p)) + len(e.ReverseImportsOf(p))
		nodes = append(nodes, node{Path: p, Lang: fm.Lang, Degree: d})
		if fm.Lang != "" {
			langCounts[fm.Lang]++
		}
	}
	// Sort by degree
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			if nodes[j].Degree > nodes[i].Degree {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			}
		}
	}
	if limit > 0 && len(nodes) > limit {
		nodes = nodes[:limit]
	}
	type edge struct{ From, To string }
	var edges []edge
	maxEdges := 1000
	for to := range e.Index {
		for _, from := range e.ReverseImportsOf(to) {
			edges = append(edges, edge{From: from, To: to})
			if maxEdges > 0 && len(edges) >= maxEdges {
				break
			}
		}
		if maxEdges > 0 && len(edges) >= maxEdges {
			break
		}
	}
	hotspots := make([]string, 0, 10)
	for i := 0; i < len(nodes) && i < 10; i++ {
		hotspots = append(hotspots, fmt.Sprintf("%s(%d)", nodes[i].Path, nodes[i].Degree))
	}
	ns := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		ns = append(ns, map[string]any{"path": n.Path, "lang": n.Lang, "degree": n.Degree})
	}
	es := make([]map[string]any, 0, len(edges))
	for _, e2 := range edges {
		es = append(es, map[string]any{"from": e2.From, "to": e2.To})
	}
	wd, _ := os.Getwd()
	relRoot, _ := filepath.Rel(wd, root)
	return ToolResult{Type: "tool_result", Tool: "search", Status: "success", Data: map[string]any{"mode": "graph", "root": filepath.ToSlash(relRoot), "files_total": len(e.Index), "lang_counts": langCounts, "nodes": ns, "edges": es, "hotspots": hotspots}, Display: fmt.Sprintf("Graph: files=%d · nodes=%d · edges=%d", len(e.Index), len(ns), len(es))}
}
