package runtime

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestShouldBypassArchitect_SimpleChangeWithQuestionMark(t *testing.T) {
	msgs := []*schema.Message{
		schema.UserMessage("能不能修复一下 warning? 现在有 lint 报错"),
	}
	if !shouldBypassArchitect(msgs) {
		t.Fatalf("expected bypass for code change with question mark")
	}
}

func TestShouldBypassArchitect_QuestionOnly(t *testing.T) {
	msgs := []*schema.Message{
		schema.UserMessage("什么是 ContextManager？"),
	}
	if shouldBypassArchitect(msgs) {
		t.Fatalf("expected not bypass for question-only")
	}
}

func TestShouldBypassArchitect_PlanPreferenceFirst(t *testing.T) {
	msgs := []*schema.Message{
		schema.SystemMessage("PLAN_PREFERENCE: prefer_plan_first"),
		schema.UserMessage("修复一个小 bug"),
	}
	if shouldBypassArchitect(msgs) {
		t.Fatalf("expected not bypass when plan preference first")
	}
}

