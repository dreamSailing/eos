package codectx

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"os"
	"path/filepath"
	"sync"
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

func TestMultiEngineConcurrentAccessDoesNotPanic(t *testing.T) {
	wd, _ := os.Getwd()
	roots := []string{
		filepath.Join(wd, "testdata", "concurrent-ws1"),
		filepath.Join(wd, "testdata", "concurrent-ws2"),
		filepath.Join(wd, "testdata", "concurrent-ws3"),
	}
	for _, root := range roots {
		_ = os.MkdirAll(root, 0o755)
	}

	m := NewMultiEngine()
	for _, root := range roots {
		_ = m.AddRoot(root)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				root := roots[(id+j)%len(roots)]
				switch j % 4 {
				case 0:
					_ = m.AddRoot(root)
				case 1:
					_ = m.Roots()
					_ = m.Active()
				case 2:
					_ = m.SetActive(root)
				case 3:
					m.RemoveRoot(root)
					_ = m.AddRoot(root)
				}
			}
		}(i)
	}
	wg.Wait()
}
