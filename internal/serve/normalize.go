package serve

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"path/filepath"
	"strings"
)

var pathKeys = map[string]bool{
	"path":        true,
	"file":        true,
	"source":      true,
	"destination": true,
	"working_dir": true,
	"root":        true,
}

func normalizePathsInParams(workspaceAbs string, params map[string]interface{}) (map[string]interface{}, error) {
	if params == nil {
		return map[string]interface{}{}, nil
	}
	outAny, err := normalizePathsAny(workspaceAbs, params)
	if err != nil {
		return nil, err
	}
	out, ok := outAny.(map[string]interface{})
	if !ok {
		return nil, &rpcError{Code: -32012, Message: "Internal"}
	}
	return out, nil
}

func normalizePathsAny(workspaceAbs string, v any) (any, error) {
	switch x := v.(type) {
	case map[string]interface{}:
		cp := make(map[string]interface{}, len(x))
		for k, vv := range x {
			key := strings.ToLower(strings.TrimSpace(k))
			if pathKeys[key] {
				if s, ok := vv.(string); ok {
					s = strings.TrimSpace(s)
					if s == "" {
						cp[k] = vv
						continue
					}
					abs, ok2, err := resolveInWorkspace(workspaceAbs, s)
					if err != nil {
						return nil, &rpcError{Code: -32005, Message: "InvalidParams", Data: map[string]any{"field": k}}
					}
					if !ok2 {
						return nil, &rpcError{Code: -32009, Message: "WorkspaceViolation", Data: map[string]any{"field": k, "path": s}}
					}
					cp[k] = filepath.Clean(abs)
					continue
				}
			}
			nv, err := normalizePathsAny(workspaceAbs, vv)
			if err != nil {
				return nil, err
			}
			cp[k] = nv
		}
		return cp, nil
	case []interface{}:
		arr := make([]interface{}, 0, len(x))
		for _, it := range x {
			nv, err := normalizePathsAny(workspaceAbs, it)
			if err != nil {
				return nil, err
			}
			arr = append(arr, nv)
		}
		return arr, nil
	default:
		return v, nil
	}
}

