package core

import "testing"

func TestToRuntimeMode(t *testing.T) {
	if got := toRuntimeMode("手动确认"); got != "manual" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := toRuntimeMode("计划优先"); got != "plan" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := toRuntimeMode("自动无人值守"); got != "auto" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := toRuntimeMode("unknown"); got != "manual" {
		t.Fatalf("unexpected mode: %s", got)
	}
}

func TestFromRuntimeMode(t *testing.T) {
	if got := fromRuntimeMode("manual"); got != "手动确认" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := fromRuntimeMode("plan"); got != "计划优先" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := fromRuntimeMode("auto"); got != "自动无人值守" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := fromRuntimeMode("unknown"); got != "手动确认" {
		t.Fatalf("unexpected mode: %s", got)
	}
}
