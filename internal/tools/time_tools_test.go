package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"testing"
)

func TestTimeNowStructured(t *testing.T) {
	m := NewManager()
	r := m.timeNowStructured(context.Background(), map[string]interface{}{})
	if r.Status != "success" {
		t.Fatalf("expected success, got %s (%s)", r.Status, r.Error)
	}
	if r.Tool != ToolTimeNow {
		t.Fatalf("expected tool %s, got %s", ToolTimeNow, r.Tool)
	}
	if r.Data == nil {
		t.Fatalf("expected data")
	}
	if _, ok := r.Data["local_rfc3339"].(string); !ok {
		t.Fatalf("missing local_rfc3339")
	}
	if _, ok := r.Data["unix_seconds"]; !ok {
		t.Fatalf("missing unix_seconds")
	}
}

