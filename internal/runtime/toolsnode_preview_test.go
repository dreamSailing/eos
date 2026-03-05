package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"github.com/dreamSailing/vb-coding/internal/session"
	"github.com/dreamSailing/vb-coding/internal/tools"
)

func TestToolsNode_PreviewForEditUsesPendingDiff(t *testing.T) {
	oldPrompt := tools.SafetyGatePrompt
	oldClassify := tools.SafetyGateClassify
	oldAllowed := tools.SafetyGateSessionAllowed
	oldAllowSession := tools.SafetyGateAllowSession
	oldSetPendingDiff := tools.SetPendingDiff
	defer func() {
		tools.SafetyGatePrompt = oldPrompt
		tools.SafetyGateClassify = oldClassify
		tools.SafetyGateSessionAllowed = oldAllowed
		tools.SafetyGateAllowSession = oldAllowSession
		tools.SetPendingDiff = oldSetPendingDiff
	}()

	var pending string
	tools.SafetyGateClassify = func(call tools.ToolCall) (string, string, string, bool) {
		return tools.ClassifyToolDanger(call)
	}
	tools.SafetyGateSessionAllowed = func(category string) bool { return false }
	tools.SafetyGateAllowSession = func(category string) {}
	tools.SetPendingDiff = func(diff string) { pending = diff }
	tools.SafetyGatePrompt = func(ctx context.Context, category, summary string) string {
		return "deny"
	}

	cm := session.NewContextManager()
	mgr := tools.NewManager()
	rt := &EinoRuntime{ctxm: cm, tools: mgr, recentToolCalls: map[string]int{}, recentAssistantHashes: map[string]int{}}

	wd, _ := os.Getwd()
	p := filepath.Join(wd, "tmp", "toolsnode_preview_edit.txt")
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	_ = os.WriteFile(p, []byte("a\n"), 0644)
	defer func() { _ = os.Remove(p) }()

	payload := `{"tool":"edit","parameters":{"mode":"single","file":"tmp/toolsnode_preview_edit.txt","find":"a","replace":"b"}}`
	_, executed, _ := rt.ToolsNode(context.Background(), payload)
	if !executed {
		t.Fatalf("expected tools node to execute")
	}
	if !strings.Contains(pending, "--- a/tmp/toolsnode_preview_edit.txt") || !strings.Contains(pending, "+++ b/tmp/toolsnode_preview_edit.txt") {
		t.Fatalf("expected pending diff to contain unified diff header, got: %q", pending)
	}
	bs, _ := os.ReadFile(p)
	if string(bs) != "a\n" {
		t.Fatalf("expected file unchanged, got: %q", string(bs))
	}
}

