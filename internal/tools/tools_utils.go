package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"path/filepath"
	"github.com/dreamSailing/eos/internal/search"
)

func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	var out []string
	switch a := v.(type) {
	case []interface{}:
		for _, x := range a {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	case []string:
		out = append(out, a...)
	}
	return out
}

func toInt(v interface{}, def int) int {
	if v == nil {
		return def
	}
	switch n := v.(type) {
	case float64:
		if int(n) > 0 {
			return int(n)
		}
	case int:
		if n > 0 {
			return n
		}
	}
	return def
}

func toInt64(v interface{}, def int64) int64 {
	if v == nil {
		return def
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	}
	return def
}

func mapResults(rs []search.Result) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(rs))
	for _, r := range rs {
		out = append(out, map[string]interface{}{
			"file":   filepath.ToSlash(r.File),
			"line":   r.Line,
			"column": r.Column,
			"match":  r.Match,
			"groups": r.Groups,
			"before": r.Before,
			"after":  r.After,
		})
	}
	return out
}
