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
		switch r.URL.Path {
		case "/second":
			_, _ = w.Write([]byte(`<!doctype html>
<html>
<head><title>Second</title></head>
<body><h1 id="page">Second Page</h1></body>
</html>`))
		default:
			_, _ = w.Write([]byte(`<!doctype html>
<html>
<head><title>Demo</title></head>
<body>
  <input id="name" value="">
  <input id="keybox" onkeydown="document.getElementById('key-status').textContent=event.key; console.log('key', event.key);">
  <select id="region">
    <option value="cn">China</option>
    <option value="us">United States</option>
  </select>
  <div id="hover-target" onmouseover="document.getElementById('hover-status').textContent='hovered';">Hover Target</div>
  <div id="hover-status"></div>
  <div id="key-status"></div>
  <div id="scroll-status"></div>
  <button id="submit" onclick="const out=document.getElementById('output'); out.textContent=document.getElementById('name').value + '-' + document.getElementById('region').value; out.style.display='block'; console.log('submitted', out.textContent);">Submit</button>
  <div id="output" style="display:none"></div>
  <div style="height:1600px"></div>
  <div id="footer">Footer</div>
  <a id="to-second" href="/second">Second Page</a>
  <script>
    window.addEventListener('scroll', () => {
      if (window.scrollY > 100) {
        document.getElementById('scroll-status').textContent = 'scrolled';
      }
    });
  </script>
</body>
</html>`))
		}
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
	if hoverRes := mgr.browserHoverStructured(ctx, map[string]interface{}{"selector": "#hover-target"}); hoverRes.Status != "success" {
		t.Fatalf("hover failed: %+v", hoverRes)
	}
	if typeRes := mgr.browserTypeStructured(ctx, map[string]interface{}{"selector": "#keybox", "text": "z"}); typeRes.Status != "success" {
		t.Fatalf("keybox type failed: %+v", typeRes)
	}
	if keyRes := mgr.browserPressKeyStructured(ctx, map[string]interface{}{"selector": "#keybox", "keys": "\n"}); keyRes.Status != "success" {
		t.Fatalf("press key failed: %+v", keyRes)
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
	if scrollRes := mgr.browserScrollStructured(ctx, map[string]interface{}{"y": 500}); scrollRes.Status != "success" {
		t.Fatalf("scroll failed: %+v", scrollRes)
	}
	if nav2 := mgr.browserClickStructured(ctx, map[string]interface{}{"selector": "#to-second"}); nav2.Status != "success" {
		t.Fatalf("second page click failed: %+v", nav2)
	}
	if waitRes := mgr.browserWaitStructured(ctx, map[string]interface{}{"selector": "#page", "timeout": 3000}); waitRes.Status != "success" {
		t.Fatalf("second page wait failed: %+v", waitRes)
	}
	if backRes := mgr.browserBackStructured(ctx, map[string]interface{}{}); backRes.Status != "success" {
		t.Fatalf("back failed: %+v", backRes)
	}
	if waitRes := mgr.browserWaitStructured(ctx, map[string]interface{}{"selector": "#output", "timeout": 3000}); waitRes.Status != "success" {
		t.Fatalf("back wait failed: %+v", waitRes)
	}
	if forwardRes := mgr.browserForwardStructured(ctx, map[string]interface{}{}); forwardRes.Status != "success" {
		t.Fatalf("forward failed: %+v", forwardRes)
	}
	if waitRes := mgr.browserWaitStructured(ctx, map[string]interface{}{"selector": "#page", "timeout": 3000}); waitRes.Status != "success" {
		t.Fatalf("forward wait failed: %+v", waitRes)
	}
	if backRes := mgr.browserBackStructured(ctx, map[string]interface{}{}); backRes.Status != "success" {
		t.Fatalf("final back failed: %+v", backRes)
	}
	snap = mgr.browserSnapshotStructured(ctx, map[string]interface{}{})
	if snap.Status != "success" {
		t.Fatalf("snapshot after click failed: %+v", snap)
	}
	if !strings.Contains(snap.Display, "alice-us") {
		t.Fatalf("snapshot missing updated DOM content: %q", snap.Display)
	}
	if !strings.Contains(snap.Display, "hovered") || !strings.Contains(snap.Display, "Enter") || !strings.Contains(snap.Display, "scrolled") {
		t.Fatalf("snapshot missing new interaction state: %q", snap.Display)
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

func TestBrowserToolsTabs(t *testing.T) {
	mgr := NewManager()
	rt := browser.NewBuiltinRuntime()
	if !rt.Status().Ready {
		t.Skipf("builtin browser unavailable: %s", rt.Status().LastError)
	}
	mgr.SetBrowserRuntime(rt)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/two":
			_, _ = w.Write([]byte(`<!doctype html><html><head><title>Two</title></head><body><h1>Two</h1></body></html>`))
		default:
			_, _ = w.Write([]byte(`<!doctype html><html><head><title>One</title></head><body><h1>One</h1></body></html>`))
		}
	}))
	defer srv.Close()

	root := t.TempDir()
	ctx := WithWorkspaceRoot(WithTraceID(context.Background(), "rid-browser-tabs"), root)

	if nav := mgr.browserNavigateStructured(ctx, map[string]interface{}{"url": srv.URL}); nav.Status != "success" {
		t.Fatalf("navigate failed: %+v", nav)
	}
	listRes := mgr.browserTabsStructured(ctx, map[string]interface{}{"action": "list"})
	if listRes.Status != "success" {
		t.Fatalf("tabs list failed: %+v", listRes)
	}
	tabsAny, ok := listRes.Data["tabs"]
	if !ok {
		t.Fatalf("tabs list missing tabs: %+v", listRes.Data)
	}
	tabs, ok := tabsAny.([]browser.TabInfo)
	if !ok || len(tabs) != 1 {
		t.Fatalf("expected 1 tab, got %#v", tabsAny)
	}

	newRes := mgr.browserTabsStructured(ctx, map[string]interface{}{"action": "new", "url": srv.URL + "/two"})
	if newRes.Status != "success" {
		t.Fatalf("tabs new failed: %+v", newRes)
	}
	if active, _ := newRes.Data["active_tab"].(string); active != "tab-2" {
		t.Fatalf("active_tab = %q, want tab-2", active)
	}
	bgRes := mgr.browserTabsStructured(ctx, map[string]interface{}{"action": "new", "url": srv.URL + "/two?bg=1", "activate": false})
	if bgRes.Status != "success" {
		t.Fatalf("tabs background new failed: %+v", bgRes)
	}
	if active, _ := bgRes.Data["active_tab"].(string); active != "tab-2" {
		t.Fatalf("active_tab after background open = %q, want tab-2", active)
	}

	switchRes := mgr.browserTabsStructured(ctx, map[string]interface{}{"action": "switch", "index": 0})
	if switchRes.Status != "success" {
		t.Fatalf("tabs switch failed: %+v", switchRes)
	}
	snap := mgr.browserSnapshotStructured(ctx, map[string]interface{}{})
	if snap.Status != "success" || !strings.Contains(snap.Display, "One") {
		t.Fatalf("snapshot after switch failed: %+v", snap)
	}

	closeRes := mgr.browserTabsStructured(ctx, map[string]interface{}{"action": "close", "id": "tab-2"})
	if closeRes.Status != "success" {
		t.Fatalf("tabs close failed: %+v", closeRes)
	}
	tabs, ok = closeRes.Data["tabs"].([]browser.TabInfo)
	if !ok || len(tabs) != 2 || tabs[0].ID != "tab-1" || !tabs[0].Active {
		t.Fatalf("unexpected tabs after close: %#v", closeRes.Data["tabs"])
	}
	if active, _ := closeRes.Data["active_tab"].(string); active != "tab-1" {
		t.Fatalf("active_tab after close = %q, want tab-1", active)
	}
	if closed, _ := closeRes.Data["closed_tab"].(string); closed != "tab-2" {
		t.Fatalf("closed_tab = %q, want tab-2", closed)
	}
}
