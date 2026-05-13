package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltinRuntimeStatus(t *testing.T) {
	rt := NewBuiltinRuntime()
	status := rt.Status()
	if got := strings.Join(status.Capabilities, ","); got != "navigate,snapshot,tabs,back,forward,click,hover,type,press_key,select,wait,scroll,screenshot,console,network" {
		t.Fatalf("capabilities = %q", got)
	}
	if !status.Ready && status.LastError == "" {
		t.Fatalf("expected unavailable runtime to expose a reason: %+v", status)
	}
}

func TestBuiltinRuntimeTraceLifecycle(t *testing.T) {
	rt := NewBuiltinRuntime()
	if !rt.Status().Ready {
		t.Skipf("builtin browser unavailable: %s", rt.Status().LastError)
	}
	rt.StartTrace("trace-1")
	if rt.SessionCount() != 1 {
		t.Fatalf("session count = %d, want 1", rt.SessionCount())
	}
	rt.ReleaseTrace("trace-1")
	if rt.SessionCount() != 0 {
		t.Fatalf("session count = %d, want 0", rt.SessionCount())
	}
}

func TestBuiltinSessionDOMActions(t *testing.T) {
	rt := NewBuiltinRuntime()
	if !rt.Status().Ready {
		t.Skipf("builtin browser unavailable: %s", rt.Status().LastError)
	}
	sess, err := rt.Session("trace-1")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.ReleaseTrace("trace-1")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/second":
			_, _ = w.Write([]byte(`<!doctype html>
<html>
<head><title>Second</title></head>
<body>
  <h1 id="page">Second Page</h1>
</body>
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
  <div style="height: 1600px"></div>
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

	ctx := context.Background()
	if _, err := sess.Navigate(ctx, NavigateRequest{URL: srv.URL}); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if _, err := sess.Type(ctx, TypeRequest{Selector: "#name", Text: "alice"}); err != nil {
		t.Fatalf("type failed: %v", err)
	}
	if _, err := sess.Hover(ctx, HoverRequest{Selector: "#hover-target"}); err != nil {
		t.Fatalf("hover failed: %v", err)
	}
	if _, err := sess.Type(ctx, TypeRequest{Selector: "#keybox", Text: "z"}); err != nil {
		t.Fatalf("type keybox failed: %v", err)
	}
	if _, err := sess.PressKey(ctx, KeyRequest{Selector: "#keybox", Keys: "\n"}); err != nil {
		t.Fatalf("press key failed: %v", err)
	}
	if _, err := sess.Select(ctx, SelectRequest{Selector: "#region", Values: []string{"us"}}); err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if _, err := sess.Click(ctx, ClickRequest{Selector: "#submit"}); err != nil {
		t.Fatalf("click failed: %v", err)
	}
	if _, err := sess.Wait(ctx, WaitRequest{Selector: "#output", Timeout: 3000}); err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if _, err := sess.Scroll(ctx, ScrollRequest{Y: 500}); err != nil {
		t.Fatalf("scroll failed: %v", err)
	}
	if _, err := sess.Click(ctx, ClickRequest{Selector: "#to-second"}); err != nil {
		t.Fatalf("navigate to second failed: %v", err)
	}
	if _, err := sess.Wait(ctx, WaitRequest{Selector: "#page", Timeout: 3000}); err != nil {
		t.Fatalf("wait second failed: %v", err)
	}
	if _, err := sess.Back(ctx); err != nil {
		t.Fatalf("back failed: %v", err)
	}
	if _, err := sess.Wait(ctx, WaitRequest{Selector: "#output", Timeout: 3000}); err != nil {
		t.Fatalf("wait after back failed: %v", err)
	}
	if _, err := sess.Forward(ctx); err != nil {
		t.Fatalf("forward failed: %v", err)
	}
	if _, err := sess.Wait(ctx, WaitRequest{Selector: "#page", Timeout: 3000}); err != nil {
		t.Fatalf("wait after forward failed: %v", err)
	}
	backRes, err := sess.Back(ctx)
	if err != nil {
		t.Fatalf("final back failed: %v", err)
	}
	if !strings.Contains(backRes.Message, srv.URL) {
		t.Fatalf("back message missing origin URL: %q", backRes.Message)
	}

	snap, err := sess.Snapshot(ctx, SnapshotRequest{})
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if !strings.Contains(snap.Message, "alice-us") {
		t.Fatalf("snapshot missing updated DOM content: %q", snap.Message)
	}
	if !strings.Contains(snap.Message, "hovered") {
		t.Fatalf("snapshot missing hover effect: %q", snap.Message)
	}
	if !strings.Contains(snap.Message, "Enter") {
		t.Fatalf("snapshot missing key effect: %q", snap.Message)
	}
	if !strings.Contains(snap.Message, "scrolled") {
		t.Fatalf("snapshot missing scroll effect: %q", snap.Message)
	}

	console, err := sess.Console(ctx, ConsoleRequest{Limit: 10})
	if err != nil {
		t.Fatalf("console failed: %v", err)
	}
	if !strings.Contains(console.Message, "submitted") {
		t.Fatalf("console missing click log: %q", console.Message)
	}

	network, err := sess.Network(ctx, NetworkRequest{Limit: 10})
	if err != nil {
		t.Fatalf("network failed: %v", err)
	}
	if !strings.Contains(network.Message, srv.URL) {
		t.Fatalf("network missing request log: %q", network.Message)
	}

	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "page.png")
	if _, err := sess.Screenshot(ctx, ScreenshotRequest{Path: outPath}); err != nil {
		t.Fatalf("screenshot failed: %v", err)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("missing screenshot output: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected non-empty screenshot output")
	}
}

func TestBuiltinSessionTabs(t *testing.T) {
	rt := NewBuiltinRuntime()
	if !rt.Status().Ready {
		t.Skipf("builtin browser unavailable: %s", rt.Status().LastError)
	}
	sess, err := rt.Session("trace-tabs")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.ReleaseTrace("trace-tabs")

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

	ctx := context.Background()
	if _, err := sess.Navigate(ctx, NavigateRequest{URL: srv.URL}); err != nil {
		t.Fatalf("navigate first failed: %v", err)
	}
	listRes, err := sess.Tabs(ctx, TabsRequest{Action: "list"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	tabsAny, ok := listRes.Data["tabs"]
	if !ok {
		t.Fatalf("list result missing tabs: %+v", listRes.Data)
	}
	tabs, ok := tabsAny.([]TabInfo)
	if !ok || len(tabs) != 1 {
		t.Fatalf("expected 1 tab, got %#v", tabsAny)
	}

	newRes, err := sess.Tabs(ctx, TabsRequest{Action: "new", URL: srv.URL + "/two"})
	if err != nil {
		t.Fatalf("new tab failed: %v", err)
	}
	tabs, ok = newRes.Data["tabs"].([]TabInfo)
	if !ok || len(tabs) != 2 {
		t.Fatalf("expected 2 tabs after open, got %#v", newRes.Data["tabs"])
	}
	if active, _ := newRes.Data["active_tab"].(string); active != "tab-2" {
		t.Fatalf("active_tab = %q, want tab-2", active)
	}
	bgRes, err := sess.Tabs(ctx, TabsRequest{Action: "new", URL: srv.URL + "/two?bg=1", Activate: false, HasActivate: true})
	if err != nil {
		t.Fatalf("background tab failed: %v", err)
	}
	tabs, ok = bgRes.Data["tabs"].([]TabInfo)
	if !ok || len(tabs) != 3 {
		t.Fatalf("expected 3 tabs after background open, got %#v", bgRes.Data["tabs"])
	}
	if active, _ := bgRes.Data["active_tab"].(string); active != "tab-2" {
		t.Fatalf("active_tab after background open = %q, want tab-2", active)
	}
	matchRes, err := sess.Tabs(ctx, TabsRequest{Action: "switch", Query: "two?bg=1"})
	if err != nil {
		t.Fatalf("switch by query failed: %v", err)
	}
	if active, _ := matchRes.Data["active_tab"].(string); active != "tab-3" {
		t.Fatalf("active_tab after query switch = %q, want tab-3", active)
	}

	switchRes, err := sess.Tabs(ctx, TabsRequest{Action: "switch", Index: 0, HasIndex: true})
	if err != nil {
		t.Fatalf("switch failed: %v", err)
	}
	if active, _ := switchRes.Data["active_tab"].(string); active != "tab-1" {
		t.Fatalf("active_tab after switch = %q, want tab-1", active)
	}
	snap, err := sess.Snapshot(ctx, SnapshotRequest{})
	if err != nil {
		t.Fatalf("snapshot after switch failed: %v", err)
	}
	if !strings.Contains(snap.Message, "One") {
		t.Fatalf("snapshot missing first tab content: %q", snap.Message)
	}
	closeByMatchRes, err := sess.Tabs(ctx, TabsRequest{Action: "close", Query: "bg=1"})
	if err != nil {
		t.Fatalf("close by query failed: %v", err)
	}
	if closed, _ := closeByMatchRes.Data["closed_tab"].(string); closed != "tab-3" {
		t.Fatalf("closed_tab by query = %q, want tab-3", closed)
	}
	tabs, ok = closeByMatchRes.Data["tabs"].([]TabInfo)
	if !ok || len(tabs) != 2 {
		t.Fatalf("expected 2 tabs after query close, got %#v", closeByMatchRes.Data["tabs"])
	}
	switchRes, err = sess.Tabs(ctx, TabsRequest{Action: "switch", ID: "tab-2"})
	if err != nil {
		t.Fatalf("switch back to tab-2 failed: %v", err)
	}
	if active, _ := switchRes.Data["active_tab"].(string); active != "tab-2" {
		t.Fatalf("active_tab after switch back = %q, want tab-2", active)
	}

	closeRes, err := sess.Tabs(ctx, TabsRequest{Action: "close", ID: "tab-2"})
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}
	tabs, ok = closeRes.Data["tabs"].([]TabInfo)
	if !ok || len(tabs) != 2 {
		t.Fatalf("expected 2 tabs after close, got %#v", closeRes.Data["tabs"])
	}
	if active, _ := closeRes.Data["active_tab"].(string); active != "tab-1" {
		t.Fatalf("active_tab after close = %q, want tab-1", active)
	}
	if closed, _ := closeRes.Data["closed_tab"].(string); closed != "tab-2" {
		t.Fatalf("closed_tab = %q, want tab-2", closed)
	}
	if tabs[0].ID != "tab-1" || !tabs[0].Active {
		t.Fatalf("unexpected primary remaining tab state: %#v", tabs[0])
	}
}
