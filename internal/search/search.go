package search

// Copyright (c) 2026 EOSAIOS
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"bufio"
	"errors"
	"github.com/eosaios/eos/internal/pkg/utils"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type Result struct {
	File   string   `json:"file"`
	Line   int      `json:"line"`
	Column int      `json:"column"`
	Match  string   `json:"match"`
	Groups []string `json:"groups"`
	Before []string `json:"before"`
	After  []string `json:"after"`
}

type Options struct {
	Root            string
	Includes        []string
	Excludes        []string
	Limit           int
	ContextLines    int
	Flags           string
	CaseInsensitive bool
	MaxFileSize     int64
}

func DirRegex(pattern string, opts Options) ([]Result, bool, error) {
	re, err := compileRegex(pattern, opts.Flags)
	if err != nil {
		return nil, false, err
	}
	return dirSearch(func(path string, data []byte) []Result {
		return findRegexInContent(path, data, re, opts.ContextLines)
	}, opts)
}

func DirText(pattern string, opts Options) ([]Result, bool, error) {
	text := pattern
	if opts.CaseInsensitive {
		text = strings.ToLower(text)
	}
	return dirSearch(func(path string, data []byte) []Result {
		return findTextInContent(path, data, text, opts.CaseInsensitive, opts.ContextLines)
	}, opts)
}

func FileRegex(file string, pattern string, opts Options) ([]Result, bool, error) {
	re, err := compileRegex(pattern, opts.Flags)
	if err != nil {
		return nil, false, err
	}
	return fileSearch(file, func(path string, data []byte) []Result {
		return findRegexInContent(path, data, re, opts.ContextLines)
	}, opts)
}

func FileText(file string, pattern string, opts Options) ([]Result, bool, error) {
	text := pattern
	return fileSearch(file, func(path string, data []byte) []Result {
		return findTextInContent(path, data, text, opts.CaseInsensitive, opts.ContextLines)
	}, opts)
}

func compileRegex(pattern string, flags string) (*regexp.Regexp, error) {
	prefix := ""
	if flags != "" {
		var f []rune
		for _, r := range flags {
			switch r {
			case 'i':
				f = append(f, 'i')
			case 'm':
				f = append(f, 'm')
			case 's':
				f = append(f, 's')
			}
		}
		if len(f) > 0 {
			prefix = "(?" + string(f) + ")"
		}
	}
	return regexp.Compile(prefix + pattern)
}

func dirSearch(worker func(string, []byte) []Result, opts Options) ([]Result, bool, error) {
	root := opts.Root
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, false, err
		}
	}
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldExclude(path, opts.Excludes) {
				return filepath.SkipDir
			}
			return nil
		}
		if shouldExclude(path, opts.Excludes) {
			return nil
		}
		if len(opts.Includes) > 0 && !shouldInclude(path, opts.Includes) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	workers := 16
	if workers < 1 {
		workers = 1
	}
	type job struct{ path string }
	jobs := make(chan job, workers)
	var mu sync.Mutex
	var results []Result
	var truncated bool
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				data, skip := readFileBounded(j.path, opts.MaxFileSize)
				if skip {
					continue
				}
				rs := worker(j.path, data)
				if len(rs) == 0 {
					continue
				}
				mu.Lock()
				if opts.Limit > 0 && len(results) >= opts.Limit {
					truncated = true
					mu.Unlock()
					continue
				}
				for _, r := range rs {
					if opts.Limit > 0 && len(results) >= opts.Limit {
						truncated = true
						break
					}
					results = append(results, r)
				}
				mu.Unlock()
			}
		}()
	}
	for _, f := range files {
		jobs <- job{path: f}
	}
	close(jobs)
	wg.Wait()
	sort.Slice(results, func(i, j int) bool {
		if results[i].File == results[j].File {
			if results[i].Line == results[j].Line {
				return results[i].Column < results[j].Column
			}
			return results[i].Line < results[j].Line
		}
		return results[i].File < results[j].File
	})
	return results, truncated, nil
}

func fileSearch(file string, worker func(string, []byte) []Result, opts Options) ([]Result, bool, error) {
	if file == "" {
		return nil, false, errors.New("file required")
	}
	data, skip := readFileBounded(file, opts.MaxFileSize)
	if skip {
		return nil, false, nil
	}
	rs := worker(file, data)
	return rs, false, nil
}

func readFileBounded(path string, maxSize int64) ([]byte, bool) {
	// 使用统一的验证逻辑
	limit := maxSize
	if limit <= 0 {
		limit = 10 * 1024 * 1024 // 默认 10MB
	}
	valid, _, _ := utils.ValidateFileForRead(path, limit)
	if !valid {
		return nil, true
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, true
	}
	return data, false
}

func findRegexInContent(path string, data []byte, re *regexp.Regexp, ctx int) []Result {
	text := string(data)
	idxs := re.FindAllStringSubmatchIndex(text, -1)
	if len(idxs) == 0 {
		return nil
	}
	lines := splitLines(text)
	lineOffsets := make([]int, len(lines)+1)
	off := 0
	for i, ln := range lines {
		lineOffsets[i] = off
		off += len(ln)
	}
	lineOffsets[len(lines)] = off
	var out []Result
	for _, m := range idxs {
		if len(m) < 2 {
			continue
		}
		start := m[0]
		end := m[1]
		li := findLineIndex(lineOffsets, start)
		col := start - lineOffsets[li]
		match := text[start:end]
		var groups []string
		for g := 2; g < len(m); g += 2 {
			s := m[g]
			e := m[g+1]
			if s >= 0 && e >= 0 {
				groups = append(groups, text[s:e])
			} else {
				groups = append(groups, "")
			}
		}
		before := collectContext(lines, li, -1, ctx)
		after := collectContext(lines, li, 1, ctx)
		out = append(out, Result{File: path, Line: li + 1, Column: col + 1, Match: match, Groups: groups, Before: before, After: after})
	}
	return out
}

func findTextInContent(path string, data []byte, pattern string, ci bool, ctx int) []Result {
	var out []Result
	s := string(data)
	lines := splitLines(s)
	for i, ln := range lines {
		line := ln
		hay := ln
		needle := pattern
		if ci {
			hay = strings.ToLower(ln)
			needle = strings.ToLower(pattern)
		}
		pos := 0
		for {
			j := strings.Index(hay[pos:], needle)
			if j < 0 {
				break
			}
			col := pos + j
			before := collectContext(lines, i, -1, ctx)
			after := collectContext(lines, i, 1, ctx)
			out = append(out, Result{File: path, Line: i + 1, Column: col + 1, Match: line[col : col+len(pattern)], Groups: nil, Before: before, After: after})
			pos = col + len(pattern)
		}
	}
	return out
}

func splitLines(s string) []string {
	var lines []string
	r := bufio.NewReader(strings.NewReader(s))
	for {
		l, err := r.ReadString('\n')
		lines = append(lines, l)
		if err != nil {
			if err == io.EOF {
				break
			}
			break
		}
	}
	return lines
}

func findLineIndex(lineOffsets []int, pos int) int {
	lo := 0
	hi := len(lineOffsets) - 1
	for lo < hi {
		mid := (lo + hi) / 2
		if lineOffsets[mid] <= pos && pos < lineOffsets[mid+1] {
			return mid
		}
		if pos < lineOffsets[mid] {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	if lo >= len(lineOffsets)-1 {
		return len(lineOffsets) - 2
	}
	return lo
}

func collectContext(lines []string, idx int, dir int, n int) []string {
	if n <= 0 {
		return nil
	}
	var out []string
	if dir < 0 {
		for i := 1; i <= n; i++ {
			j := idx - i
			if j < 0 {
				break
			}
			out = append(out, strings.TrimRight(lines[j], "\n"))
		}
		reverse(out)
		return out
	}
	for i := 1; i <= n; i++ {
		j := idx + i
		if j >= len(lines) {
			break
		}
		out = append(out, strings.TrimRight(lines[j], "\n"))
	}
	return out
}

func reverse(a []string) {
	i := 0
	j := len(a) - 1
	for i < j {
		a[i], a[j] = a[j], a[i]
		i++
		j--
	}
}

func shouldExclude(path string, excludes []string) bool {
	if len(excludes) == 0 {
		return false
	}
	p := filepath.ToSlash(path)
	for _, ex := range excludes {
		if ex == "" {
			continue
		}
		if strings.Contains(p, ex) {
			return true
		}
	}
	return false
}

func shouldInclude(path string, includes []string) bool {
	if len(includes) == 0 {
		return true
	}
	p := filepath.ToSlash(path)
	for _, in := range includes {
		if in == "" {
			continue
		}
		if strings.Contains(p, in) {
			return true
		}
	}
	return false
}
