package tools

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
