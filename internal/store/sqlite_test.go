package store

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func newTestSQLiteStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := NewSQLiteStore("test", dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSQLiteStore_NameAndRoot(t *testing.T) {
	s := newTestSQLiteStore(t)

	if got := s.Name(); got != "test" {
		t.Fatalf("Name() = %q, want %q", got, "test")
	}
	if root := s.Root(); root == "" {
		t.Fatal("Root() should not be empty")
	}
}

func TestSQLiteStore_NewSQLiteStore_EmptyName(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	_, err := NewSQLiteStore("", dbPath)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestSQLiteStore_NewSQLiteStore_EmptyPath(t *testing.T) {
	_, err := NewSQLiteStore("test", "")
	if err == nil {
		t.Fatal("expected error for empty dbPath")
	}
}

func TestSQLiteStore_ReadWriteFile(t *testing.T) {
	s := newTestSQLiteStore(t)

	payload := []byte(`hello world`)
	if err := s.WriteFile("greeting.txt", payload); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := s.ReadFile("greeting.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("content mismatch: got %q, want %q", string(got), string(payload))
	}
}

func TestSQLiteStore_ReadFile_NotFound(t *testing.T) {
	s := newTestSQLiteStore(t)

	_, err := s.ReadFile("nonexistent.txt")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestSQLiteStore_WriteFile_Overwrite(t *testing.T) {
	s := newTestSQLiteStore(t)

	if err := s.WriteFile("data.txt", []byte("v1")); err != nil {
		t.Fatalf("WriteFile v1: %v", err)
	}
	if err := s.WriteFile("data.txt", []byte("v2")); err != nil {
		t.Fatalf("WriteFile v2: %v", err)
	}

	got, err := s.ReadFile("data.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "v2" {
		t.Fatalf("overwrite failed: got %q, want %q", string(got), "v2")
	}
}

func TestSQLiteStore_WriteFileAtomic(t *testing.T) {
	s := newTestSQLiteStore(t)

	payload := []byte(`{"atomic": true}`)
	if err := s.WriteFileAtomic("config/settings.json", payload); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	got, err := s.ReadFile("config/settings.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("content mismatch: got %q, want %q", string(got), string(payload))
	}
}

func TestSQLiteStore_ExistsAndRemove(t *testing.T) {
	s := newTestSQLiteStore(t)

	if s.Exists("missing.txt") {
		t.Fatal("Exists should return false for missing file")
	}

	if err := s.WriteFile("present.txt", []byte("data")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if !s.Exists("present.txt") {
		t.Fatal("Exists should return true after write")
	}

	if err := s.Remove("present.txt"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if s.Exists("present.txt") {
		t.Fatal("Exists should return false after remove")
	}
}

func TestSQLiteStore_Remove_NotFound(t *testing.T) {
	s := newTestSQLiteStore(t)

	err := s.Remove("nonexistent.txt")
	if err == nil {
		t.Fatal("expected error for removing missing file")
	}
}

func TestSQLiteStore_ListFiles(t *testing.T) {
	s := newTestSQLiteStore(t)

	_ = s.WriteFile("a.json", []byte("1"))
	_ = s.WriteFile("b.json", []byte("2"))
	_ = s.WriteFile("c.txt", []byte("3"))

	all, err := s.ListFiles("")
	if err != nil {
		t.Fatalf("ListFiles(\"\"): %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 files, got %d: %v", len(all), all)
	}

	jsons, err := s.ListFiles(".json")
	if err != nil {
		t.Fatalf("ListFiles(\".json\"): %v", err)
	}
	if len(jsons) != 2 {
		t.Fatalf("expected 2 json files, got %d: %v", len(jsons), jsons)
	}
}

func TestSQLiteStore_ListFiles_Empty(t *testing.T) {
	s := newTestSQLiteStore(t)

	files, err := s.ListFiles("")
	if err != nil {
		t.Fatalf("ListFiles on empty store: %v", err)
	}
	if files != nil {
		t.Fatalf("expected nil, got %v", files)
	}
}

func TestSQLiteStore_ReadWriteJSON(t *testing.T) {
	s := newTestSQLiteStore(t)

	type item struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	original := item{Name: "hello", Value: 42}
	if err := s.WriteJSON("data/item.json", &original); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var loaded item
	if err := s.ReadJSON("data/item.json", &loaded); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if loaded.Name != original.Name || loaded.Value != original.Value {
		t.Fatalf("roundtrip mismatch: got %+v, want %+v", loaded, original)
	}
}

func TestSQLiteStore_JSONRoundtrip(t *testing.T) {
	s := newTestSQLiteStore(t)

	type record struct {
		ID      string            `json:"id"`
		Tags    []string          `json:"tags,omitempty"`
		Metrics map[string]float64 `json:"metrics,omitempty"`
	}

	original := record{
		ID:      "r1",
		Tags:    []string{"alpha", "beta"},
		Metrics: map[string]float64{"score": 0.95},
	}
	if err := s.WriteJSON("records/r1.json", &original); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	raw, err := s.ReadFile("records/r1.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var pretty json.RawMessage
	if err := json.Unmarshal(raw, &pretty); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if !strings.Contains(string(raw), "  ") {
		t.Fatal("WriteJSON should produce indented output")
	}

	var loaded record
	if err := s.ReadJSON("records/r1.json", &loaded); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if loaded.ID != original.ID {
		t.Fatalf("ID mismatch: %q vs %q", loaded.ID, original.ID)
	}
	if len(loaded.Tags) != len(original.Tags) {
		t.Fatalf("Tags length mismatch: %d vs %d", len(loaded.Tags), len(original.Tags))
	}
}

func TestSQLiteStore_WriteJSONAtomic(t *testing.T) {
	s := newTestSQLiteStore(t)

	type cfg struct {
		Debug bool `json:"debug"`
	}
	original := cfg{Debug: true}
	if err := s.WriteJSONAtomic("config.json", &original); err != nil {
		t.Fatalf("WriteJSONAtomic: %v", err)
	}

	var loaded cfg
	if err := s.ReadJSON("config.json", &loaded); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if loaded.Debug != original.Debug {
		t.Fatalf("roundtrip mismatch: got %+v, want %+v", loaded, original)
	}
}

func TestSQLiteStore_WriteJSON_MarshalError(t *testing.T) {
	s := newTestSQLiteStore(t)

	badSrc := make(chan int)
	if err := s.WriteJSON("bad.json", badSrc); err == nil {
		t.Fatal("expected marshal error for unmarshalable type")
	}
}

func TestSQLiteStore_ReadJSON_NotFound(t *testing.T) {
	s := newTestSQLiteStore(t)

	var dest any
	if err := s.ReadJSON("missing.json", &dest); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestSQLiteStore_ConcurrentReadWrite(t *testing.T) {
	s := newTestSQLiteStore(t)

	const goroutines = 20
	const writesPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < writesPerGoroutine; i++ {
				key := filepath.Join("concurrent", "g"+string(rune('A'+id))+".txt")
				payload := []byte(strings.Repeat("x", 100))
				if err := s.WriteFile(key, payload); err != nil {
					t.Errorf("goroutine %d write %d: %v", id, i, err)
					return
				}
				if _, err := s.ReadFile(key); err != nil {
					t.Errorf("goroutine %d read %d: %v", id, i, err)
					return
				}
			}
		}(g)
	}
	wg.Wait()
}

func TestSQLiteStore_ConcurrentExistsAndRemove(t *testing.T) {
	s := newTestSQLiteStore(t)

	const count = 100
	for i := 0; i < count; i++ {
		key := filepath.Join("bulk", "file"+string(rune('0'+i%10))+".txt")
		if err := s.WriteFile(key, []byte("data")); err != nil {
			t.Fatalf("setup write %d: %v", i, err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func(id int) {
			defer wg.Done()
			key := filepath.Join("bulk", "file"+string(rune('0'+id%10))+".txt")
			s.Exists(key)
			if id%3 == 0 {
				s.Remove(key)
			}
		}(i)
	}
	wg.Wait()
}

func TestSQLiteStore_ConcurrentJSON(t *testing.T) {
	s := newTestSQLiteStore(t)

	type entry struct {
		Seq int `json:"seq"`
	}

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				e := entry{Seq: id*1000 + i}
				if err := s.WriteJSON("seq.json", &e); err != nil {
					t.Errorf("goroutine %d writeJSON: %v", id, err)
					return
				}
				var loaded entry
				if err := s.ReadJSON("seq.json", &loaded); err != nil {
					t.Errorf("goroutine %d readJSON: %v", id, err)
					return
				}
			}
		}(g)
	}
	wg.Wait()
}

func TestSQLiteStore_ImplementsReadWriteStore(t *testing.T) {
	var _ ReadWriteStore = (*SQLiteStore)(nil)
}

func TestSQLiteStore_NestedPaths(t *testing.T) {
	s := newTestSQLiteStore(t)

	paths := []string{
		"a.txt",
		"dir/b.txt",
		"dir/sub/c.txt",
		"deep/nested/path/d.txt",
	}

	for _, p := range paths {
		if err := s.WriteFile(p, []byte("content")); err != nil {
			t.Fatalf("WriteFile(%q): %v", p, err)
		}
	}

	for _, p := range paths {
		if !s.Exists(p) {
			t.Fatalf("Exists(%q) = false, want true", p)
		}
		got, err := s.ReadFile(p)
		if err != nil {
			t.Fatalf("ReadFile(%q): %v", p, err)
		}
		if string(got) != "content" {
			t.Fatalf("ReadFile(%q) = %q, want %q", p, string(got), "content")
		}
	}
}

func TestSQLiteStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "persist.db")

	s1, err := NewSQLiteStore("persist", dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	if err := s1.WriteFile("key.txt", []byte("value")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	s1.Close()

	s2, err := NewSQLiteStore("persist", dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore reopen: %v", err)
	}
	defer s2.Close()

	got, err := s2.ReadFile("key.txt")
	if err != nil {
		t.Fatalf("ReadFile after reopen: %v", err)
	}
	if string(got) != "value" {
		t.Fatalf("persistence failed: got %q, want %q", string(got), "value")
	}
}

func TestSQLiteStore_LargeData(t *testing.T) {
	s := newTestSQLiteStore(t)

	big := make([]byte, 1<<20)
	for i := range big {
		big[i] = byte(i % 256)
	}

	if err := s.WriteFile("big.bin", big); err != nil {
		t.Fatalf("WriteFile large: %v", err)
	}

	got, err := s.ReadFile("big.bin")
	if err != nil {
		t.Fatalf("ReadFile large: %v", err)
	}
	if len(got) != len(big) {
		t.Fatalf("size mismatch: got %d, want %d", len(got), len(big))
	}
	for i := range big {
		if got[i] != big[i] {
			t.Fatalf("byte mismatch at %d", i)
		}
	}
}
