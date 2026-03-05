package runtime

import (
	"context"
	"fmt"
	"strings"
	"github.com/dreamSailing/vb-coding/internal/tools"

	"github.com/cloudwego/eino/components/tool"
)

// toolCategory 工具分类
type toolCategory struct {
	Name     string   // 分类名称
	ToolKeys []string // 该分类包含的工具名（按 definitions.go 中的常量）
}

// getToolCategories 返回工具分类列表
func getToolCategories() []toolCategory {
	return []toolCategory{
		{
			Name:     "文件操作",
			ToolKeys: []string{tools.ToolRead, tools.ToolFS, tools.ToolEdit},
		},
		{
			Name:     "搜索",
			ToolKeys: []string{tools.ToolSearch, tools.ToolToolSearch, tools.ToolProjectStructure},
		},
		{
			Name:     "命令执行",
			ToolKeys: []string{tools.ToolBash, tools.ToolBashSession},
		},
		{
			Name:     "Git 版本控制",
			ToolKeys: []string{tools.ToolGitStatus, tools.ToolGitDiff, tools.ToolGitLog, tools.ToolGitAdd, tools.ToolGitCommit, tools.ToolGitBranchList, tools.ToolGitCheckout, tools.ToolGitInit, tools.ToolGitPull, tools.ToolGitPush, tools.ToolGitStash, tools.ToolGitReset, tools.ToolGitRevert, tools.ToolGitMerge, tools.ToolGitRebase},
		},
		{
			Name:     "规划与任务",
			ToolKeys: []string{tools.ToolPlanSteps, tools.ToolTodoRead, tools.ToolTodoWrite},
		},
		{
			Name:     "交互",
			ToolKeys: []string{tools.ToolUserConfirm, tools.ToolUserInput, tools.ToolUserSelect},
		},
		{
			Name:     "技能",
			ToolKeys: []string{tools.ToolSkill},
		},
	}
}

// GetAvailableToolsDescription generates a detailed, structured description of available tools
// by reading from the centralized tool definitions in definitions.go.
func GetAvailableToolsDescription(ctx context.Context, mcpTools []tool.BaseTool) string {
	var sb strings.Builder
	allDefs := tools.GetAllToolDefinitions()
	defMap := make(map[string]tools.ToolDefinition, len(allDefs))
	for _, d := range allDefs {
		defMap[d.Name] = d
	}

	categories := getToolCategories()
	for _, cat := range categories {
		sb.WriteString(fmt.Sprintf("**%s**：\n", cat.Name))
		for _, toolName := range cat.ToolKeys {
			def, ok := defMap[toolName]
			if !ok {
				continue
			}
			// Tool name + description
			sb.WriteString(fmt.Sprintf("- `%s` — %s\n", def.Name, def.Description))

			// Key parameters (required first, then optional, skip obvious ones)
			if len(def.Params) > 0 {
				var required, optional []string
				for pName, pInfo := range def.Params {
					desc := pName
					if pInfo.Desc != "" {
						// 取描述的第一句（到第一个句号或括号前）
						short := pInfo.Desc
						if idx := strings.IndexAny(short, "。(（"); idx > 0 && idx < 60 {
							short = short[:idx]
						}
						if len(short) > 50 {
							short = short[:50] + "…"
						}
						desc = fmt.Sprintf("%s: %s", pName, short)
					}
					if pInfo.Required {
						required = append(required, desc)
					} else {
						optional = append(optional, desc)
					}
				}
				parts := append(required, optional...)
				if len(parts) > 5 {
					parts = parts[:5]
					parts = append(parts, "...")
				}
				sb.WriteString(fmt.Sprintf("  参数: %s\n", strings.Join(parts, ", ")))
			}

			// First example (if available)
			if len(def.Examples) > 0 {
				ex := def.Examples[0]
				sb.WriteString(fmt.Sprintf("  示例: %s %s\n", def.Name, formatExampleParams(ex.Input)))
			}
		}
		sb.WriteString("\n")
	}

	// MCP extension tools
	if len(mcpTools) > 0 {
		sb.WriteString("**MCP 扩展工具**：\n")
		for _, t := range mcpTools {
			info, err := t.Info(ctx)
			if err == nil && info != nil {
				sb.WriteString(fmt.Sprintf("- `%s` — %s\n", info.Name, info.Desc))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatExampleParams 将示例参数格式化为可读字符串
func formatExampleParams(params map[string]interface{}) string {
	if len(params) == 0 {
		return "{}"
	}
	var parts []string
	for k, v := range params {
		switch val := v.(type) {
		case string:
			// 截断过长的内容字符串
			s := val
			if len(s) > 40 {
				s = s[:37] + "..."
			}
			parts = append(parts, fmt.Sprintf(`%s: "%s"`, k, s))
		case bool:
			parts = append(parts, fmt.Sprintf(`%s: %v`, k, val))
		case []string:
			parts = append(parts, fmt.Sprintf(`%s: [%s]`, k, strings.Join(val, ", ")))
		default:
			parts = append(parts, fmt.Sprintf(`%s: %v`, k, val))
		}
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// GetToolNamesForDisplay returns a list of all built-in tool names
func GetToolNamesForDisplay() []string {
	defs := tools.GetAllToolDefinitions()
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	return names
}
