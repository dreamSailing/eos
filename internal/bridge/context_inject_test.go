package bridge

import (
	"strings"
	"testing"

	codectx "github.com/dreamSailing/vb-coding/internal/context"
)

func TestExtractMentionedPaths(t *testing.T) {
	got := extractMentionedPaths("please check @internal/bridge/runtime_invoke.go and docs/iteration_optimization_plan.md")
	if len(got) < 2 {
		t.Fatalf("expected paths, got %#v", got)
	}
}

func TestExtractRelevantSnippetCentersOnKeyword(t *testing.T) {
	content := strings.Repeat("line\n", 200) + "targetKeyword here\n" + strings.Repeat("tail\n", 200)
	out := extractRelevantSnippet(content, "targetKeyword", nil, 2000)
	if !strings.Contains(out, "targetKeyword") {
		t.Fatalf("expected keyword in snippet")
	}
	if len(out) > 2100 {
		t.Fatalf("snippet too large: %d", len(out))
	}
}

func TestBuildInjectCandidatesDedup(t *testing.T) {
	sugg := []codectx.Suggestion{
		{Path: "internal/bridge/runtime_invoke.go", Symbols: []string{"ProcessContextHints"}},
		{Path: "internal/bridge/runtime_invoke.go", Symbols: []string{"ProcessContextHints"}},
	}
	out := buildInjectCandidates("runtime_invoke.go", sugg, 4)
	seen := map[string]struct{}{}
	for _, s := range out {
		if _, ok := seen[s.Path]; ok {
			t.Fatalf("duplicate path: %s", s.Path)
		}
		seen[s.Path] = struct{}{}
	}
}
