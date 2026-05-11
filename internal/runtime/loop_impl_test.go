package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"errors"
	"testing"
)

func TestSlidingWindowLoopDetector_SameReadArgsForceBreak(t *testing.T) {
	d := NewSlidingWindowLoopDetector()
	args := map[string]interface{}{"path": "internal/ui/panels_context.go"}
	var err error
	for i := 0; i < 4; i++ {
		err = d.CheckLoop("read", args)
	}
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrLoopForceBreak) {
		t.Fatalf("expected ErrLoopForceBreak, got %v", err)
	}
}

func TestSlidingWindowLoopDetector_CheckLoopResultIncludesWrapUp(t *testing.T) {
	d := NewSlidingWindowLoopDetector()
	args := map[string]interface{}{"path": "internal/ui/panels_context.go"}
	var result *LoopCheckResult
	for i := 0; i < 4; i++ {
		result = d.CheckLoopResult("read", args)
	}
	if result == nil {
		t.Fatal("expected structured result, got nil")
	}
	if result.Level != LoopLevelForceBreak {
		t.Fatalf("Level=%q, want %q", result.Level, LoopLevelForceBreak)
	}
	if !result.WrapUpRequired {
		t.Fatal("expected WrapUpRequired to be true")
	}
	if result.ToolName != "read" {
		t.Fatalf("ToolName=%q, want read", result.ToolName)
	}
}

