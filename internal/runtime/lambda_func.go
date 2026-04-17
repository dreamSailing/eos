package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"github.com/dreamSailing/eos/internal/tools"

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
		tools.ToolAskUserQuestion: true,
		"vision_parse":      true,
		tools.ToolGitStatus: true,
		tools.ToolGitDiff:   true,
		tools.ToolGitLog:    true,
		tools.ToolGitShow:   true,
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

func converterNode(ctx context.Context, in []*schema.Message) (*schema.Message, error) {
	if len(in) == 0 {
		return nil, nil
	}
	// 返回最后一条消息（可能是 Reviewer 的结论，也可能是 Exec 的结果）
	return in[len(in)-1], nil
}
