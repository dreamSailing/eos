package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"errors"
	"strings"
	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/tools"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

func runSkillForkFromToolResult(ctx context.Context, dt *DispatchTools, r tools.ToolResult) (string, error) {
	if dt == nil || dt.subAgentMgr == nil {
		return "", errors.New("dispatch tools not initialized")
	}
	if dt.toolsManager == nil {
		return "", errors.New("tools manager not initialized")
	}

	prompt, _ := r.Data["prompt"].(string)
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("missing fork prompt")
	}
	agentName, _ := r.Data["agent"].(string)
	agentType, role, agent := resolveForkAgent(dt, agentName)
	if agent == nil {
		return "", errors.New("fork agent not available: " + strings.TrimSpace(agentName))
	}

	if mo, ok := r.Data["model_override"].(string); ok {
		mo = strings.TrimSpace(mo)
		if mo != "" {
			apiKey, base, _ := ai.ResolveAPISettings()
			mdl, err := NewChatModelWithSettings(ctx, apiKey, base, mo, "")
			if err != nil {
				return "", err
			}
			a2, err := newRoleAgentWithModel(ctx, role, dt, mdl)
			if err != nil {
				return "", err
			}
			agent = a2
		}
	}

	allowed := map[string]bool{}
	if raw, ok := r.Data["allowed_tools"].([]string); ok && len(raw) > 0 {
		for _, t := range normalizeAllowedTools(raw) {
			allowed[t] = true
		}
	}
	if len(allowed) > 0 {
		ctx = tools.WithAllowedTools(ctx, allowed)
	}

	initial := []*schema.Message{schema.UserMessage(prompt)}
	subCtx := dt.subAgentMgr.CreateContextWithStrategy(agentType, ctx, initial, ContextStrategyIndependent, nil)
	outMsgs, err := invokeRoleAgentWithSubContext(ctx, nil, role, agent, dt.onMeta, dt.onReasoning, dt.mcpToolsInfo, subCtx, dt.subAgentMgr, dt.hookMgr)
	if err != nil {
		return "", err
	}
	if len(outMsgs) == 0 {
		return "", nil
	}
	out := outMsgs[len(outMsgs)-1]
	if out == nil {
		return "", nil
	}
	return strings.TrimSpace(out.Content), nil
}

func newRoleAgentWithModel(ctx context.Context, role string, dt *DispatchTools, mdl AIModel) (*react.Agent, error) {
	provider, ok := mdl.(ToolCallingProvider)
	if !ok {
		return nil, errors.New("model does not implement ToolCallingProvider")
	}
	base := provider.ToolCalling()

	cfg0, _ := config.Load()
	maxStep := cfg0.Agent.MaxStep
	if maxStep <= 0 {
		maxStep = 160
	}
	if maxStep < 12 {
		maxStep = 12
	}

	var toolsList []tool.BaseTool
	switch role {
	case "planner", "reviewer":
		toolsList = BuildRuntimeReadOnlyToolsWithMCP(ctx, dt.toolsManager, dt, dt.mcpTools)
	case "tester":
		toolsList = buildTesterToolsForModel(ctx, dt)
	default:
		toolsList = BuildRuntimeToolsWithMCP(ctx, dt.toolsManager, dt, dt.mcpTools)
	}

	ag, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: base,
		ToolsConfig:      buildToolsNodeConfig(toolsList, dt.onMeta),
		MaxStep:          maxStep,
	})
	if err != nil {
		return nil, err
	}
	return ag, nil
}

func buildTesterToolsForModel(ctx context.Context, dt *DispatchTools) []tool.BaseTool {
	base := BuildRuntimeReadOnlyToolsWithMCP(ctx, dt.toolsManager, dt, dt.mcpTools)
	return append(base,
		[]tool.BaseTool{
			&ToolImpl{
				config: &ToolConfig{
					Manager:  dt.toolsManager,
					Name:     "bash",
					Desc:     "执行 Shell 命令",
					Params:   buildRuntimeParams(map[string]*schema.ParameterInfo{"command": {Type: schema.String, Required: true}}),
					Dispatch: dt,
				},
			},
			&ToolImpl{
				config: &ToolConfig{
					Manager: dt.toolsManager,
					Name:    "bash_session",
					Desc:    "在后台会话中执行 Shell 命令（启动/输出/终止）",
					Params: buildRuntimeParams(map[string]*schema.ParameterInfo{
						"mode":    {Type: schema.String, Required: true},
						"id":      {Type: schema.String, Required: false},
						"command": {Type: schema.String, Required: false},
					}),
					Dispatch: dt,
				},
			},
		}...)
}

func resolveForkAgent(dt *DispatchTools, agentName string) (agentType SubAgentType, role string, agent *react.Agent) {
	n := strings.ToLower(strings.TrimSpace(agentName))
	switch n {
	case "plan", "planner":
		return SubAgentTypePlanner, "planner", dt.plannerAgent
	case "test", "tester":
		return SubAgentTypeTester, "tester", dt.testerAgent
	case "review", "reviewer":
		return SubAgentTypeReviewer, "reviewer", dt.reviewerAgent
	case "explore", "explorer":
		return SubAgentTypeExplore, "senior-dev", dt.seniorDevAgent
	case "", "general", "senior-dev", "dev", "developer":
		return SubAgentTypeSeniorDev, "senior-dev", dt.seniorDevAgent
	default:
		return SubAgentTypeSeniorDev, "senior-dev", dt.seniorDevAgent
	}
}

func normalizeAllowedTools(in []string) []string {
	out := make([]string, 0, len(in))
	for _, t := range in {
		tt := strings.ToLower(strings.TrimSpace(t))
		if i := strings.Index(tt, "("); i >= 0 {
			tt = strings.TrimSpace(tt[:i])
		}
		switch tt {
		case "":
			continue
		case "glob", "grep":
			tt = "search"
		case "read":
			tt = "read"
		case "bash":
			tt = "bash"
		case "write", "write_file":
			tt = "fs"
		case "edit":
			tt = "edit"
		}
		out = append(out, tt)
	}
	return out
}

func buildAllowedToolsMap(in []string) map[string]bool {
	if len(in) == 0 {
		return nil
	}
	m := map[string]bool{}
	for _, t := range normalizeAllowedTools(in) {
		if t != "" {
			m[t] = true
		}
	}
	if len(m) == 0 {
		return nil
	}
	m[tools.ToolSkill] = true
	return m
}
