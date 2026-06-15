package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/session"
	"github.com/dreamSailing/eos/internal/tools"
)

func TestBuildHistoryMessages_MergesLeadingSystemMessages(t *testing.T) {
	cm := session.NewContextManager()
	cm.AddPinned(ai.Message{Role: "system", Content: "SYSTEM_A"})
	cm.AddPinned(ai.Message{Role: "system", Content: "TASK_SUMMARY_HISTORY:\n- t1"})
	cm.AddUser("hello")

	msgs := buildHistoryMessages(cm, nil, "")
	if len(msgs) != 3 {
		t.Fatalf("message count = %d, want 3", len(msgs))
	}
	if msgs[0].Role != schema.System {
		t.Fatalf("first role = %v, want system", msgs[0].Role)
	}
	if msgs[1].Role != schema.User {
		t.Fatalf("second role = %v, want user", msgs[1].Role)
	}
	if msgs[2].Role != schema.User {
		t.Fatalf("third role = %v, want user", msgs[2].Role)
	}
	content := strings.TrimSpace(msgs[0].Content)
	if !strings.Contains(content, "SYSTEM_A") {
		t.Fatalf("merged system content missing SYSTEM_A: %q", content)
	}
	if strings.Contains(content, "TASK_SUMMARY_HISTORY:\n- t1") {
		t.Fatalf("task summary should not stay in system content: %q", content)
	}
	summaryContent := strings.TrimSpace(msgs[1].Content)
	if !strings.Contains(summaryContent, "[TASK SUMMARY]") || !strings.Contains(summaryContent, "TASK_SUMMARY_HISTORY:\n- t1") {
		t.Fatalf("unexpected task summary user content: %q", summaryContent)
	}
}

func TestBuildHistoryMessages_PreservesTrailingSystemMessages(t *testing.T) {
	cm := session.NewContextManager()
	cm.AddPinned(ai.Message{Role: "system", Content: "SYSTEM_A"})
	cm.AddUser("hello")

	msgs := buildHistoryMessages(cm, []ai.Message{{Role: "system", Content: "STOP_HOOK: retry"}}, "")
	if len(msgs) != 3 {
		t.Fatalf("message count = %d, want 3", len(msgs))
	}
	if msgs[0].Role != schema.System || strings.TrimSpace(msgs[0].Content) != "SYSTEM_A" {
		t.Fatalf("unexpected first message: role=%v content=%q", msgs[0].Role, msgs[0].Content)
	}
	if msgs[1].Role != schema.User || strings.TrimSpace(msgs[1].Content) != "hello" {
		t.Fatalf("unexpected user message: role=%v content=%q", msgs[1].Role, msgs[1].Content)
	}
	if msgs[2].Role != schema.System || strings.TrimSpace(msgs[2].Content) != "STOP_HOOK: retry" {
		t.Fatalf("unexpected trailing system message: role=%v content=%q", msgs[2].Role, msgs[2].Content)
	}
}

func TestBuildProjectPromptAdditionsUsesEOSGuideNaming(t *testing.T) {
	dir := t.TempDir()
	legacyGuideName := "VB" + ".md"
	if err := os.WriteFile(filepath.Join(dir, "EOS.md"), []byte("# EOS.md\n\nRules"), 0o644); err != nil {
		t.Fatal(err)
	}

	prompt := BuildProjectPromptAdditions(dir)
	if !strings.Contains(prompt, "EOS.md") {
		t.Fatalf("expected prompt to mention EOS.md, got %q", prompt)
	}
	if strings.Contains(prompt, legacyGuideName) {
		t.Fatalf("expected prompt to drop the legacy guide naming, got %q", prompt)
	}
}

func TestBuildPlanPromptForStyleVariants(t *testing.T) {
	concise := BuildPlanPromptForStyle("")
	if concise != PlanPrompt {
		t.Fatalf("empty style should use base plan prompt")
	}

	detailed := BuildPlanPromptForStyle("detailed")
	if detailed == PlanPrompt || !strings.Contains(detailed, "计划提示风格：详细") {
		t.Fatalf("detailed style did not append detailed instructions:\n%s", detailed)
	}

	custom := BuildPlanPromptForStyle("custom:先给出风险排序")
	if !strings.Contains(custom, "计划提示风格：自定义") || !strings.Contains(custom, "先给出风险排序") {
		t.Fatalf("custom style missing custom instructions:\n%s", custom)
	}

	legacy := BuildPlanPromptForStyle("structured")
	if !strings.Contains(legacy, "计划提示风格：自定义") || !strings.Contains(legacy, "structured") {
		t.Fatalf("legacy free-form style should be treated as custom:\n%s", legacy)
	}
}

func TestBuildRoleSystemPromptUsesPlanPromptStyleOnlyForPlanner(t *testing.T) {
	ctx := WithPlanPromptStyle(context.Background(), "detailed")

	plannerPrompt := buildRoleSystemPrompt(ctx, "planner", "")
	if !strings.Contains(plannerPrompt, "计划提示风格：详细") {
		t.Fatalf("planner prompt missing detailed style:\n%s", plannerPrompt)
	}

	seniorPrompt := buildRoleSystemPrompt(ctx, "senior-dev", "")
	if strings.Contains(seniorPrompt, "计划提示风格：详细") {
		t.Fatalf("senior-dev prompt should not receive planner style:\n%s", seniorPrompt)
	}
}

func TestBuildRoleSystemPromptUsesProjectRoleOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".eos"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `{"roles":[{"id":"reviewer","legacy_names":["review"],"system_prompt":"CUSTOM_REVIEWER_PROMPT","context_strategy":"independent","allowed_tools":["read"]}]}`
	if err := os.WriteFile(filepath.Join(dir, ".eos", "roles.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := tools.WithWorkspaceRoot(context.Background(), dir)
	prompt := buildRoleSystemPrompt(ctx, "review", "MCP_TOOLS_SHOULD_NOT_APPEAR")

	if !strings.Contains(prompt, "CUSTOM_REVIEWER_PROMPT") {
		t.Fatalf("prompt missing project role override:\n%s", prompt)
	}
	if strings.Contains(prompt, "MCP_TOOLS_SHOULD_NOT_APPEAR") {
		t.Fatalf("reviewer prompt should not receive MCP tools info:\n%s", prompt)
	}
	if !strings.Contains(prompt, "- 工作目录: "+dir) {
		t.Fatalf("prompt missing workspace env info:\n%s", prompt)
	}
}

func TestRuntimeRoleConfigOverridesStrategyAndAllowedTools(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".eos"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `{"roles":[{"id":"verification","legacy_aliases":["verify"],"system_prompt":"verify","context_strategy":"hybrid","allowed_tools":["read"]}]}`
	if err := os.WriteFile(filepath.Join(dir, ".eos", "roles.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := tools.WithWorkspaceRoot(context.Background(), dir)
	if got := runtimeRoleContextStrategy(ctx, "verify", ContextStrategyIndependent); got != ContextStrategyHybrid {
		t.Fatalf("strategy = %s, want hybrid", got)
	}
	allowed := runtimeRoleAllowedTools(ctx, "verification", nil)
	if len(allowed) != 1 || allowed[0] != "read" {
		t.Fatalf("allowed tools = %#v, want [read]", allowed)
	}
}

func TestSkillCreationPromptsMentionScopeWorkflow(t *testing.T) {
	if !strings.Contains(RoleDefaultPrompt, "create_skill") {
		t.Fatalf("RoleDefaultPrompt should mention create_skill workflow:\n%s", RoleDefaultPrompt)
	}
	if !strings.Contains(RoleDefaultPrompt, "ask_user_question") {
		t.Fatalf("RoleDefaultPrompt should mention asking the user for scope:\n%s", RoleDefaultPrompt)
	}

	usingTools := getUsingToolsSection(&PromptConfig{})
	if !strings.Contains(usingTools, "create_skill") {
		t.Fatalf("using tools guidance should mention create_skill:\n%s", usingTools)
	}
	if !strings.Contains(usingTools, "ask_user_question") {
		t.Fatalf("using tools guidance should mention ask_user_question:\n%s", usingTools)
	}
}

func TestUsingToolsSectionEnforcesParameterContract(t *testing.T) {
	// Playbook §5「工具调用策略」要求系统层提示词约束：参数不确定时先探索，
	// 不要猜参数调用。这些是所有 agent 共享的行为约束，必须出现在
	// getUsingToolsSection 中，不能只留在 RoleSeniorDevPrompt 里。
	usingTools := getUsingToolsSection(&PromptConfig{})

	for _, want := range []string{
		"把工具调用当成严格 API 调用",
		"参数不确定时：先探索，再调用",
		"不允许编造路径、文件名、命令、API 名、配置字段或 ID",
		"并行工具调用",
	} {
		if !strings.Contains(usingTools, want) {
			t.Fatalf("using tools guidance missing %q\n%s", want, usingTools)
		}
	}
}
