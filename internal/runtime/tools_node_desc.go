package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"github.com/dreamSailing/eos/internal/tools"
	"sort"
	"strings"

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
			ToolKeys: []string{tools.ToolGitStatus, tools.ToolGitDiff, tools.ToolGitLog, tools.ToolGitShow, tools.ToolGitAdd, tools.ToolGitCommit, tools.ToolGitBranchList, tools.ToolGitCheckout, tools.ToolGitInit, tools.ToolGitPull, tools.ToolGitPush, tools.ToolGitStash, tools.ToolGitReset, tools.ToolGitRevert, tools.ToolGitMerge, tools.ToolGitRebase},
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
			Name: "浏览器",
			ToolKeys: []string{
				tools.ToolBrowserStatus, tools.ToolBrowserNavigate, tools.ToolBrowserSnapshot, tools.ToolBrowserInspect, tools.ToolBrowserTabs,
				tools.ToolBrowserBack, tools.ToolBrowserForward, tools.ToolBrowserReload, tools.ToolBrowserClick, tools.ToolBrowserHover,
				tools.ToolBrowserType, tools.ToolBrowserPressKey, tools.ToolBrowserSelect, tools.ToolBrowserWait, tools.ToolBrowserScroll,
				tools.ToolBrowserScreenshot, tools.ToolBrowserConsole, tools.ToolBrowserNetwork, tools.ToolBrowserViewport, tools.ToolBrowserVisibility,
				tools.ToolBrowserClipboard, tools.ToolBrowserCUA, tools.ToolBrowserDOMCUA, tools.ToolBrowserLocator, tools.ToolBrowserDevLogs,
				tools.ToolBrowserDownloads, tools.ToolBrowserUserTabs, tools.ToolBrowserSessionName,
			},
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

// GetDispatchToolsDescription returns the actual invocable tools available to the
// dispatch agent, rather than the full runtime tool catalog.
func GetDispatchToolsDescription() string {
	var sb strings.Builder
	sb.WriteString("**调度工具**：\n")
	for _, ti := range GetDispatchToolsInfo() {
		name, _ := ti["name"].(string)
		desc, _ := ti["description"].(string)
		if strings.TrimSpace(name) == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("- `%s` — %s\n", name, desc))
		params, _ := ti["parameters"].(map[string]any)
		sb.WriteString(formatDispatchToolParams(params))
	}
	sb.WriteString("\n")
	return sb.String()
}

func formatDispatchToolParams(params map[string]any) string {
	if len(params) == 0 {
		return ""
	}
	props, _ := params["properties"].(map[string]any)
	if len(props) == 0 {
		return ""
	}
	requiredSet := map[string]bool{}
	switch req := params["required"].(type) {
	case []string:
		for _, key := range req {
			requiredSet[key] = true
		}
	case []any:
		for _, item := range req {
			if key, ok := item.(string); ok {
				requiredSet[key] = true
			}
		}
	}

	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	// Keep output stable for tests and prompt caching.
	sort.Strings(names)

	parts := make([]string, 0, len(names))
	for _, name := range names {
		prop, _ := props[name].(map[string]any)
		desc, _ := prop["description"].(string)
		part := name
		if strings.TrimSpace(desc) != "" {
			short := desc
			if idx := strings.IndexAny(short, "。；;（("); idx > 0 && idx < 60 {
				short = short[:idx]
			}
			if len(short) > 50 {
				short = short[:50] + "..."
			}
			part = fmt.Sprintf("%s: %s", name, short)
		}
		if requiredSet[name] {
			part += " [required]"
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return ""
	}
	return "  参数: " + strings.Join(parts, ", ") + "\n"
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
