package runtime

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"github.com/dreamSailing/vb-coding/internal/config"
	"github.com/dreamSailing/vb-coding/internal/tools"
)

func buildToolsNodeConfig(toolsList []tool.BaseTool, onMeta func(string)) compose.ToolsNodeConfig {
	cfg := compose.ToolsNodeConfig{Tools: toolsList}
	if onMeta != nil {
		cfg0, _ := config.Load()
		cfg.ToolCallMiddlewares = []compose.ToolMiddleware{buildToolCallMiddleware(onMeta, resolveAgentToolTimeout(cfg0))}
	}
	return cfg
}

func newRuntimeAgentsWithDispatchTools(
	ctx context.Context,
	tm *tools.Manager,
	mdl AIModel,
	dt *DispatchTools,
	mcpTools []tool.BaseTool,
	onMeta func(string),
) (*react.Agent, *react.Agent, *react.Agent, *react.Agent, *react.Agent, error) {
	if tm == nil || mdl == nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("tools manager or model is nil")
	}
	provider, ok := mdl.(ToolCallingProvider)
	if !ok {
		return nil, nil, nil, nil, nil, fmt.Errorf("model does not implement ToolCallingProvider")
	}
	base := provider.ToolCalling()
	if dt != nil && dt.hookMgr != nil {
		dt.hookMgr.SetModel(mdl)
	}

	cfg0, _ := config.Load()
	maxStep := cfg0.Agent.MaxStep
	if maxStep <= 0 {
		maxStep = 160
	}
	if maxStep < 12 {
		maxStep = 12
	}

	var ag *react.Agent
	var dag *react.Agent
	var pag *react.Agent
	var rag *react.Agent
	var tag *react.Agent

	cfg := &react.AgentConfig{
		ToolCallingModel: base,
		ToolsConfig:      buildToolsNodeConfig(BuildRuntimeToolsWithMCP(ctx, tm, dt, mcpTools), onMeta),
		MaxStep:          maxStep,
	}
	if a, e := react.NewAgent(ctx, cfg); e != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to create exec agent: %w", e)
	} else {
		ag = a
	}

	dcfg := &react.AgentConfig{
		ToolCallingModel: base,
		ToolsConfig:      buildToolsNodeConfig(BuildDispatchTools(ctx, dt), onMeta),
		MaxStep:          maxStep,
	}
	if da, e3 := react.NewAgent(ctx, dcfg); e3 != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to create dispatch agent: %w", e3)
	} else {
		dag = da
	}

	pcfg := &react.AgentConfig{
		ToolCallingModel: base,
		ToolsConfig:      buildToolsNodeConfig(BuildRuntimeReadOnlyToolsWithMCP(ctx, tm, dt, mcpTools), onMeta),
		MaxStep:          maxStep,
	}
	if pa, e1 := react.NewAgent(ctx, pcfg); e1 != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to create plan agent: %w", e1)
	} else {
		pag = pa
	}
	if dt != nil && dt.hookMgr != nil && pag != nil {
		dt.hookMgr.SetAgentEvaluator(func(ctx context.Context, prompt string) (string, error) {
			out, err := pag.Generate(ctx, []*schema.Message{schema.UserMessage(prompt)})
			if err != nil {
				return "", err
			}
			return out.Content, nil
		})
	}

	testTools := append(
		BuildRuntimeReadOnlyToolsWithMCP(ctx, tm, dt, mcpTools),
		[]tool.BaseTool{
			&ToolImpl{
				config: &ToolConfig{
					Manager: tm,
					Name:    "bash",
					Desc:    "执行 Shell 命令",
					Params:  buildRuntimeParams(map[string]*schema.ParameterInfo{"command": {Type: schema.String, Required: true}}),
					Dispatch: dt,
				},
			},
			&ToolImpl{
				config: &ToolConfig{
					Manager: tm,
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
	tcfg := &react.AgentConfig{
		ToolCallingModel: base,
		ToolsConfig:      buildToolsNodeConfig(testTools, onMeta),
		MaxStep:          maxStep,
	}
	if ta, e2 := react.NewAgent(ctx, tcfg); e2 != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to create test agent: %w", e2)
	} else {
		tag = ta
	}

	rcfg := &react.AgentConfig{
		ToolCallingModel: base,
		ToolsConfig:      buildToolsNodeConfig(BuildRuntimeReadOnlyToolsWithMCP(ctx, tm, dt, mcpTools), onMeta),
		MaxStep:          maxStep,
	}
	if ra, e4 := react.NewAgent(ctx, rcfg); e4 != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to create review agent: %w", e4)
	} else {
		rag = ra
	}

	return ag, dag, pag, rag, tag, nil
}
