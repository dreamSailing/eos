package impl

import (
	"context"
	"fmt"
	"strings"

	"github.com/dreamSailing/vb-coding/internal/toolapi"
	"github.com/dreamSailing/vb-coding/internal/tools"
)

type catalog struct {
	defs []toolapi.ToolDefinition
}

func newCatalog() toolapi.Catalog {
	src := tools.GetAllToolDefinitions()
	out := make([]toolapi.ToolDefinition, 0, len(src))
	for _, d := range src {
		params := map[string]toolapi.ParameterInfo{}
		for k, v := range d.Params {
			if v == nil {
				continue
			}
			params[k] = toolapi.ParameterInfo{
				Type:     strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", v.Type))),
				Required: v.Required,
				Desc:     v.Desc,
			}
		}
		examples := make([]toolapi.ToolExample, 0, len(d.Examples))
		for _, ex := range d.Examples {
			examples = append(examples, toolapi.ToolExample{
				Description: ex.Description,
				Input:       ex.Input,
			})
		}
		out = append(out, toolapi.ToolDefinition{
			Name:        d.Name,
			Description: d.Description,
			RiskLevel:   toRiskLevel(d.RiskLevel),
			Params:      params,
			Examples:    examples,
		})
	}
	return &catalog{defs: out}
}

func (c *catalog) List(_ context.Context) ([]toolapi.ToolDefinition, error) {
	if c == nil {
		return nil, nil
	}
	return append([]toolapi.ToolDefinition{}, c.defs...), nil
}

func (c *catalog) RiskLevel(toolName string) toolapi.RiskLevel {
	return toRiskLevel(tools.GetToolRiskLevel(strings.TrimSpace(toolName)))
}

func toRiskLevel(level tools.ToolRiskLevel) toolapi.RiskLevel {
	switch level {
	case tools.RiskLevelLow:
		return toolapi.RiskLow
	case tools.RiskLevelMedium:
		return toolapi.RiskMedium
	case tools.RiskLevelHigh:
		return toolapi.RiskHigh
	default:
		return toolapi.RiskLow
	}
}
