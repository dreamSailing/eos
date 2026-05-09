package runtime

import (
	"context"
	"strings"
	"testing"
)

func TestGetDispatchToolsDescription_OnlyListsDispatchTools(t *testing.T) {
	desc := GetDispatchToolsDescription()

	for _, want := range []string{
		"`invoke_planner`",
		"`invoke_senior_dev`",
		"`invoke_tester`",
		"`invoke_reviewer`",
		"`spawn_agent`",
		"`wait_agent`",
	} {
		if !strings.Contains(desc, want) {
			t.Fatalf("dispatch tools description missing %s\n%s", want, desc)
		}
	}

	for _, unwanted := range []string{
		"`ProjectStructure`",
		"`read`",
		"`search`",
		"`mcp_status`",
		"`skills_list`",
	} {
		if strings.Contains(desc, unwanted) {
			t.Fatalf("dispatch tools description unexpectedly contains %s\n%s", unwanted, desc)
		}
	}
}

func TestBuildDispatchSystemPrompt_UsesDispatchToolsOnly(t *testing.T) {
	prompt := buildDispatchSystemPrompt(context.Background(), nil, nil, nil)

	for _, want := range []string{
		"`invoke_senior_dev`",
		"`invoke_planner`",
		"`spawn_agent`",
		"不能直接调用 read、search、ProjectStructure、MCP、skills",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("dispatch system prompt missing %s\n%s", want, prompt)
		}
	}

	for _, unwanted := range []string{
		"`ProjectStructure` — 获取项目目录结构视图",
		"`read` — 统一读取工具",
		"`search` — 统一搜索工具",
		"`mcp_status`",
		"`skills_list`",
		"suggest_memory",
	} {
		if strings.Contains(prompt, unwanted) {
			t.Fatalf("dispatch system prompt unexpectedly contains %s\n%s", unwanted, prompt)
		}
	}
}
