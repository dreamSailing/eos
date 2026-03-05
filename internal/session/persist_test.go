package session

import (
	"testing"

	"github.com/dreamSailing/vb-coding/internal/ai"
)

func TestContextManager_ExportImportStateRoundTrip(t *testing.T) {
	cm := NewContextManager()
	cm.SetModel("gpt-4o")
	cm.AddPinned(ai.Message{Role: "system", Content: "p"})
	cm.AddUser("u1")
	cm.AddAssistant("a1")
	cm.AddToolSummary("tool summary")

	st := cm.ExportState()

	cm2 := NewContextManager()
	cm2.ImportState(st)

	got := cm2.BuildPreview()
	if len(got) == 0 {
		t.Fatalf("expected messages")
	}
	if st.ModelName != "gpt-4o" {
		t.Fatalf("expected model name saved, got %q", st.ModelName)
	}
}
