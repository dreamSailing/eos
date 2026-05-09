package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"fmt"
	"strings"

	"github.com/dreamSailing/eos/internal/tools/shell"
)

// toolSearchStructured handles tool search for dynamic tool discovery
func (m *Manager) toolSearchStructured(ctx context.Context, params map[string]any) ToolResult {
	query, _ := params["query"].(string)
	category, _ := params["category"].(string)
	riskLevelStr, _ := params["risk_level"].(string)
	limit := toInt(params["limit"], 10)

	var result *ToolSearchResult

	if category != "" {
		matches := m.toolIndex.GetByCategory(category)
		result = &ToolSearchResult{
			Query:   "category:" + category,
			Matches: matches,
			Total:   len(matches),
		}
	} else if riskLevelStr != "" {
		var level ToolRiskLevel
		switch riskLevelStr {
		case "low":
			level = RiskLevelLow
		case "medium":
			level = RiskLevelMedium
		case "high":
			level = RiskLevelHigh
		default:
			return ToolResult{
				Type:   "tool_result",
				Tool:   "tool_search",
				Status: "error",
				Error:  "invalid risk_level: use low, medium, or high",
			}
		}
		matches := m.toolIndex.GetByRiskLevel(level)
		result = &ToolSearchResult{
			Query:   "risk_level:" + riskLevelStr,
			Matches: matches,
			Total:   len(matches),
		}
	} else {
		result = m.toolIndex.Search(query)
	}

	if limit > 0 && len(result.Matches) > limit {
		result.Matches = result.Matches[:limit]
		result.Total = limit
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("工具搜索结果 (查询: %s, 匹配: %d)\n\n", result.Query, result.Total))

	for i, match := range result.Matches {
		output.WriteString(fmt.Sprintf("%d. `%s` - %s\n", i+1, match.Name, match.Description))
		if match.MatchReason != "" {
			output.WriteString(fmt.Sprintf("   匹配原因: %s, 相关度: %.0f%%\n", match.MatchReason, match.Score*100))
		}
	}

	return ToolResult{
		Type:   "tool_result",
		Tool:   "tool_search",
		Status: "success",
		Data: map[string]any{
			"query":   result.Query,
			"matches": result.Matches,
			"total":   result.Total,
			"results": output.String(),
		},
		Display: fmt.Sprintf("找到 %d 个工具", result.Total),
	}
}

// GetToolIndex returns the tool index
func (m *Manager) GetToolIndex() *ToolIndex {
	return m.toolIndex
}

// GetToolIndexStats returns tool index statistics
func (m *Manager) GetToolIndexStats() map[string]any {
	return m.toolIndex.GetStats()
}

// ExecuteBashDirect executes bash command bypassing tool call mechanism
func (m *Manager) ExecuteBashDirect(ctx context.Context, cmd string) (string, error) {
	return m.shell.ExecuteTypedWithWorkingDirCtx(ctx, shell.ShellTypeBash, cmd, WorkspaceRootFromContext(ctx))
}
