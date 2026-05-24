package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileStore_ReadWriteJSON(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore("test", dir)

	type item struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	original := item{Name: "hello", Value: 42}
	if err := fs.WriteJSON("data/item.json", &original); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var loaded item
	if err := fs.ReadJSON("data/item.json", &loaded); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if loaded.Name != original.Name || loaded.Value != original.Value {
		t.Fatalf("roundtrip mismatch: got %+v, want %+v", loaded, original)
	}
}

func TestFileStore_WriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore("test", dir)

	payload := []byte(`{"atomic": true}`)
	if err := fs.WriteFileAtomic("config/settings.json", payload); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	got, err := fs.ReadFile("config/settings.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("content mismatch: got %q, want %q", string(got), string(payload))
	}

	tmpPath := filepath.Join(dir, "config", "settings.json.tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatalf("tmp file should not exist after atomic write")
	}
}

func TestFileStore_ExistsAndRemove(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore("test", dir)

	if fs.Exists("missing.json") {
		t.Fatal("Exists should return false for missing file")
	}

	if err := fs.WriteFile("present.json", []byte("data")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if !fs.Exists("present.json") {
		t.Fatal("Exists should return true after write")
	}

	if err := fs.Remove("present.json"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if fs.Exists("present.json") {
		t.Fatal("Exists should return false after remove")
	}
}

func TestFileStore_ListFiles(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore("test", dir)

	_ = fs.WriteFile("a.json", []byte("1"))
	_ = fs.WriteFile("b.json", []byte("2"))
	_ = fs.WriteFile("c.txt", []byte("3"))

	all, err := fs.ListFiles("")
	if err != nil {
		t.Fatalf("ListFiles(\"\"): %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 files, got %d: %v", len(all), all)
	}

	jsons, err := fs.ListFiles(".json")
	if err != nil {
		t.Fatalf("ListFiles(\".json\"): %v", err)
	}
	if len(jsons) != 2 {
		t.Fatalf("expected 2 json files, got %d: %v", len(jsons), jsons)
	}
}

func TestFileStore_ListFiles_MissingDir(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore("test", filepath.Join(dir, "nonexistent"))

	files, err := fs.ListFiles("")
	if err != nil {
		t.Fatalf("ListFiles on missing dir: %v", err)
	}
	if files != nil {
		t.Fatalf("expected nil, got %v", files)
	}
}

func TestFileStore_NameAndRoot(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore("sessions", dir)

	if got := fs.Name(); got != "sessions" {
		t.Fatalf("Name() = %q, want %q", got, "sessions")
	}
	if got := fs.Root(); got != dir {
		t.Fatalf("Root() = %q, want %q", got, dir)
	}
}

func TestFileStore_AtomicWritePreservesExistingOnMarshalError(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore("test", dir)

	_ = fs.WriteFile("keep.json", []byte("original"))

	badSrc := make(chan int)
	if err := fs.WriteJSONAtomic("keep.json", badSrc); err == nil {
		t.Fatal("expected marshal error for unmarshalable type")
	}

	got, err := fs.ReadFile("keep.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "original" {
		t.Fatalf("existing file corrupted: got %q", string(got))
	}
}

func TestFileStore_JSONRoundtrip(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore("test", dir)

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
	if err := fs.WriteJSON("records/r1.json", &original); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	raw, err := fs.ReadFile("records/r1.json")
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
	if err := fs.ReadJSON("records/r1.json", &loaded); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if loaded.ID != original.ID {
		t.Fatalf("ID mismatch: %q vs %q", loaded.ID, original.ID)
	}
	if len(loaded.Tags) != len(original.Tags) {
		t.Fatalf("Tags length mismatch: %d vs %d", len(loaded.Tags), len(original.Tags))
	}
}
