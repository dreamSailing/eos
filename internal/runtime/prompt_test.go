package runtime

import (
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
