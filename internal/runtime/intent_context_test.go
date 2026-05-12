package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

func TestPlanIntentContext_ExplicitPathWinsOverAutomaticSignals(t *testing.T) {
	dir := t.TempDir()
	writeVersionedContent(t, dir, "internal/runtime/loop_impl.go", "20260512-101010.000000001.content", -2*time.Hour)
	writeVersionedContent(t, dir, "internal/runtime/loop_impl_test.go", "20260512-101010.000000002.content", -1*time.Hour)

	plan := planIntentContext(dir, "请修改 internal/runtime/orchestration.go 的调度逻辑，并顺便看看 loop_impl")
	if plan.Decision != IntentContextDecisionPreferExplicit {
		t.Fatalf("Decision = %q, want %q", plan.Decision, IntentContextDecisionPreferExplicit)
	}
	if !plan.HasExplicitSignals {
		t.Fatalf("expected explicit signals to be detected")
	}
	if len(plan.Candidates) == 0 || plan.Candidates[0].Path != "internal/runtime/orchestration.go" {
		t.Fatalf("first candidate = %+v, want explicit path first", plan.Candidates)
	}
	if plan.Fallback != IntentContextFallbackNone {
		t.Fatalf("Fallback = %q, want none", plan.Fallback)
	}
}

func TestPlanIntentContext_CloseCandidatesTriggerClarify(t *testing.T) {
	dir := t.TempDir()
	writeSourceFile(t, dir, "internal/runtime/loop_impl.go", "package runtime\n\nfunc DetectLoop() {}\n")
	writeSourceFile(t, dir, "internal/runtime/loop_impl_test.go", "package runtime\n\nfunc TestDetectLoop() {}\n")
	writeVersionedContent(t, dir, "internal/runtime/loop_impl.go", "20260512-101010.000000001.content", -2*time.Hour)
	writeVersionedContent(t, dir, "internal/runtime/loop_impl_test.go", "20260512-101010.000000002.content", -1*time.Hour)

	plan := planIntentContext(dir, "请检查 loop_impl 相关逻辑")
	if plan.Decision != IntentContextDecisionClarify {
		t.Fatalf("Decision = %q, want %q", plan.Decision, IntentContextDecisionClarify)
	}
	if plan.Fallback != IntentContextFallbackClarify {
		t.Fatalf("Fallback = %q, want %q", plan.Fallback, IntentContextFallbackClarify)
	}
	if len(plan.Candidates) < 2 {
		t.Fatalf("expected at least 2 candidates, got %+v", plan.Candidates)
	}
}

func TestPlanIntentContext_NaturalLanguageRecallUsesContextEngine(t *testing.T) {
	dir := t.TempDir()
	writeSourceFile(t, dir, "internal/runtime/runtime_auth.go", "package runtime\n\nfunc LoginSession() {}\nfunc ValidateToken() {}\n")
	writeSourceFile(t, dir, "internal/runtime/context_env.go", "package runtime\n\nfunc LoadContextEnv() {}\n")
	writeVersionedContent(t, dir, "internal/runtime/runtime_auth.go", "20260512-101010.000000001.content", -1*time.Hour)

	plan := planIntentContext(dir, "登录鉴权那块有问题，帮我排查一下")
	if plan.Decision != IntentContextDecisionAutoLocate {
		t.Fatalf("Decision = %q, want %q", plan.Decision, IntentContextDecisionAutoLocate)
	}
	if len(plan.Candidates) == 0 || plan.Candidates[0].Path != "internal/runtime/runtime_auth.go" {
		t.Fatalf("first candidate = %+v, want runtime_auth.go", plan.Candidates)
	}
	if !containsIntentSource(plan.Candidates[0].Sources, IntentContextSourceIntentRecall) {
		t.Fatalf("expected intent recall source on top candidate, got %+v", plan.Candidates[0].Sources)
	}
	if plan.Confidence < 0.56 {
		t.Fatalf("Confidence = %f, want confident auto locate", plan.Confidence)
	}
}

func TestPlanIntentContext_VagueQueryFallsBackToBroadSearch(t *testing.T) {
	dir := t.TempDir()
	writeVersionedContent(t, dir, "internal/runtime/context_env.go", "20260512-101010.000000001.content", -1*time.Hour)

	plan := planIntentContext(dir, "这块有点奇怪，帮我看看")
	if plan.Decision != IntentContextDecisionBroadSearch {
		t.Fatalf("Decision = %q, want %q", plan.Decision, IntentContextDecisionBroadSearch)
	}
	if plan.Fallback != IntentContextFallbackBroadSearch {
		t.Fatalf("Fallback = %q, want %q", plan.Fallback, IntentContextFallbackBroadSearch)
	}
	if len(plan.Candidates) == 0 {
		t.Fatalf("expected low-confidence fallback candidates")
	}
	if plan.Confidence >= 0.55 {
		t.Fatalf("Confidence = %f, want low confidence", plan.Confidence)
	}
}

func TestBuildIntentPromptAdditions_EmbedsDecisionAndCandidates(t *testing.T) {
	dir := t.TempDir()
	writeVersionedContent(t, dir, "internal/runtime/loop_impl.go", "20260512-101010.000000001.content", -1*time.Hour)

	history := []*schema.Message{
		schema.UserMessage("请修改 internal/runtime/loop_impl.go，修复循环检测"),
	}
	text := buildIntentPromptAdditions(dir, history)
	if !strings.Contains(text, "**意图感知上下文编排**") {
		t.Fatalf("prompt missing orchestration section: %q", text)
	}
	if !strings.Contains(text, "决策: prefer_explicit") {
		t.Fatalf("prompt missing explicit decision: %q", text)
	}
	if !strings.Contains(text, "internal/runtime/loop_impl.go") {
		t.Fatalf("prompt missing candidate path: %q", text)
	}
}

func writeVersionedContent(t *testing.T, root string, relPath string, versionFile string, age time.Duration) {
	t.Helper()
	dir := filepath.Join(root, ".eos", "versions", filepath.FromSlash(relPath))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", dir, err)
	}
	fullPath := filepath.Join(dir, versionFile)
	if err := os.WriteFile(fullPath, []byte("content\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", fullPath, err)
	}
	ts := time.Now().Add(age)
	if err := os.Chtimes(fullPath, ts, ts); err != nil {
		t.Fatalf("Chtimes(%q) error = %v", fullPath, err)
	}
}

func writeSourceFile(t *testing.T, root string, relPath string, content string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", fullPath, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", fullPath, err)
	}
}

func containsIntentSource(values []IntentContextSource, target IntentContextSource) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
