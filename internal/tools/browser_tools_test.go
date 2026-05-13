package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dreamSailing/eos/internal/browser"
)

func TestBrowserToolsNavigateSnapshotAndScreenshot(t *testing.T) {
	mgr := NewManager()
	rt := browser.NewBuiltinRuntime()
	mgr.SetBrowserRuntime(rt)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><head><title>Demo</title></head><body><h1>Hello Browser</h1></body></html>"))
	}))
	defer srv.Close()

	root := t.TempDir()
	ctx := WithWorkspaceRoot(WithTraceID(context.Background(), "rid-browser"), root)

	nav := mgr.browserNavigateStructured(ctx, map[string]interface{}{"url": srv.URL})
	if nav.Status != "success" {
		t.Fatalf("navigate failed: %+v", nav)
	}

	snap := mgr.browserSnapshotStructured(ctx, map[string]interface{}{})
	if snap.Status != "success" {
		t.Fatalf("snapshot failed: %+v", snap)
	}
	if !strings.Contains(snap.Display, "Demo") {
		t.Fatalf("snapshot missing title: %q", snap.Display)
	}

	outPath := filepath.ToSlash(filepath.Join("artifacts", "page.html"))
	shot := mgr.browserScreenshotStructured(ctx, map[string]interface{}{"path": outPath})
	if shot.Status != "success" {
		t.Fatalf("screenshot failed: %+v", shot)
	}
	abs := filepath.Join(root, "artifacts", "page.html")
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("expected screenshot output at %s: %v", abs, err)
	}
}

func TestBrowserToolsClickReturnsUnsupportedForBuiltin(t *testing.T) {
	mgr := NewManager()
	mgr.SetBrowserRuntime(browser.NewBuiltinRuntime())
	ctx := WithTraceID(context.Background(), "rid-browser")
	res := mgr.browserClickStructured(ctx, map[string]interface{}{"selector": "button"})
	if res.Status != "error" {
		t.Fatalf("expected error result, got %+v", res)
	}
	if !strings.Contains(res.Error, "DOM-capable backend") {
		t.Fatalf("unexpected error: %q", res.Error)
	}
}
