package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"github.com/dreamSailing/vb-coding/internal/tools"

	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
		if j := strings.LastIndex(s, "```"); j >= 0 {
			s = s[:j]
		}
	}
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i >= 0 && j >= i {
		return s[i : j+1]
	}
	return ""
}

type dispatchDirective struct {
	Type         string   `json:"type"`
	Role         string   `json:"role"`
	Task         string   `json:"task"`
	ToolsAllowed []string `json:"tools_allowed"`
}

func parseDispatchDirective(msg *schema.Message) (dispatchDirective, bool) {
	if msg == nil {
		return dispatchDirective{}, false
	}
	raw := strings.TrimSpace(msg.Content)
	j := extractJSON(raw)
	if j == "" {
		j = raw
	}
	if strings.TrimSpace(j) == "" {
		return dispatchDirective{}, false
	}
	var d dispatchDirective
	if json.Unmarshal([]byte(j), &d) != nil {
		return dispatchDirective{}, false
	}
	if strings.TrimSpace(d.Type) == "" {
		return dispatchDirective{}, false
	}
	return d, true
}

func intersectAllowedTools(base map[string]bool, want []string) map[string]bool {
	if base == nil {
		return nil
	}
	if len(want) == 0 {
		cp := make(map[string]bool, len(base))
		for k, v := range base {
			cp[k] = v
		}
		return cp
	}
	allowed := make(map[string]bool, len(want))
	for _, w := range want {
		n := strings.ToLower(strings.TrimSpace(w))
		if n == "" {
			continue
		}
		if base[n] {
			allowed[n] = true
		}
	}
	return allowed
}

func AllowedTools(role string) map[string]bool {
	ro := map[string]bool{
		"duckduckgo_search": true,
		"wikipedia_search":  true,
		tools.ToolRead:      true,
		tools.ToolSearch:    true,
		tools.ToolTodoRead:  true,
		tools.ToolUserInput: true,
		tools.ToolUserSelect: true,
		"vision_parse":      true,
		tools.ToolGitStatus: true,
		tools.ToolGitDiff:   true,
		tools.ToolGitLog:    true,
		strings.ToLower(tools.ToolProjectStructure): true,
	}
	rwDev := map[string]bool{
		tools.ToolEdit:          true,
		tools.ToolFS:            true,
		tools.ToolPlanSteps:     true,
		tools.ToolGitAdd:        true,
		tools.ToolGitCommit:     true,
		tools.ToolGitBranchList: true,
		tools.ToolGitCheckout:   true,
		tools.ToolGitInit:       true,
		tools.ToolGitPull:       true,
		tools.ToolGitPush:       true,
		tools.ToolTodoWrite:     true,
	}
	bashTools := map[string]bool{
		tools.ToolBash:        true,
		tools.ToolBashSession: true,
	}
	tester := map[string]bool{}
	for k := range ro {
		tester[k] = true
	}
	tester[tools.ToolGitStash] = true
	tester[tools.ToolGitReset] = true
	tester[tools.ToolGitRevert] = true
	tester[tools.ToolGitMerge] = true
	tester[tools.ToolGitRebase] = true
	// Add bash tools for tester role (can run tests via "go test ./...")
	tester[tools.ToolBash] = true
	tester[tools.ToolBashSession] = true
	tester["http_get"] = true
	tester["http_post"] = true
	tester["http_put"] = true
	tester["http_delete"] = true
	dev := map[string]bool{}
	for k := range ro {
		dev[k] = true
	}
	for k := range rwDev {
		dev[k] = true
	}
	dev[tools.ToolGitStash] = true
	dev[tools.ToolGitReset] = true
	dev[tools.ToolGitRevert] = true
	dev[tools.ToolGitMerge] = true
	dev[tools.ToolGitRebase] = true
	dev["http_get"] = true
	dev["http_post"] = true
	dev["http_put"] = true
	dev["http_delete"] = true
	switch role {
	case "architect", "reviewer", "planner":
		return ro
	case "tester":
		return tester
	case "senior-dev":
		for k := range bashTools {
			dev[k] = true
		}
		return dev
	default:
		return ro
	}
}

func RoleInstruction(role string) string {
	switch role {
	case "architect":
		return RoleArchitectPrompt
	case "planner":
		return PlanPrompt
	case "senior-dev":
		return RoleSeniorDevPrompt
	case "reviewer":
		return RoleReviewerPrompt
	case "tester":
		return RoleTesterPrompt
	default:
		return RoleDefaultPrompt
	}
}

func invokeRoleAgent(ctx context.Context, in []*schema.Message, role string, agent *react.Agent, onMeta func(string), onReasoning func(string), mcpToolsInfo string) ([]*schema.Message, error) {
	if agent == nil {
		return nil, fmt.Errorf("agent for role %s not initialized", role)
	}

	LogDebug("runtime.role_agent.invoke.start", "role", role, "input_messages", len(in))

	roleNorm := strings.ToLower(strings.TrimSpace(role))
	// 子 agent 使用角色的完整工具集
	allowed := AllowedTools(roleNorm)
	base := in
	task := ""
	if len(in) > 0 {
		if d, ok := parseDispatchDirective(in[len(in)-1]); ok {
			if strings.EqualFold(strings.TrimSpace(d.Type), "assign") {
				base = in[:len(in)-1]
				r := strings.ToLower(strings.TrimSpace(d.Role))
				if r != "" && r == roleNorm {
					allowed = intersectAllowedTools(allowed, d.ToolsAllowed)
					task = strings.TrimSpace(d.Task)
					LogDebug("runtime.role_agent.dispatch",
						"role", role,
						"task", task,
						"available_tools_count", len(allowed))
				}
			}
		} else {
			// 检查是否有任务分配事件
			lastMsg := in[len(in)-1]
			for _, line := range strings.Split(lastMsg.Content, "\n") {
				if strings.Contains(line, EventAgentTask+":") {
					task = strings.TrimSpace(strings.TrimPrefix(line, EventAgentTask+":"))
					LogDebug("runtime.role_agent.agent_task", "role", role, "task", task)
					break
				}
			}
		}
	}
	ctx = tools.WithRole(ctx, role)
	ctx = tools.WithAllowedTools(ctx, allowed)

	// 使用更丰富的提示词
	instruction := RoleInstruction(role)
	if task != "" {
		instruction += "\n\n当前任务: " + task
	}
	if mcpToolsInfo != "" {
		instruction += "\n\n" + mcpToolsInfo
	}

	msgs := append([]*schema.Message{schema.SystemMessage(instruction)}, base...)

	LogDebug("runtime.role_agent.invoke", "role", role, "task", task, "allowed_tools_count", len(allowed))

	// 调用 Agent
	out, err := agent.Generate(ctx, msgs)
	if err != nil {
		return nil, wrapMaxStepError(err)
	}

	// 先处理推理内容（如果有）
	if onReasoning != nil && out.ReasoningContent != "" {
		onReasoning(out.ReasoningContent)
		LogDebug("runtime.role_agent.reasoning", "role", role, "length", len(out.ReasoningContent))
	}

	// 发送 agent.final 事件，使子 Agent 输出显示为白色圆点（区别于调度 Agent 的绿色圆点）
	if onMeta != nil && out.Content != "" {
		onMeta(EventAgentFinal + ":" + out.Content)
	}

	return append(base, out), nil
}

func converterNode(ctx context.Context, in []*schema.Message) (*schema.Message, error) {
	if len(in) == 0 {
		return nil, nil
	}
	// 返回最后一条消息（可能是 Reviewer 的结论，也可能是 Exec 的结果）
	return in[len(in)-1], nil
}
