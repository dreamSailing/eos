package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import "testing"

func TestClassifyToolDanger_WriteIsDangerous(t *testing.T) {
	_, _, _, dangerous := ClassifyToolDanger(ToolCall{
		Tool: "fs",
		Parameters: map[string]interface{}{
			"mode": "write",
			"path": "a.txt",
		},
	})
	if !dangerous {
		t.Fatalf("expected dangerous=true")
	}
}

func TestClassifyToolDanger_EditIsDangerous(t *testing.T) {
	_, _, _, dangerous := ClassifyToolDanger(ToolCall{
		Tool: ToolEdit,
		Parameters: map[string]interface{}{
			"mode": "single",
		},
	})
	if !dangerous {
		t.Fatalf("expected dangerous=true")
	}
}

