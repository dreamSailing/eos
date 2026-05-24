package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/dreamSailing/eos/internal/store"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	fs := store.NewFileStore("memory-records", dir)
	return NewService(fs)
}

func TestService_Add_Basic(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	rec, err := svc.Add(ctx, &MemoryRecord{
		Scope:   "project",
		Kind:    "fact",
		Content: "该项目使用 Go 1.25",
		Source:  "user",
		Tags:    []string{"go", "version"},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if rec.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if rec.Scope != "project" {
		t.Fatalf("Scope = %q, want %q", rec.Scope, "project")
	}
	if rec.Kind != "fact" {
		t.Fatalf("Kind = %q, want %q", rec.Kind, "fact")
	}
	if rec.Content != "该项目使用 Go 1.25" {
		t.Fatalf("Content = %q, want %q", rec.Content, "该项目使用 Go 1.25")
	}
	if rec.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}
	if rec.UpdatedAt.IsZero() {
		t.Fatal("expected non-zero UpdatedAt")
	}

	listed, err := svc.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List length = %d, want 1", len(listed))
	}
	if listed[0].ID != rec.ID {
		t.Fatalf("listed ID = %q, want %q", listed[0].ID, rec.ID)
	}
}

func TestService_Add_Deduplicate(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	r1, err := svc.Add(ctx, &MemoryRecord{
		Scope:   "global",
		Kind:    "preference",
		Content: "默认使用中文回答",
		Source:  "user",
	})
	if err != nil {
		t.Fatalf("first Add: %v", err)
	}

	r2, err := svc.Add(ctx, &MemoryRecord{
		Scope:   "global",
		Kind:    "preference",
		Content: "默认使用中文回答",
		Source:  "agent",
	})
	if err != nil {
		t.Fatalf("second Add: %v", err)
	}

	if r1.ID != r2.ID {
		t.Fatalf("dedup failed: IDs differ %q vs %q", r1.ID, r2.ID)
	}

	listed, err := svc.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List length = %d, want 1 after dedup", len(listed))
	}
}

func TestService_Add_DifferentScope_NotDeduped(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	r1, err := svc.Add(ctx, &MemoryRecord{
		Scope:   "global",
		Kind:    "fact",
		Content: "same content",
	})
	if err != nil {
		t.Fatalf("Add 1: %v", err)
	}

	r2, err := svc.Add(ctx, &MemoryRecord{
		Scope:   "project",
		Kind:    "fact",
		Content: "same content",
	})
	if err != nil {
		t.Fatalf("Add 2: %v", err)
	}

	if r1.ID == r2.ID {
		t.Fatal("different scope should not be deduped")
	}

	listed, err := svc.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("List length = %d, want 2", len(listed))
	}
}

func TestService_Add_EmptyContent(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Add(context.Background(), &MemoryRecord{
		Scope:   "global",
		Content: "   ",
	})
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestService_Add_EmptyScope(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Add(context.Background(), &MemoryRecord{
		Content: "something",
	})
	if err == nil {
		t.Fatal("expected error for empty scope")
	}
}

func TestService_Add_NilRecord(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Add(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil record")
	}
}

func TestService_List_FilterByScope(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	_, _ = svc.Add(ctx, &MemoryRecord{Scope: "global", Content: "a"})
	_, _ = svc.Add(ctx, &MemoryRecord{Scope: "project", Content: "b"})
	_, _ = svc.Add(ctx, &MemoryRecord{Scope: "project", Content: "c"})

	listed, err := svc.List(ctx, Filter{Scope: "project"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("List length = %d, want 2", len(listed))
	}
	for _, r := range listed {
		if r.Scope != "project" {
			t.Fatalf("unexpected scope %q", r.Scope)
		}
	}
}

func TestService_List_FilterByKind(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	_, _ = svc.Add(ctx, &MemoryRecord{Scope: "global", Kind: "fact", Content: "x"})
	_, _ = svc.Add(ctx, &MemoryRecord{Scope: "global", Kind: "preference", Content: "y"})

	listed, err := svc.List(ctx, Filter{Kind: "fact"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List length = %d, want 1", len(listed))
	}
	if listed[0].Kind != "fact" {
		t.Fatalf("Kind = %q, want %q", listed[0].Kind, "fact")
	}
}

func TestService_List_FilterByTags(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	_, _ = svc.Add(ctx, &MemoryRecord{Scope: "global", Content: "a", Tags: []string{"go", "build"}})
	_, _ = svc.Add(ctx, &MemoryRecord{Scope: "global", Content: "b", Tags: []string{"go"}})
	_, _ = svc.Add(ctx, &MemoryRecord{Scope: "global", Content: "c", Tags: []string{"python"}})

	listed, err := svc.List(ctx, Filter{Tags: []string{"go"}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("List length = %d, want 2", len(listed))
	}

	listed, err = svc.List(ctx, Filter{Tags: []string{"go", "build"}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List length = %d, want 1 (require both tags)", len(listed))
	}
}

func TestService_Search_Keywords(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	_, _ = svc.Add(ctx, &MemoryRecord{Scope: "project", Content: "该项目使用 Go 语言开发"})
	_, _ = svc.Add(ctx, &MemoryRecord{Scope: "project", Content: "Python 脚本用于数据处理"})
	_, _ = svc.Add(ctx, &MemoryRecord{Scope: "project", Content: "Go 模块路径为 github.com/dreamSailing/eos"})

	results, err := svc.Search(ctx, SearchQuery{Keywords: []string{"Go"}})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Search results = %d, want 2", len(results))
	}
}

func TestService_Search_KeywordsAndTags(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	_, _ = svc.Add(ctx, &MemoryRecord{
		Scope: "project", Content: "Go 构建配置", Tags: []string{"build"},
	})
	_, _ = svc.Add(ctx, &MemoryRecord{
		Scope: "project", Content: "Go 测试策略", Tags: []string{"test"},
	})
	_, _ = svc.Add(ctx, &MemoryRecord{
		Scope: "project", Content: "Python 构建配置", Tags: []string{"build"},
	})

	results, err := svc.Search(ctx, SearchQuery{
		Keywords: []string{"Go"},
		Tags:     []string{"build"},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search results = %d, want 1", len(results))
	}
	if results[0].Content != "Go 构建配置" {
		t.Fatalf("content = %q, want %q", results[0].Content, "Go 构建配置")
	}
}

func TestService_Search_ScopeAndKind(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	_, _ = svc.Add(ctx, &MemoryRecord{Scope: "global", Kind: "fact", Content: "alpha"})
	_, _ = svc.Add(ctx, &MemoryRecord{Scope: "project", Kind: "fact", Content: "beta"})
	_, _ = svc.Add(ctx, &MemoryRecord{Scope: "project", Kind: "preference", Content: "gamma"})

	results, err := svc.Search(ctx, SearchQuery{Scope: "project", Kind: "fact"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search results = %d, want 1", len(results))
	}
	if results[0].Content != "beta" {
		t.Fatalf("content = %q, want %q", results[0].Content, "beta")
	}
}

func TestService_Search_NoMatch(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	_, _ = svc.Add(ctx, &MemoryRecord{Scope: "global", Content: "hello world"})

	results, err := svc.Search(ctx, SearchQuery{Keywords: []string{"nonexistent"}})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Search results = %d, want 0", len(results))
	}
}

func TestService_Delete(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	rec, err := svc.Add(ctx, &MemoryRecord{Scope: "global", Content: "to be deleted"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := svc.Delete(ctx, rec.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	listed, err := svc.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("List length = %d, want 0 after delete", len(listed))
	}
}

func TestService_Delete_NotFound(t *testing.T) {
	svc := newTestService(t)
	err := svc.Delete(context.Background(), "nonexistent-id")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestService_ContextCanceled(t *testing.T) {
	svc := newTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.Add(ctx, &MemoryRecord{Scope: "global", Content: "x"})
	if err == nil {
		t.Fatal("expected error on canceled context for Add")
	}

	_, err = svc.List(ctx, Filter{})
	if err == nil {
		t.Fatal("expected error on canceled context for List")
	}

	_, err = svc.Search(ctx, SearchQuery{})
	if err == nil {
		t.Fatal("expected error on canceled context for Search")
	}

	err = svc.Delete(ctx, "x")
	if err == nil {
		t.Fatal("expected error on canceled context for Delete")
	}
}

func TestContentHash_Deterministic(t *testing.T) {
	h1 := ContentHash("global", "fact", "hello")
	h2 := ContentHash("global", "fact", "hello")
	if h1 != h2 {
		t.Fatalf("same input produced different hashes: %q vs %q", h1, h2)
	}

	h3 := ContentHash("global", "fact", "different")
	if h1 == h3 {
		t.Fatal("different content produced same hash")
	}

	h4 := ContentHash("project", "fact", "hello")
	if h1 == h4 {
		t.Fatal("different scope produced same hash")
	}
}

func TestService_PreservesExistingTimestamps(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	rec, err := svc.Add(ctx, &MemoryRecord{
		Scope:     "global",
		Content:   "timestamp test",
		WorkspaceRoot: "/tmp/ws",
		SessionID: "sess-1",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if rec.WorkspaceRoot != "/tmp/ws" {
		t.Fatalf("WorkspaceRoot = %q, want %q", rec.WorkspaceRoot, "/tmp/ws")
	}
	if rec.SessionID != "sess-1" {
		t.Fatalf("SessionID = %q, want %q", rec.SessionID, "sess-1")
	}

	listed, err := svc.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List length = %d, want 1", len(listed))
	}
	if listed[0].WorkspaceRoot != "/tmp/ws" {
		t.Fatalf("WorkspaceRoot roundtrip = %q", listed[0].WorkspaceRoot)
	}
}

func TestService_MultipleRecords(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		_, err := svc.Add(ctx, &MemoryRecord{
			Scope:   "project",
			Kind:    "fact",
			Content: "record-" + string(rune('a'+i)),
		})
		if err != nil {
			t.Fatalf("Add %d: %v", i, err)
		}
	}

	listed, err := svc.List(ctx, Filter{Scope: "project"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 10 {
		t.Fatalf("List length = %d, want 10", len(listed))
	}
}
