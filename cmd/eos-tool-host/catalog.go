//go:build legacy

package main

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/cloudwego/eino/schema"

	"github.com/dreamSailing/eos/internal/tools"
	"github.com/dreamSailing/eos/pkg/coreapi/sidecar/toolhost"
)

func (r *managerRunner) ListTools(_ context.Context, req toolhost.CatalogRequest) ([]toolhost.ToolDefinition, error) {
	defs := tools.GetAllToolDefinitions()
	out := make([]toolhost.ToolDefinition, 0, len(defs))
	for _, def := range defs {
		out = append(out, convertToolDefinition(def))
	}
	return filterCatalog(out, req), nil
}

func convertToolDefinition(def tools.ToolDefinition) toolhost.ToolDefinition {
	params, paramsSchema := convertParams(def.Params)
	tags := make([]string, 0, 2)
	if def.ConcurrencySafe {
		tags = append(tags, "concurrency-safe")
	}
	if def.NeedsSandboxRunner {
		tags = append(tags, "needs-sandbox-runner")
	}

	return toolhost.ToolDefinition{
		Name:         def.Name,
		Description:  def.Description,
		RiskLevel:    riskLevelString(def.RiskLevel),
		Params:       params,
		Examples:     convertExamples(def.Examples),
		Source:       "go-legacy",
		Category:     def.Category,
		ReadOnly:     def.ReadOnly || def.RiskLevel == tools.RiskLevelLow,
		Invocable:    true,
		Tags:         tags,
		ParamsSchema: paramsSchema,
		Metadata: map[string]any{
			"concurrency_safe":     def.ConcurrencySafe,
			"needs_sandbox_runner": def.NeedsSandboxRunner,
		},
	}
}

func convertExamples(examples []tools.ToolExample) []toolhost.ToolExample {
	if len(examples) == 0 {
		return nil
	}
	out := make([]toolhost.ToolExample, 0, len(examples))
	for _, ex := range examples {
		out = append(out, toolhost.ToolExample{
			Description: ex.Description,
			Input:       ex.Input,
		})
	}
	return out
}

func convertParams(params map[string]*schema.ParameterInfo) (map[string]toolhost.ToolParameterInfo, json.RawMessage) {
	if len(params) == 0 {
		return nil, nil
	}

	dto := make(map[string]toolhost.ToolParameterInfo, len(params))
	properties := make(map[string]any, len(params))
	var required []string
	for name, param := range params {
		if param == nil {
			continue
		}
		dto[name] = toolhost.ToolParameterInfo{
			Type:     string(param.Type),
			Required: param.Required,
			Desc:     param.Desc,
		}
		properties[name] = parameterSchema(param)
		if param.Required {
			required = append(required, name)
		}
	}
	sort.Strings(required)

	root := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		root["required"] = required
	}

	raw, err := json.Marshal(root)
	if err != nil {
		return dto, nil
	}
	return dto, json.RawMessage(raw)
}

func parameterSchema(param *schema.ParameterInfo) map[string]any {
	out := map[string]any{
		"type": string(param.Type),
	}
	if param.Desc != "" {
		out["description"] = param.Desc
	}
	if len(param.Enum) > 0 {
		out["enum"] = append([]string(nil), param.Enum...)
	}
	if param.Type == schema.Array && param.ElemInfo != nil {
		out["items"] = parameterSchema(param.ElemInfo)
	}
	if param.Type == schema.Object && len(param.SubParams) > 0 {
		properties := make(map[string]any, len(param.SubParams))
		var required []string
		for name, child := range param.SubParams {
			if child == nil {
				continue
			}
			properties[name] = parameterSchema(child)
			if child.Required {
				required = append(required, name)
			}
		}
		sort.Strings(required)
		out["properties"] = properties
		if len(required) > 0 {
			out["required"] = required
		}
	}
	return out
}

func riskLevelString(level tools.ToolRiskLevel) string {
	switch level {
	case tools.RiskLevelLow:
		return "low"
	case tools.RiskLevelMedium:
		return "medium"
	case tools.RiskLevelHigh:
		return "high"
	default:
		return "high"
	}
}

func filterCatalog(defs []toolhost.ToolDefinition, req toolhost.CatalogRequest) []toolhost.ToolDefinition {
	include := makeSet(req.IncludeTools)
	allowed := makeSet(req.AllowedTools)
	out := make([]toolhost.ToolDefinition, 0, len(defs))
	for _, def := range defs {
		if len(include) > 0 {
			if _, ok := include[def.Name]; !ok {
				continue
			}
		}
		if len(allowed) > 0 {
			if _, ok := allowed[def.Name]; !ok {
				continue
			}
		}
		out = append(out, def)
	}
	return out
}

func makeSet(items []string) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item != "" {
			out[item] = struct{}{}
		}
	}
	return out
}

var _ toolhost.ToolCatalogRunner = (*managerRunner)(nil)
