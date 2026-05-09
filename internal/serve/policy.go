package serve

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Policy struct {
	ID    string       `json:"id"`
	Rules []PolicyRule `json:"rules"`
}

type PolicyRule struct {
	ID         string                 `json:"id"`
	Tool       string                 `json:"tool"`
	RiskLevel  string                 `json:"riskLevel"`
	Decision   string                 `json:"decision"`
	Constraints map[string]interface{} `json:"constraints"`
}

func LoadPolicy(path string) (*Policy, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Policy
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *Policy) FindRule(tool string, riskLevel string) *PolicyRule {
	if p == nil {
		return nil
	}
	tool = strings.ToLower(strings.TrimSpace(tool))
	riskLevel = strings.ToLower(strings.TrimSpace(riskLevel))
	for i := range p.Rules {
		r := &p.Rules[i]
		if strings.ToLower(strings.TrimSpace(r.Tool)) != tool {
			continue
		}
		if strings.ToLower(strings.TrimSpace(r.RiskLevel)) != "" && strings.ToLower(strings.TrimSpace(r.RiskLevel)) != riskLevel {
			continue
		}
		return r
	}
	return nil
}

func (r *PolicyRule) AllowedCommands() []string {
	if r == nil {
		return nil
	}
	raw, ok := r.Constraints["allowedCommands"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, it := range arr {
		s, ok := it.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func (r *PolicyRule) DenyPathGlobs() []string {
	if r == nil {
		return nil
	}
	raw, ok := r.Constraints["denyPathGlobs"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, it := range arr {
		s, ok := it.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, filepath.ToSlash(s))
	}
	return out
}

