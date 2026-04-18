package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import "testing"

func TestGitShow_ToolDefinitionRegistered(t *testing.T) {
	def, ok := GetToolDefinition(ToolGitShow)
	if !ok {
		t.Fatalf("tool definition not found: %s", ToolGitShow)
	}
	if def.Name != ToolGitShow {
		t.Fatalf("unexpected tool name: %s", def.Name)
	}
	if def.RiskLevel != RiskLevelLow {
		t.Fatalf("unexpected risk level: %v", def.RiskLevel)
	}
}

func TestGitShow_ManagerStructuredRegistered(t *testing.T) {
	m := NewManager()
	if _, ok := m.structured[ToolGitShow]; !ok {
		t.Fatalf("tool handler not registered: %s", ToolGitShow)
	}
}
