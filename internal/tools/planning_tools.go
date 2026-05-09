package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/ai"
	codectx "github.com/dreamSailing/eos/internal/context"
	"github.com/dreamSailing/eos/internal/pkg/utils"
)

func (m *Manager) planStepsStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	req, _ := params["user_request"].(string)
	constraints := toStringSlice(params["constraints"])
	prefLangs := toStringSlice(params["preferred_languages"])
	maxSteps := toInt(params["max_steps"], 8)
	if strings.TrimSpace(req) == "" {
		return ToolResult{Type: "tool_result", Tool: "plan_steps", Status: "error", Error: "user_request required"}
	}
	wd, _ := os.Getwd()
	e := codectx.NewEngine(wd)
	idxp := filepath.Join(wd, ".eos", "index.json")
	langCounts := map[string]int{}
	if _, err := os.Stat(idxp); err == nil {
		_ = e.LoadIndex(idxp)
	} else {
		_ = e.BuildIndex()
	}
	for _, fm := range e.Index {
		if fm.Lang != "" {
			langCounts[fm.Lang]++
		}
	}
	ctxK := toInt(params["context_k"], 6)
	nbDepth := toInt(params["neighbors_depth"], 1)
	attachSnips := false
	if v, ok := params["attach_snippets"].(bool); ok {
		attachSnips = v
	}
	snipLimit := toInt(params["snippet_bytes_limit"], 1024)
	symLimit := toInt(params["symbols_limit"], 12)
	cands := make([]map[string]interface{}, 0, ctxK)
	neighbors := map[string][]string{}
	if ctxK > 0 {
		sugg := e.Suggest(req, ctxK)
		for _, s := range sugg {
			deg := len(e.ImportsOf(s.Path)) + len(e.ReverseImportsOf(s.Path))
			syms := s.Symbols
			if len(syms) > 8 {
				syms = syms[:8]
			}
			cands = append(cands, map[string]interface{}{"path": s.Path, "lang": s.Lang, "degree": deg, "symbols": syms})
			ns := e.Neighbors(s.Path, nbDepth)
			if len(ns) > 12 {
				ns = ns[:12]
			}
			neighbors[s.Path] = ns
		}
	}
	var nodesDeg []struct {
		P string
		D int
	}
	for p := range e.Index {
		d := len(e.ImportsOf(p)) + len(e.ReverseImportsOf(p))
		nodesDeg = append(nodesDeg, struct {
			P string
			D int
		}{P: p, D: d})
	}
	sort.Slice(nodesDeg, func(i, j int) bool { return nodesDeg[i].D > nodesDeg[j].D })
	hotspots := make([]string, 0, 10)
	for i := 0; i < len(nodesDeg) && i < 10; i++ {
		hotspots = append(hotspots, fmt.Sprintf("%s(%d)", nodesDeg[i].P, nodesDeg[i].D))
	}
	snippets := map[string]string{}
	symbols := map[string][]string{}
	if attachSnips {
		wd2, _ := os.Getwd()
		for _, c := range cands {
			p, _ := c["path"].(string)
			if strings.TrimSpace(p) == "" {
				continue
			}
			ap := p
			if !filepath.IsAbs(ap) {
				ap = filepath.Join(wd2, filepath.FromSlash(p))
			}
			if rel, e2 := filepath.Rel(wd2, ap); e2 != nil || strings.HasPrefix(rel, "..") {
				continue
			}
			if txt, err := m.fileOps.ReadFile(ap); err == nil && txt != "" {
				if snipLimit > 0 && len(txt) > snipLimit {
					txt = txt[:snipLimit]
				}
				snippets[p] = txt
			}
			if fm := e.Index[p]; fm != nil {
				sy := fm.Symbols
				if symLimit > 0 && len(sy) > symLimit {
					sy = sy[:symLimit]
				}
				symbols[p] = sy
			}
		}
	}
	var dyn struct {
		Steps       []map[string]interface{} `json:"steps"`
		Summary     string                   `json:"summary"`
		Assumptions []string                 `json:"assumptions"`
		Notes       []string                 `json:"notes"`
	}
	if apiKey, base, model := ai.ResolveAPISettings(); strings.TrimSpace(apiKey) != "" && strings.TrimSpace(base) != "" && strings.TrimSpace(model) != "" {
		sys := "You are a senior software planning assistant. Given a vague user request and repository context, produce a concise execution plan as strict JSON with fields: steps[{id,title,description,type,inputs,outputs,depends_on,risk,effort}], summary, assumptions, notes. Limit steps to max_steps and map actions to available tools (search_files, read_file, generate_diff, git_*)."
		usr := fmt.Sprintf("request: %s\nconstraints: %s\npreferred_languages: %s\nlang_counts: %v\ncontext_candidates: %v\ncontext_neighbors: %v\nhotspots: %v\ncontext_symbols: %v\ncontext_snippets: %v\nmax_steps: %d", req, strings.Join(constraints, "; "), strings.Join(prefLangs, ", "), langCounts, cands, neighbors, hotspots, symbols, snippets, maxSteps)
		body := map[string]any{
			"model": model,
			"messages": []map[string]any{
				{"role": "system", "content": sys},
				{"role": "user", "content": usr},
			},
		}
		bs, _ := json.Marshal(body)
		url := strings.TrimRight(base, "/") + "/v1/chat/completions"
		// 添加超时控制：最多等待 60 秒
		reqCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(reqCtx, "POST", url, bytes.NewReader(bs))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		// 使用重试机制
		retryPolicy := utils.RetryPolicy{
			MaxAttempts: 3,
			BaseDelay:   500 * time.Millisecond,
			MaxDelay:    5 * time.Second,
			Multiplier:  2.0,
		}
		result := utils.DoHTTPRetryWithClient(reqCtx, http.DefaultClient, req, retryPolicy)

		if result.Error == nil && result.Response != nil {
			defer func() { _ = result.Response.Body.Close() }()
			rb, _ := io.ReadAll(result.Response.Body)
			if result.Response.StatusCode == 200 {
				var out struct {
					Choices []struct {
						Message struct {
							Content string `json:"content"`
						} `json:"message"`
					} `json:"choices"`
				}
				if err := json.Unmarshal(rb, &out); err == nil && len(out.Choices) > 0 {
					outText := strings.TrimSpace(out.Choices[0].Message.Content)
					if outText != "" {
						txt := outText
						if i := strings.Index(outText, "{"); i >= 0 {
							txt = outText[i:]
						}
						_ = json.Unmarshal([]byte(txt), &dyn)
					}
				}
			}
		}
	}
	if len(dyn.Steps) > 0 {
		ctxobj := map[string]interface{}{"candidates": cands, "neighbors": neighbors, "hotspots": hotspots, "snippets": snippets, "symbols": symbols}
		data := map[string]interface{}{"steps": dyn.Steps, "summary": dyn.Summary, "assumptions": dyn.Assumptions, "notes": dyn.Notes, "lang_counts": langCounts, "context": ctxobj}
		disp := fmt.Sprintf("%d step(s)", len(dyn.Steps))
		return ToolResult{Type: "tool_result", Tool: "plan_steps", Status: "success", Data: data, Display: disp}
	}
	type step struct {
		Id        int
		Title     string
		Desc      string
		Type      string
		Inputs    []string
		Outputs   []string
		DependsOn []int
		Risk      string
		Effort    string
	}
	var steps []step
	add := func(s step) {
		if maxSteps <= 0 || len(steps) < maxSteps {
			s.Id = len(steps) + 1
			steps = append(steps, s)
		}
	}
	add(step{Title: "分析需求与影响范围", Desc: "理解用户请求，确定涉及的模块与语言", Type: "analysis", Inputs: []string{req}, Outputs: []string{"目标模块", "影响清单"}, Risk: "low", Effort: "S"})
	add(step{Title: "检索相关文件与符号", Desc: "使用搜索工具查找相关路径与关键词", Type: "search", Inputs: []string{"keywords from request"}, Outputs: []string{"候选文件"}, DependsOn: []int{1}, Risk: "low", Effort: "S"})
	add(step{Title: "拟定修改方案与差异", Desc: "生成并审阅代码变更，最小可行修改", Type: "edit", Inputs: []string{"候选文件"}, Outputs: []string{"diff"}, DependsOn: []int{2}, Risk: "medium", Effort: "M"})
	add(step{Title: "构建/测试并解析错误", Desc: "运行构建或测试，解析并修复报错", Type: "test", Inputs: []string{"diff"}, Outputs: []string{"通过的构建/测试"}, DependsOn: []int{3}, Risk: "medium", Effort: "M"})
	add(step{Title: "复查与记录", Desc: "回顾改动、记录说明与后续事项", Type: "review", Inputs: []string{"diff"}, Outputs: []string{"说明与后续"}, DependsOn: []int{3, 4}, Risk: "low", Effort: "S"})
	notes := []string{}
	if len(constraints) > 0 {
		notes = append(notes, "constraints: "+strings.Join(constraints, "; "))
	}
	if len(prefLangs) > 0 {
		notes = append(notes, "preferred_languages: "+strings.Join(prefLangs, ", "))
	}
	arr := make([]map[string]interface{}, 0, len(steps))
	for _, s := range steps {
		arr = append(arr, map[string]interface{}{"id": s.Id, "title": s.Title, "description": s.Desc, "type": s.Type, "inputs": s.Inputs, "outputs": s.Outputs, "depends_on": s.DependsOn, "risk": s.Risk, "effort": s.Effort})
	}
	assumptions := []string{"使用现有工具进行检索/编辑/测试", "保持最小改动以降低风险"}
	summary := "生成基于仓库语言分布与现有工具能力的执行计划"
	ctxobj := map[string]interface{}{"candidates": cands, "neighbors": neighbors, "hotspots": hotspots, "snippets": snippets, "symbols": symbols}
	data := map[string]interface{}{"steps": arr, "summary": summary, "assumptions": assumptions, "notes": notes, "lang_counts": langCounts, "context": ctxobj}
	disp := fmt.Sprintf("%d step(s)", len(arr))
	return ToolResult{Type: "tool_result", Tool: "plan_steps", Status: "success", Data: data, Display: disp}
}

func (m *Manager) todoReadStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	items0 := DefaultTodoStore().List()
	items := make([]map[string]interface{}, 0, len(items0))
	for _, it := range items0 {
		item := map[string]interface{}{}
		if strings.TrimSpace(it.ID) != "" {
			item["id"] = it.ID
		}
		item["content"] = it.Content
		item["status"] = it.Status
		if it.Priority != nil {
			item["priority"] = it.Priority
		}
		items = append(items, item)
	}
	data := map[string]interface{}{"items": items}
	disp := fmt.Sprintf("%d todo item(s)", len(items))
	return ToolResult{Type: "tool_result", Tool: "todo_read", Status: "success", Data: data, Display: disp}
}

func (m *Manager) todoWriteStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	raw, ok := params["items"]
	if !ok {
		return ToolResult{Type: "tool_result", Tool: "todo_write", Status: "error", Error: "items required"}
	}
	var arr []map[string]interface{}
	switch v := raw.(type) {
	case []interface{}:
		arr = make([]map[string]interface{}, 0, len(v))
		for i, it := range v {
			obj, ok := it.(map[string]interface{})
			if !ok {
				return ToolResult{Type: "tool_result", Tool: "todo_write", Status: "error", Error: fmt.Sprintf("items[%d] must be object", i)}
			}
			arr = append(arr, obj)
		}
	case []map[string]interface{}:
		arr = v
	default:
		return ToolResult{Type: "tool_result", Tool: "todo_write", Status: "error", Error: "items must be array"}
	}

	keysOf := func(obj map[string]interface{}) string {
		if obj == nil {
			return ""
		}
		var ks []string
		for k := range obj {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		return strings.Join(ks, ",")
	}

	items := make([]TodoItem, 0, len(arr))
	for i, obj := range arr {
		content, _ := obj["content"].(string)
		if strings.TrimSpace(content) == "" {
			if s, ok := obj["text"].(string); ok {
				content = s
			} else if s, ok := obj["title"].(string); ok {
				content = s
			}
		}
		if strings.TrimSpace(content) == "" {
			return ToolResult{Type: "tool_result", Tool: "todo_write", Status: "error", Error: fmt.Sprintf("items[%d].content required (keys: %s)", i, keysOf(obj))}
		}
		status, _ := obj["status"].(string)
		status = strings.ToLower(strings.TrimSpace(status))
		if status == "" {
			status = "pending"
		}
		if status != "pending" && status != "in_progress" && status != "completed" {
			return ToolResult{Type: "tool_result", Tool: "todo_write", Status: "error", Error: fmt.Sprintf("items[%d].status must be pending, in_progress, or completed", i)}
		}
		item := TodoItem{
			Content: content,
			Status:  status,
		}
		if id, ok := obj["id"].(string); ok {
			item.ID = strings.TrimSpace(id)
		}
		if pr, ok := obj["priority"]; ok {
			item.Priority = pr
		}
		items = append(items, item)
	}
	DefaultTodoStore().Replace(items)
	data := map[string]interface{}{"count": len(items)}
	disp := fmt.Sprintf("Updated %d todo item(s)", len(items))
	return ToolResult{Type: "tool_result", Tool: "todo_write", Status: "success", Data: data, Display: disp}
}
