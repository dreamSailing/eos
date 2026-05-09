package session

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"testing"

	"github.com/dreamSailing/eos/internal/ai"
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
