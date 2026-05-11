package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"strings"
	"testing"
	"github.com/dreamSailing/eos/internal/tools"
)

func TestAllowedTools_IncludeGitShow(t *testing.T) {
	roles := []string{"architect", "planner", "reviewer", "tester", "verification", "senior-dev"}
	for _, role := range roles {
		allowed := AllowedTools(role)
		if !allowed[tools.ToolGitShow] {
			t.Fatalf("role %s missing %s", role, tools.ToolGitShow)
		}
		if !allowed[strings.ToLower(tools.ToolProjectStructure)] {
			t.Fatalf("role %s missing %s", role, tools.ToolProjectStructure)
		}
	}
}
