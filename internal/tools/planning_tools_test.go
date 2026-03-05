package tools

import (
	"context"
	"testing"
)

func TestTodoWriteAcceptsTextAlias(t *testing.T) {
	m := NewManager()
	res := m.todoWriteStructured(context.Background(), map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"id": "1", "text": "hello", "status": "pending"},
		},
	})
	if res.Status != "success" {
		t.Fatalf("expected success, got status=%s err=%s", res.Status, res.Error)
	}
	rd := m.todoReadStructured(context.Background(), map[string]interface{}{})
	items, _ := rd.Data["items"].([]map[string]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0]["content"] != "hello" {
		t.Fatalf("expected content=hello, got %v", items[0]["content"])
	}
}

