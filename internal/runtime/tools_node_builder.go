package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"github.com/dreamSailing/eos/internal/tools"
	"strings"
	"time"

	duckduckgo "github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2"
	httpdelete "github.com/cloudwego/eino-ext/components/tool/httprequest/delete"
	httpget "github.com/cloudwego/eino-ext/components/tool/httprequest/get"
	httppost "github.com/cloudwego/eino-ext/components/tool/httprequest/post"
	httpput "github.com/cloudwego/eino-ext/components/tool/httprequest/put"
	"github.com/cloudwego/eino-ext/components/tool/wikipedia"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

func buildRuntimeParams(m map[string]*schema.ParameterInfo) *schema.ParamsOneOf {
	if m == nil {
		return nil
	}
	return schema.NewParamsOneOfByParams(m)
}

func buildDuckDuckGoSearchTool(ctx context.Context) tool.BaseTool {
	if ctx == nil {
		ctx = context.Background()
	}
	t, err := duckduckgo.NewTextSearchTool(ctx, &duckduckgo.Config{
		ToolName:   "duckduckgo_search",
		ToolDesc:   "通过 DuckDuckGo 搜索信息",
		Region:     duckduckgo.RegionWT,
		MaxResults: 3,
	})
	if err != nil {
		LogError("runtime.tools_node.duckduckgo.init_failed", "err", err)
		return nil
	}
	return t
}

func buildWikipediaSearchTool(ctx context.Context) tool.BaseTool {
	if ctx == nil {
		ctx = context.Background()
	}
	t, err := wikipedia.NewTool(ctx, &wikipedia.Config{
		ToolName:    "wikipedia_search",
		ToolDesc:    "通过 Wikipedia 查询百科信息",
		Language:    "zh",
		TopK:        3,
		Timeout:     15 * time.Second,
		MaxRedirect: 3,
		DocMaxChars: 2000,
		UserAgent:   "eos (https://github.com/cloudwego/eino)",
	})
	if err != nil {
		LogError("runtime.tools_node.wikipedia.init_failed", "err", err)
		return nil
	}
	return t
}

func buildHTTPGetTool(ctx context.Context) tool.BaseTool {
	if ctx == nil {
		ctx = context.Background()
	}
	t, err := httpget.NewTool(ctx, &httpget.Config{
		ToolName: "http_get",
		ToolDesc: "通过 HTTP GET 获取指定 URL 的文本响应内容",
		Headers: map[string]string{
			"User-Agent": "eos (https://github.com/cloudwego/eino)",
		},
	})
	if err != nil {
		LogError("runtime.tools_node.http_get.init_failed", "err", err)
		return nil
	}
	return t
}

func buildHTTPPostTool(ctx context.Context) tool.BaseTool {
	if ctx == nil {
		ctx = context.Background()
	}
	t, err := httppost.NewTool(ctx, &httppost.Config{
		ToolName: "http_post",
		ToolDesc: "通过 HTTP POST 向指定 URL 发送请求并返回响应内容",
		Headers: map[string]string{
			"User-Agent": "eos (https://github.com/cloudwego/eino)",
		},
	})
	if err != nil {
		LogError("runtime.tools_node.http_post.init_failed", "err", err)
		return nil
	}
	return t
}

func buildHTTPPutTool(ctx context.Context) tool.BaseTool {
	if ctx == nil {
		ctx = context.Background()
	}
	t, err := httpput.NewTool(ctx, &httpput.Config{
		ToolName: "http_put",
		ToolDesc: "通过 HTTP PUT 向指定 URL 发送请求并返回响应内容",
		Headers: map[string]string{
			"User-Agent": "eos (https://github.com/cloudwego/eino)",
		},
	})
	if err != nil {
		LogError("runtime.tools_node.http_put.init_failed", "err", err)
		return nil
	}
	return t
}

func buildHTTPDeleteTool(ctx context.Context) tool.BaseTool {
	if ctx == nil {
		ctx = context.Background()
	}
	t, err := httpdelete.NewTool(ctx, &httpdelete.Config{
		ToolName: "http_delete",
		ToolDesc: "通过 HTTP DELETE 向指定 URL 发送请求并返回响应内容",
		Headers: map[string]string{
			"User-Agent": "eos (https://github.com/cloudwego/eino)",
		},
	})
	if err != nil {
		LogError("runtime.tools_node.http_delete.init_failed", "err", err)
		return nil
	}
	return t
}

func BuildRuntimeTools(ctx context.Context, mgr *tools.Manager, detector LoopDetector) []tool.BaseTool {
	_ = detector
	return BuildRuntimeToolsWithMCP(ctx, mgr, nil, nil)
}

func BuildRuntimeToolsWithMCP(ctx context.Context, mgr *tools.Manager, dt *DispatchTools, mcpTools []tool.BaseTool) []tool.BaseTool {
	wrap := func(name, desc string, params *schema.ParamsOneOf) tool.BaseTool {
		var det LoopDetector
		if dt != nil {
			det = dt.loopDetector
		}
		config := &ToolConfig{Manager: mgr, Name: name, Desc: desc, Params: params, LoopDetector: det, Dispatch: dt}
		return &ToolImpl{config: config}
	}
	toolsList := make([]tool.BaseTool, 0)
	for _, def := range tools.GetAllToolDefinitions() {
		var params *schema.ParamsOneOf
		if def.Params != nil {
			params = buildRuntimeParams(def.Params)
		}
		desc := def.Description
		if def.Name == tools.ToolSkill && mgr != nil {
			if sm := mgr.GetSkillManager(); sm != nil {
				list := strings.TrimSpace(sm.FormatSkillsForPrompt())
				if list != "" {
					desc = strings.TrimSpace(desc) + "\n\n" + list
				}
			}
		}
		toolsList = append(toolsList, wrap(def.Name, desc, params))
	}
	if ddg := buildDuckDuckGoSearchTool(ctx); ddg != nil {
		toolsList = append(toolsList, ddg)
	}
	if wiki := buildWikipediaSearchTool(ctx); wiki != nil {
		toolsList = append(toolsList, wiki)
	}
	if httpGet := buildHTTPGetTool(ctx); httpGet != nil {
		toolsList = append(toolsList, httpGet)
	}
	if httpPost := buildHTTPPostTool(ctx); httpPost != nil {
		toolsList = append(toolsList, httpPost)
	}
	if httpPut := buildHTTPPutTool(ctx); httpPut != nil {
		toolsList = append(toolsList, httpPut)
	}
	if httpDel := buildHTTPDeleteTool(ctx); httpDel != nil {
		toolsList = append(toolsList, httpDel)
	}
	if len(mcpTools) > 0 {
		toolsList = append(toolsList, mcpTools...)
	}
	return toolsList
}

func BuildRuntimeReadOnlyTools(ctx context.Context, mgr *tools.Manager) []tool.BaseTool {
	return BuildRuntimeReadOnlyToolsWithMCP(ctx, mgr, nil, nil)
}

func BuildRuntimeReadOnlyToolsWithMCP(ctx context.Context, mgr *tools.Manager, dt *DispatchTools, mcpTools []tool.BaseTool) []tool.BaseTool {
	wrap := func(name, desc string, params *schema.ParamsOneOf) tool.BaseTool {
		config := &ToolConfig{Manager: mgr, Name: name, Desc: desc, Params: params, Dispatch: dt}
		return &ToolImpl{config: config}
	}
	toolsList := make([]tool.BaseTool, 0)
	for _, def := range tools.GetAllToolDefinitions() {
		if def.RiskLevel != tools.RiskLevelLow {
			continue
		}
		var params *schema.ParamsOneOf
		if def.Params != nil {
			params = buildRuntimeParams(def.Params)
		}
		desc := def.Description
		if def.Name == tools.ToolSkill && mgr != nil {
			if sm := mgr.GetSkillManager(); sm != nil {
				list := strings.TrimSpace(sm.FormatSkillsForPrompt())
				if list != "" {
					desc = strings.TrimSpace(desc) + "\n\n" + list
				}
			}
		}
		toolsList = append(toolsList, wrap(def.Name, desc, params))
	}
	toolsList = append(toolsList, wrap("vision_parse", "使用当前模型解析图片", buildRuntimeParams(map[string]*schema.ParameterInfo{
		"images": {Type: schema.Array, Required: true},
		"prompt": {Type: schema.String, Required: false},
	})))
	if ddg := buildDuckDuckGoSearchTool(ctx); ddg != nil {
		toolsList = append(toolsList, ddg)
	}
	if wiki := buildWikipediaSearchTool(ctx); wiki != nil {
		toolsList = append(toolsList, wiki)
	}
	if httpGet := buildHTTPGetTool(ctx); httpGet != nil {
		toolsList = append(toolsList, httpGet)
	}
	if httpPost := buildHTTPPostTool(ctx); httpPost != nil {
		toolsList = append(toolsList, httpPost)
	}
	if httpPut := buildHTTPPutTool(ctx); httpPut != nil {
		toolsList = append(toolsList, httpPut)
	}
	if httpDel := buildHTTPDeleteTool(ctx); httpDel != nil {
		toolsList = append(toolsList, httpDel)
	}
	if len(mcpTools) > 0 {
		toolsList = append(toolsList, mcpTools...)
	}
	return toolsList
}

func BuildRuntimeTesterTools(ctx context.Context, mgr *tools.Manager) []tool.BaseTool {
	return BuildRuntimeReadOnlyTools(ctx, mgr)
}

func BuildDispatchTools(ctx context.Context, dt *DispatchTools) []tool.BaseTool {
	wrap := func(name string, handler func(map[string]any) (DispatchResult, error)) tool.BaseTool {
		return &DispatchToolImpl{
			name:    name,
			handler: handler,
		}
	}
	toStrings := func(v any) []string {
		switch x := v.(type) {
		case []string:
			return append([]string(nil), x...)
		case []any:
			out := make([]string, 0, len(x))
			for _, item := range x {
				if s, ok := item.(string); ok {
					out = append(out, s)
				}
			}
			return out
		default:
			return nil
		}
	}

	return []tool.BaseTool{
		wrap("invoke_planner", func(args map[string]any) (DispatchResult, error) {
			task, _ := args["task"].(string)
			return dt.InvokePlanner(DispatchTask{Task: task}), nil
		}),
		wrap("invoke_senior_dev", func(args map[string]any) (DispatchResult, error) {
			task, _ := args["task"].(string)
			return dt.InvokeSeniorDev(DispatchTask{Task: task}), nil
		}),
		wrap("invoke_tester", func(args map[string]any) (DispatchResult, error) {
			task, _ := args["task"].(string)
			return dt.InvokeTester(DispatchTask{Task: task}), nil
		}),
		wrap("invoke_verification", func(args map[string]any) (DispatchResult, error) {
			task, _ := args["task"].(string)
			return dt.InvokeVerification(DispatchTask{Task: task}), nil
		}),
		wrap("invoke_reviewer", func(args map[string]any) (DispatchResult, error) {
			task, _ := args["task"].(string)
			return dt.InvokeReviewer(DispatchTask{Task: task}), nil
		}),
		wrap("spawn_agent", func(args map[string]any) (DispatchResult, error) {
			agentName, _ := args["agent"].(string)
			task, _ := args["task"].(string)
			forkContext, ok := args["fork_context"].(bool)
			if !ok {
				forkContext = true
			}
			strategy, _ := args["context_strategy"].(string)
			return dt.SpawnAgent(agentName, task, forkContext, strategy, toStrings(args["allowed_tools"]))
		}),
		wrap("send_input", func(args map[string]any) (DispatchResult, error) {
			agentID, _ := args["agent_id"].(string)
			input, _ := args["input"].(string)
			return dt.SendInput(agentID, input)
		}),
		wrap("wait_agent", func(args map[string]any) (DispatchResult, error) {
			agentID, _ := args["agent_id"].(string)
			timeoutMS := 0.0
			switch v := args["timeout_ms"].(type) {
			case float64:
				timeoutMS = v
			case int:
				timeoutMS = float64(v)
			case int64:
				timeoutMS = float64(v)
			}
			return dt.WaitAgent(agentID, time.Duration(timeoutMS)*time.Millisecond)
		}),
		wrap("resume_agent", func(args map[string]any) (DispatchResult, error) {
			agentID, _ := args["agent_id"].(string)
			task, _ := args["task"].(string)
			return dt.ResumeAgent(agentID, task)
		}),
		wrap("close_agent", func(args map[string]any) (DispatchResult, error) {
			agentID, _ := args["agent_id"].(string)
			return dt.CloseAgent(agentID)
		}),
	}
}
