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
