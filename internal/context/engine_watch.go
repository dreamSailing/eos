package codectx

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func (e *Engine) StartPoll(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = e.BuildIndex()
			}
		}
	}()
}

func (e *Engine) StartWatch(ctx context.Context) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("context.start_watch.new_watcher.error",
			"root", e.Root,
			"error", err)
		return err
	}
	root := e.Root
	errWalk := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := filepath.Base(path)
			lb := strings.ToLower(base)
			for _, ig := range e.ignorePatterns {
				if lb == ig {
					return filepath.SkipDir
				}
			}
			if err := w.Add(path); err != nil {
				slog.Warn("context.start_watch.add_dir.warn",
					"root", e.Root,
					"dir_path", path,
					"error", err)
			}
		}
		return nil
	})
	if errWalk != nil {
		slog.Warn("context.start_watch.walk_dir.warn",
			"root", e.Root,
			"error", errWalk)
	}
	debounce := e.debounceMs
	type evt struct {
		path string
		ts   time.Time
		op   fsnotify.Op
	}
	buf := make(chan evt, 256)
	go func() {
		defer func() { _ = w.Close() }()
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-w.Events:
				if ev.Name == "" {
					continue
				}
				name := filepath.Base(ev.Name)
				if strings.HasPrefix(name, ".") {
					continue
				}
				buf <- evt{path: ev.Name, ts: time.Now(), op: ev.Op}
				if (ev.Op & fsnotify.Create) != 0 {
					if fi, err := os.Stat(ev.Name); err == nil && fi.IsDir() {
						if err := w.Add(ev.Name); err != nil {
							slog.Warn("context.start_watch.add_new_dir.warn",
								"root", e.Root,
								"new_dir", ev.Name,
								"error", err)
						}
					}
				}
			case err := <-w.Errors:
				slog.Error("context.start_watch.fsnotify.error",
					"root", e.Root,
					"error", err)
			}
		}
	}()
	go func() {
		pending := map[string]evt{}
		ticker := time.NewTicker(time.Duration(debounce) * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case e2 := <-buf:
				pending[e2.path] = e2
			case <-ticker.C:
				if len(pending) == 0 {
					continue
				}
				snap := pending
				pending = map[string]evt{}
				for p, v := range snap {
					if (v.op&fsnotify.Remove) != 0 || (v.op&fsnotify.Rename) != 0 {
						e.RemoveFile(p)
					} else {
						e.UpdateFile(p)
					}
				}
				go func() {
					wd, _ := os.Getwd()
					idxp := filepath.Join(wd, ".eos", "index.json")
					_ = os.MkdirAll(filepath.Dir(idxp), 0755)
					_ = e.SaveIndex(idxp)
				}()
			}
		}
	}()
	return nil
}

func (e *Engine) UpdateFile(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	root := e.Root
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	rel := path
	if r, err := filepath.Rel(root, path); err == nil {
		rel = filepath.ToSlash(r)
	}
	if old, ok := e.Index[rel]; ok {
		for _, t := range old.Tokens {
			if m := e.inv[t]; m != nil {
				if c, ok2 := m[rel]; ok2 {
					if c <= 1 {
						delete(m, rel)
					} else {
						m[rel] = c - 1
					}
				}
			}
		}
		delete(e.Index, rel)
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return
	}
	ext := strings.ToLower(filepath.Ext(path))
	if info.Size() > 2*1024*1024 {
		return
	}
	fset := token.NewFileSet()
	wordRe := regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]{2,}`)
	fm := &FileMeta{Path: rel, Size: int(info.Size()), MTime: info.ModTime().Unix(), Lang: strings.TrimPrefix(ext, ".")}
	switch ext {
	case ".go":
		fm.Lang = "go"
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err == nil && file != nil {
			for _, im := range file.Imports {
				s := strings.Trim(strings.TrimSpace(im.Path.Value), `"`)
				if s != "" {
					fm.Imports = append(fm.Imports, s)
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
		b, _ := os.ReadFile(path)
		fm.Tokens = normalizeTokens(wordRe.FindAllString(string(b), -1))
	case ".ts", ".tsx", ".js", ".jsx":
		fm.Lang = "ts"
		b, _ := os.ReadFile(path)
		s := string(b)
		fm.Tokens = normalizeTokens(wordRe.FindAllString(s, -1))
		impRe1 := regexp.MustCompile(`(?m)import\s+[^;]*?from\s+['\"]([^'\"]+)['\"]`)
		impRe2 := regexp.MustCompile(`(?m)require\(\s*['\"]([^'\"]+)['\"]\s*\)`)
		for _, m := range impRe1.FindAllStringSubmatch(s, -1) {
			fm.Imports = append(fm.Imports, m[1])
		}
		for _, m := range impRe2.FindAllStringSubmatch(s, -1) {
			fm.Imports = append(fm.Imports, m[1])
		}
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
		fm.Tokens = normalizeTokens(wordRe.FindAllString(s, -1))
		impRe1 := regexp.MustCompile(`(?m)^\s*import\s+([A-Za-z0-9_\.]+)`)
		impRe2 := regexp.MustCompile(`(?m)^\s*from\s+([A-Za-z0-9_\.]+)\s+import\s+`)
		for _, m := range impRe1.FindAllStringSubmatch(s, -1) {
			fm.Imports = append(fm.Imports, m[1])
		}
		for _, m := range impRe2.FindAllStringSubmatch(s, -1) {
			fm.Imports = append(fm.Imports, m[1])
		}
		fRe := regexp.MustCompile(`(?m)^\s*def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
		cRe := regexp.MustCompile(`(?m)^\s*class\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
		for _, m := range fRe.FindAllStringSubmatch(s, -1) {
			fm.Symbols = append(fm.Symbols, m[1])
		}
		for _, m := range cRe.FindAllStringSubmatch(s, -1) {
			fm.Symbols = append(fm.Symbols, m[1])
		}
	default:
		b, _ := os.ReadFile(path)
		fm.Tokens = normalizeTokens(wordRe.FindAllString(string(b), -1))
	}
	e.Index[rel] = fm
	seen := map[string]struct{}{}
	for _, t := range fm.Tokens {
		if e.inv[t] == nil {
			e.inv[t] = map[string]int{}
		}
		e.inv[t][rel]++
		if _, ok := seen[t]; !ok {
			e.docFreq[t]++
			seen[t] = struct{}{}
		}
	}
	e.imports[rel] = map[string]struct{}{}
	for _, s := range fm.Imports {
		e.imports[rel][s] = struct{}{}
	}
	for pkg := range e.imports[rel] {
		for path := range e.Index {
			if strings.HasPrefix(pkg, moduleGuess(path)) {
				if e.rimports[path] == nil {
					e.rimports[path] = map[string]struct{}{}
				}
				e.rimports[path][rel] = struct{}{}
			}
		}
	}
}

func (e *Engine) RemoveFile(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	root := e.Root
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	rel := path
	if r, err := filepath.Rel(root, path); err == nil {
		rel = filepath.ToSlash(r)
	}
	old, ok := e.Index[rel]
	if !ok {
		return
	}
	for _, t := range old.Tokens {
		if m := e.inv[t]; m != nil {
			delete(m, rel)
		}
	}
	delete(e.Index, rel)
	delete(e.imports, rel)
	for k := range e.rimports {
		delete(e.rimports[k], rel)
	}
}

func (e *Engine) SetDebounce(ms int) {
	e.mu.Lock()
	e.debounceMs = ms
	e.mu.Unlock()
}

func (e *Engine) Debounce() int {
	e.mu.RLock()
	d := e.debounceMs
	e.mu.RUnlock()
	return d
}
