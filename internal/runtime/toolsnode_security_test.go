package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"github.com/dreamSailing/eos/internal/session"
	"github.com/dreamSailing/eos/internal/tools"
)

func TestToolsNode_SafetyGateBlocksFsWrite(t *testing.T) {
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

	tools.SafetyGateClassify = func(call tools.ToolCall) (string, string, string, bool) {
		return tools.ClassifyToolDanger(call)
	}
	tools.SafetyGateSessionAllowed = func(category string) bool { return false }
	tools.SafetyGateAllowSession = func(category string) {}
	tools.SetPendingDiff = func(diff string) {}
	tools.SafetyGatePrompt = func(ctx context.Context, category, summary string) string {
		return "deny"
	}

	cm := session.NewContextManager()
	mgr := tools.NewManager()
	rt := &EinoRuntime{ctxm: cm, tools: mgr, recentToolCalls: map[string]int{}, recentAssistantHashes: map[string]int{}}

	wd, _ := os.Getwd()
	p := filepath.Join(wd, "tmp", "toolsnode_security_test.txt")
	_ = os.Remove(p)
	defer func() { _ = os.Remove(p) }()

	payload := `{"tool":"fs","parameters":{"mode":"write","path":"tmp/toolsnode_security_test.txt","content":"x"}}`
	_, executed, _ := rt.ToolsNode(context.Background(), payload)
	if !executed {
		t.Fatalf("expected tools node to parse and attempt execution")
	}
	if _, err := os.Stat(p); err == nil {
		t.Fatalf("expected fs write to be blocked, but file exists: %s", p)
	}
}

