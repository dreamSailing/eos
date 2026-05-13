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

func TestBrowserToolsDOMActions(t *testing.T) {
	mgr := NewManager()
	rt := browser.NewBuiltinRuntime()
	if !rt.Status().Ready {
		t.Skipf("builtin browser unavailable: %s", rt.Status().LastError)
	}
	mgr.SetBrowserRuntime(rt)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html>
<html>
<head><title>Demo</title></head>
<body>
  <input id="name" value="">
  <select id="region">
    <option value="cn">China</option>
    <option value="us">United States</option>
  </select>
  <button id="submit" onclick="const out=document.getElementById('output'); out.textContent=document.getElementById('name').value + '-' + document.getElementById('region').value; out.style.display='block'; console.log('submitted', out.textContent);">Submit</button>
  <div id="output" style="display:none"></div>
</body>
</html>`))
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
	if typeRes := mgr.browserTypeStructured(ctx, map[string]interface{}{"selector": "#name", "text": "alice"}); typeRes.Status != "success" {
		t.Fatalf("type failed: %+v", typeRes)
	}
	if selectRes := mgr.browserSelectStructured(ctx, map[string]interface{}{"selector": "#region", "values": []string{"us"}}); selectRes.Status != "success" {
		t.Fatalf("select failed: %+v", selectRes)
	}
	if clickRes := mgr.browserClickStructured(ctx, map[string]interface{}{"selector": "#submit"}); clickRes.Status != "success" {
		t.Fatalf("click failed: %+v", clickRes)
	}
	if waitRes := mgr.browserWaitStructured(ctx, map[string]interface{}{"selector": "#output", "timeout": 3000}); waitRes.Status != "success" {
		t.Fatalf("wait failed: %+v", waitRes)
	}
	snap = mgr.browserSnapshotStructured(ctx, map[string]interface{}{})
	if snap.Status != "success" {
		t.Fatalf("snapshot after click failed: %+v", snap)
	}
	if !strings.Contains(snap.Display, "alice-us") {
		t.Fatalf("snapshot missing updated DOM content: %q", snap.Display)
	}
	console := mgr.browserConsoleStructured(ctx, map[string]interface{}{"limit": 10})
	if console.Status != "success" || !strings.Contains(console.Display, "submitted") {
		t.Fatalf("console failed: %+v", console)
	}
	network := mgr.browserNetworkStructured(ctx, map[string]interface{}{"limit": 10})
	if network.Status != "success" || !strings.Contains(network.Display, srv.URL) {
		t.Fatalf("network failed: %+v", network)
	}

	outPath := filepath.ToSlash(filepath.Join("artifacts", "page.png"))
	shot := mgr.browserScreenshotStructured(ctx, map[string]interface{}{"path": outPath})
	if shot.Status != "success" {
		t.Fatalf("screenshot failed: %+v", shot)
	}
	abs := filepath.Join(root, "artifacts", "page.png")
	info, err := os.Stat(abs)
	if err != nil {
		t.Fatalf("expected screenshot output at %s: %v", abs, err)
	}
	if info.Size() == 0 {
		t.Fatal("expected non-empty screenshot output")
	}
}
