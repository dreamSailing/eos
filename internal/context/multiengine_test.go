package codectx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMultiEngineAddUseRemove(t *testing.T) {
	wd, _ := os.Getwd()
	tmp1 := filepath.Join(wd, "testdata", "ws1")
	tmp2 := filepath.Join(wd, "testdata", "ws2")
	_ = os.MkdirAll(tmp1, 0755)
	_ = os.MkdirAll(tmp2, 0755)

	m := NewMultiEngine()
	e1 := m.AddRoot(tmp1)
	if e1 == nil || e1.Root != tmp1 {
		t.Fatalf("expected engine for %s", tmp1)
	}
	e2 := m.AddRoot(tmp2)
	if e2 == nil || e2.Root != tmp2 {
		t.Fatalf("expected engine for %s", tmp2)
	}

	if len(m.Roots()) < 2 {
		t.Fatalf("expected >=2 roots, got %d", len(m.Roots()))
	}

	if a := m.Active(); a == nil {
		t.Fatalf("expected active engine")
	}

	if m.SetActive(tmp2) == nil || m.Active().Root != tmp2 {
		t.Fatalf("expected active=%s", tmp2)
	}

	m.RemoveRoot(tmp2)
	if m.Active() != nil && m.Active().Root == tmp2 {
		t.Fatalf("expected active switched away from %s", tmp2)
	}
}
