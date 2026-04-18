package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import "testing"

func drainEvents(ch <-chan Event) []Event {
	var out []Event
	for {
		select {
		case e := <-ch:
			out = append(out, e)
		default:
			return out
		}
	}
}

func TestStreamParser_TaskAndFinalSingleChunk(t *testing.T) {
	ch := make(chan Event, 16)
	p := NewStreamParser(ch)
	p.Reset()
	p.Process("agent.task:senior-dev 你好啊agent.final:你好！有什么我可以帮助你的吗？")
	p.Flush()

	events := drainEvents(ch)
	var tasks []Event
	var finals []Event
	for _, e := range events {
		switch e.Type {
		case "agent.progress":
			tasks = append(tasks, e)
		case "agent.completed":
			finals = append(finals, e)
		}
	}
	if len(tasks) != 1 || tasks[0].RID != "senior-dev" {
		t.Fatalf("expected 1 agent.progress with RID senior-dev, got %+v", tasks)
	}
	if len(finals) != 1 || finals[0].RID != "senior-dev" {
		t.Fatalf("expected 1 agent.completed with RID senior-dev, got %+v", finals)
	}
	if tasks[0].Content != "你好啊" {
		t.Fatalf("expected task content 你好啊, got %q", tasks[0].Content)
	}
	if finals[0].Content != "你好！有什么我可以帮助你的吗？" {
		t.Fatalf("expected final content, got %q", finals[0].Content)
	}
}

func TestStreamParser_PartialMarkerDoesNotLeak(t *testing.T) {
	ch := make(chan Event, 32)
	p := NewStreamParser(ch)
	p.Reset()
	p.Process("a")
	p.Process("g")
	p.Process("e")
	p.Process("n")
	p.Process("t")
	p.Process(".")
	p.Process("t")
	p.Process("a")
	p.Process("s")
	p.Process("k")
	p.Process(":")
	p.Process("senior-dev hi")
	p.Process("agent.final:done")
	p.Flush()

	events := drainEvents(ch)
	for _, e := range events {
		if e.Type != "text.delta" {
			continue
		}
		if e.Content == "agent.task:" || e.Content == "agent.final:" {
			t.Fatalf("marker leaked as delta: %+v", e)
		}
	}
}
