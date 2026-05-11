package runtime

import (
	"context"
	"strings"
	"testing"

	"github.com/dreamSailing/eos/internal/session"
	"github.com/dreamSailing/eos/internal/tools"
)

func TestToolsNode_ActivatesWrapUpAfterRepeatedRead(t *testing.T) {
	cm := session.NewContextManager()
	mgr := tools.NewManager()
	rt := &EinoRuntime{
		ctxm:                  cm,
		tools:                 mgr,
		recentToolCalls:       map[string]int{},
		recentAssistantHashes: map[string]int{},
		loopDetector:          NewSlidingWindowLoopDetector(),
	}

	var meta []string
	rt.WithOnMeta(func(line string) {
		meta = append(meta, line)
	})

	payload := `{"tool":"read","parameters":{"path":"."}}`
	for i := 0; i < 3; i++ {
		_, executed, _ := rt.ToolsNode(context.Background(), payload)
		if !executed {
			t.Fatalf("expected execution on round %d", i+1)
		}
	}

	results, executed, wantContinue := rt.ToolsNode(context.Background(), payload)
	if executed {
		t.Fatal("expected repeated call to stop before executing tool")
	}
	if wantContinue {
		t.Fatal("wantContinue=true, want false after wrap-up activation")
	}
	if !rt.turnWrapUp.Active {
		t.Fatal("expected turnWrapUp to become active")
	}
	foundLoopBlock := false
	for _, item := range results {
		if strings.HasPrefix(item, EventLoopBlock+":") {
			foundLoopBlock = true
			break
		}
	}
	if !foundLoopBlock {
		t.Fatalf("expected loop block event in results, got %v", results)
	}

	foundWrapUpMeta := false
	for _, line := range meta {
		if strings.Contains(line, EventTurnWrapUp) {
			foundWrapUpMeta = true
			break
		}
	}
	if !foundWrapUpMeta {
		t.Fatalf("expected %q meta event, got %v", EventTurnWrapUp, meta)
	}
}
