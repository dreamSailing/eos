package codectx

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"bufio"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type FileMeta struct {
	Path    string   `json:"path"`
	Lang    string   `json:"lang"`
	Imports []string `json:"imports"`
	Symbols []string `json:"symbols"`
	Tokens  []string `json:"tokens"`
	Size    int      `json:"size"`
	MTime   int64    `json:"mtime"`
}

type Engine struct {
	Root           string
	Index          map[string]*FileMeta
	inv            map[string]map[string]int // token -> path -> tf
	docFreq        map[string]int            // token -> df
	imports        map[string]map[string]struct{}
	rimports       map[string]map[string]struct{}
	lastBuild      time.Time
	mu             sync.RWMutex
	ignorePatterns []string
	debounceMs     int
}

func NewEngine(root string) *Engine {
	e := &Engine{
		Root:           root,
		Index:          map[string]*FileMeta{},
		inv:            map[string]map[string]int{},
		docFreq:        map[string]int{},
		imports:        map[string]map[string]struct{}{},
		rimports:       map[string]map[string]struct{}{},
		ignorePatterns: []string{".git", ".eos", ".claude", "node_modules", "dist", "build", "vendor", ".idea", ".vscode"},
		debounceMs:     300,
	}
	// Load .vbignore patterns and merge into ignorePatterns
	if extra := loadVBIgnorePatterns(root); len(extra) > 0 {
		e.ignorePatterns = append(e.ignorePatterns, extra...)
	}
	return e
}

// loadVBIgnorePatterns reads .vbignore from the project root and returns patterns.
// Duplicated from tools.DotIgnore to avoid circular import (internal/tools -> internal/context).
func loadVBIgnorePatterns(root string) []string {
	ignorePath := filepath.Join(root, ".vbignore")
	f, err := os.Open(ignorePath)
	if err != nil {
		return nil
	}
	defer f.Close()
	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

func (e *Engine) BuildIndex() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Index = map[string]*FileMeta{}
	e.inv = map[string]map[string]int{}
	e.docFreq = map[string]int{}
	e.imports = map[string]map[string]struct{}{}
	e.rimports = map[string]map[string]struct{}{}
	root := e.Root
	if root == "" {
		wd, _ := os.Getwd()
		root = wd
		e.Root = root
	}
	fset := token.NewFileSet()
	wordRe := regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]{2,}`)
	now := time.Now()
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		// ignore dirs
		if d.IsDir() {
			base := filepath.Base(path)
			lb := strings.ToLower(base)
			for _, ig := range e.ignorePatterns {
				if lb == ig {
					return filepath.SkipDir
				}
			}
			return nil
		}
		// Check .vbignore patterns for files (basename match)
		base := filepath.Base(path)
		for _, ig := range e.ignorePatterns {
			if matched, _ := filepath.Match(ig, base); matched {
				return nil
			}
		}
		// only index text-like files
		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" {
			return nil
		}
		switch ext {
		case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".cs", ".php", ".rb", ".sql", ".sh", ".yaml", ".yml", ".toml", ".json":
		default:
			return nil
		}
		info, _ := os.Stat(path)
		if info != nil && info.Size() > 2*1024*1024 { // skip very large files
			return nil
		}
		rel := path
		if !filepath.IsAbs(rel) {
			rel = filepath.Join(root, path)
		}
		if r, err := filepath.Rel(root, rel); err == nil {
			rel = filepath.ToSlash(r)
		}
		fm := &FileMeta{Path: rel, Size: int(info.Size()), MTime: info.ModTime().Unix()}
		switch ext {
		case ".go":
			fm.Lang = "go"
			// parse go file for imports and symbols
			file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err == nil && file != nil {
				for _, im := range file.Imports {
					s := strings.Trim(strings.TrimSpace(im.Path.Value), `"`)
					if s != "" {
						fm.Imports = append(fm.Imports, s)
						if e.imports[fm.Path] == nil {
							e.imports[fm.Path] = map[string]struct{}{}
						}
						e.imports[fm.Path][s] = struct{}{}
					}
				}
				ast.Inspect(file, func(n ast.Node) bool {
					switch x := n.(type) {
					case *ast.FuncDecl:
						if x.Name != nil {
							fm.Symbols = append(fm.Symbols, x.Name.Name)
						}
					case *ast.TypeSpec:
						fm.Symbols = append(fm.Symbols, x.Name.Name)
					case *ast.ValueSpec:
						for _, id := range x.Names {
							fm.Symbols = append(fm.Symbols, id.Name)
						}
					}
					return true
				})
			}
			// tokenize source for generic retrieval
			b, _ := os.ReadFile(path)
			toks := wordRe.FindAllString(string(b), -1)
			fm.Tokens = normalizeTokens(toks)
		case ".ts", ".tsx", ".js", ".jsx":
			fm.Lang = "ts"
			b, _ := os.ReadFile(path)
			s := string(b)
			toks := wordRe.FindAllString(s, -1)
			fm.Tokens = normalizeTokens(toks)
			// imports: import ... from 'x'; require('x')
			impRe1 := regexp.MustCompile(`(?m)import\s+[^;]*?from\s+['\"]([^'\"]+)['\"]`)
			impRe2 := regexp.MustCompile(`(?m)require\(\s*['\"]([^'\"]+)['\"]\s*\)`)
			for _, m := range impRe1.FindAllStringSubmatch(s, -1) {
				fm.Imports = append(fm.Imports, m[1])
			}
			for _, m := range impRe2.FindAllStringSubmatch(s, -1) {
				fm.Imports = append(fm.Imports, m[1])
			}
			if len(fm.Imports) > 0 {
				if e.imports[fm.Path] == nil {
					e.imports[fm.Path] = map[string]struct{}{}
				}
				for _, p := range fm.Imports {
					e.imports[fm.Path][p] = struct{}{}
				}
			}
			// symbols: function name(...) / class Name
			fRe := regexp.MustCompile(`(?m)function\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
			cRe := regexp.MustCompile(`(?m)class\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
			for _, m := range fRe.FindAllStringSubmatch(s, -1) {
				fm.Symbols = append(fm.Symbols, m[1])
			}
			for _, m := range cRe.FindAllStringSubmatch(s, -1) {
				fm.Symbols = append(fm.Symbols, m[1])
			}
		case ".py":
			fm.Lang = "py"
			b, _ := os.ReadFile(path)
			s := string(b)
			toks := wordRe.FindAllString(s, -1)
			fm.Tokens = normalizeTokens(toks)
			// imports: import x / from x import y
			impRe1 := regexp.MustCompile(`(?m)^\s*import\s+([A-Za-z0-9_\.]+)`)
			impRe2 := regexp.MustCompile(`(?m)^\s*from\s+([A-Za-z0-9_\.]+)\s+import\s+`)
			for _, m := range impRe1.FindAllStringSubmatch(s, -1) {
				fm.Imports = append(fm.Imports, m[1])
			}
			for _, m := range impRe2.FindAllStringSubmatch(s, -1) {
				fm.Imports = append(fm.Imports, m[1])
			}
			if len(fm.Imports) > 0 {
				if e.imports[fm.Path] == nil {
					e.imports[fm.Path] = map[string]struct{}{}
				}
				for _, p := range fm.Imports {
					e.imports[fm.Path][p] = struct{}{}
				}
			}
			// symbols: def name(...): / class Name:
			fRe := regexp.MustCompile(`(?m)^\s*def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
			cRe := regexp.MustCompile(`(?m)^\s*class\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
			for _, m := range fRe.FindAllStringSubmatch(s, -1) {
				fm.Symbols = append(fm.Symbols, m[1])
			}
			for _, m := range cRe.FindAllStringSubmatch(s, -1) {
				fm.Symbols = append(fm.Symbols, m[1])
			}
		default:
			fm.Lang = strings.TrimPrefix(ext, ".")
			b, _ := os.ReadFile(path)
			toks := wordRe.FindAllString(string(b), -1)
			fm.Tokens = normalizeTokens(toks)
		}
		e.Index[fm.Path] = fm
		// build inverted index
		seen := map[string]struct{}{}
		for _, t := range fm.Tokens {
			if e.inv[t] == nil {
				e.inv[t] = map[string]int{}
			}
			e.inv[t][fm.Path]++
			if _, ok := seen[t]; !ok {
				e.docFreq[t]++
				seen[t] = struct{}{}
			}
		}
		return nil
	})
	// reverse imports mapping (best-effort on package names)
	for src, pkgs := range e.imports {
		for pkg := range pkgs {
			for path := range e.Index {
				// heuristic: match by package path prefix
				if strings.HasPrefix(pkg, moduleGuess(path)) {
					if e.rimports[path] == nil {
						e.rimports[path] = map[string]struct{}{}
					}
					e.rimports[path][src] = struct{}{}
				}
			}
		}
	}
	e.lastBuild = now
	return nil
}

func normalizeTokens(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.ToLower(s)
		if len(s) < 3 {
			continue
		}
		if isStopWord(s) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func isStopWord(s string) bool {
	switch s {
	case "the", "and", "for", "with", "from", "this", "that", "have", "not", "using", "into", "true", "false":
		return true
	default:
		return false
	}
}

func (e *Engine) SaveIndex(path string) error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	bs, err := json.MarshalIndent(struct {
		Root    string               `json:"root"`
		Files   map[string]*FileMeta `json:"files"`
		Imports map[string][]string  `json:"imports"`
		RImps   map[string][]string  `json:"rimports"`
	}{Root: e.Root, Files: e.Index, Imports: e.exportMap(e.imports), RImps: e.exportMap(e.rimports)}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, bs, 0644)
}

func (e *Engine) LoadIndex(path string) error {
	bs, err := os.ReadFile(path)
	if err != nil {
		slog.Error("context.load_index.read_file.error",
			"path", path,
			"error", err)
		return err
	}
	var obj struct {
		Root    string               `json:"root"`
		Files   map[string]*FileMeta `json:"files"`
		Imports map[string][]string  `json:"imports"`
		RImps   map[string][]string  `json:"rimports"`
	}
	if err := json.Unmarshal(bs, &obj); err != nil {
		slog.Error("context.load_index.unmarshal.error",
			"path", path,
			"data_size", len(bs),
			"error", err)
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Root = obj.Root
	e.Index = obj.Files
	e.imports = e.importMap(obj.Imports)
	e.rimports = e.importMap(obj.RImps)
	// rebuild inverted index/docFreq
	e.inv = map[string]map[string]int{}
	e.docFreq = map[string]int{}
	for p, fm := range e.Index {
		seen := map[string]struct{}{}
		for _, t := range fm.Tokens {
			if e.inv[t] == nil {
				e.inv[t] = map[string]int{}
			}
			e.inv[t][p]++
			if _, ok := seen[t]; !ok {
				e.docFreq[t]++
				seen[t] = struct{}{}
			}
		}
	}
	return nil
}

func (e *Engine) exportMap(m map[string]map[string]struct{}) map[string][]string {
	out := map[string][]string{}
	for k, v := range m {
		var arr []string
		for x := range v {
			arr = append(arr, x)
		}
		sort.Strings(arr)
		out[k] = arr
	}
	return out
}

func (e *Engine) importMap(m map[string][]string) map[string]map[string]struct{} {
	out := map[string]map[string]struct{}{}
	for k, arr := range m {
		mm := map[string]struct{}{}
		for _, x := range arr {
			mm[x] = struct{}{}
		}
		out[k] = mm
	}
	return out
}
