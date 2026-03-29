package core

import (
	"path/filepath"
	"reflect"
	"testing"
)

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
	if got := toRuntimeMode("unknown"); got != "auto" {
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
	if got := fromRuntimeMode("unknown"); got != "自动无人值守" {
		t.Fatalf("unexpected mode: %s", got)
	}
}

func TestFilterTrustedWorkspaces(t *testing.T) {
	target := filepath.Join("C:", "Users", "tester", "demo")
	trusted := []string{
		target,
		filepath.Join("C:", "Users", "tester", "keep"),
	}
	filtered, changed := filterTrustedWorkspaces(trusted, filepath.Join("C:", "Users", "tester", "demo", "."))
	if !changed {
		t.Fatal("expected target workspace to be removed from trusted list")
	}
	want := []string{filepath.Join("C:", "Users", "tester", "keep")}
	if !reflect.DeepEqual(filtered, want) {
		t.Fatalf("filtered=%v, want %v", filtered, want)
	}
}

func TestFilterTrustedWorkspacesNoMatch(t *testing.T) {
	trusted := []string{filepath.Join("C:", "Users", "tester", "keep")}
	filtered, changed := filterTrustedWorkspaces(trusted, filepath.Join("C:", "Users", "tester", "demo"))
	if changed {
		t.Fatal("expected no trusted workspace removal")
	}
	if !reflect.DeepEqual(filtered, trusted) {
		t.Fatalf("filtered=%v, want %v", filtered, trusted)
	}
}
