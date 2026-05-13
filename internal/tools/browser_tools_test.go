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
	matchRes := mgr.browserTabsStructured(ctx, map[string]interface{}{"action": "switch", "match": "two?bg=1"})
	if matchRes.Status != "success" {
		t.Fatalf("tabs match switch failed: %+v", matchRes)
	}
	if active, _ := matchRes.Data["active_tab"].(string); active != "tab-3" {
		t.Fatalf("active_tab after match switch = %q, want tab-3", active)
	}

	switchRes := mgr.browserTabsStructured(ctx, map[string]interface{}{"action": "switch", "index": 0})
	if switchRes.Status != "success" {
		t.Fatalf("tabs switch failed: %+v", switchRes)
	}
	currentRes := mgr.browserTabsStructured(ctx, map[string]interface{}{"action": "current"})
	if currentRes.Status != "success" {
		t.Fatalf("tabs current failed: %+v", currentRes)
	}
	if target, _ := currentRes.Data["target_tab"].(string); target != "tab-1" {
		t.Fatalf("target_tab for current = %q, want tab-1", target)
	}
	if currentInfo, ok := currentRes.Data["tab"].(browser.TabInfo); !ok || currentInfo.ID != "tab-1" || !currentInfo.Active {
		t.Fatalf("unexpected current tab info: %#v", currentRes.Data["tab"])
	}
	lastRes := mgr.browserTabsStructured(ctx, map[string]interface{}{"action": "activate_last"})
	if lastRes.Status != "success" {
		t.Fatalf("tabs activate_last failed: %+v", lastRes)
	}
	if target, _ := lastRes.Data["target_tab"].(string); target != "tab-3" {
		t.Fatalf("target_tab for activate_last = %q, want tab-3", target)
	}
	snap := mgr.browserSnapshotStructured(ctx, map[string]interface{}{})
	if snap.Status != "success" || !strings.Contains(snap.Display, "Two") {
		t.Fatalf("snapshot after switch failed: %+v", snap)
	}
	if snapInfo, ok := snap.Data["tab"].(browser.TabInfo); !ok || snapInfo.ID != "tab-3" || !snapInfo.Active {
		t.Fatalf("snapshot missing tab info: %#v", snap.Data["tab"])
	}
	closeMatchRes := mgr.browserTabsStructured(ctx, map[string]interface{}{"action": "close", "match": "bg=1"})
	if closeMatchRes.Status != "success" {
		t.Fatalf("tabs close by match failed: %+v", closeMatchRes)
	}
	if closed, _ := closeMatchRes.Data["closed_tab"].(string); closed != "tab-3" {
		t.Fatalf("closed_tab by match = %q, want tab-3", closed)
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

	reopenLeftRes := mgr.browserTabsStructured(ctx, map[string]interface{}{"action": "new", "url": srv.URL + "/two?a=1"})
	if reopenLeftRes.Status != "success" {
		t.Fatalf("tabs reopen left failed: %+v", reopenLeftRes)
	}
	leftID, _ := reopenLeftRes.Data["opened_tab"].(string)
	reopenRightRes := mgr.browserTabsStructured(ctx, map[string]interface{}{"action": "new", "url": srv.URL + "/two?a=2", "activate": false})
	if reopenRightRes.Status != "success" {
		t.Fatalf("tabs reopen right failed: %+v", reopenRightRes)
	}
	rightID, _ := reopenRightRes.Data["opened_tab"].(string)
	closeRightRes := mgr.browserTabsStructured(ctx, map[string]interface{}{"action": "close_right", "index": 0})
	if closeRightRes.Status != "success" {
		t.Fatalf("tabs close_right failed: %+v", closeRightRes)
	}
	if active, _ := closeRightRes.Data["active_tab"].(string); active != "tab-1" {
		t.Fatalf("active_tab after close_right = %q, want tab-1", active)
	}
	if closedTabs, ok := closeRightRes.Data["closed_tabs"].([]string); !ok || len(closedTabs) != 2 || closedTabs[0] != leftID || closedTabs[1] != rightID {
		t.Fatalf("closed_tabs after close_right = %#v", closeRightRes.Data["closed_tabs"])
	}

	keepRes := mgr.browserTabsStructured(ctx, map[string]interface{}{"action": "new", "url": srv.URL + "/two?keep=1"})
	if keepRes.Status != "success" {
		t.Fatalf("tabs keep open failed: %+v", keepRes)
	}
	keepID, _ := keepRes.Data["opened_tab"].(string)
	dropRes := mgr.browserTabsStructured(ctx, map[string]interface{}{"action": "new", "url": srv.URL + "/two?drop=1", "activate": false})
	if dropRes.Status != "success" {
		t.Fatalf("tabs drop open failed: %+v", dropRes)
	}
	dropID, _ := dropRes.Data["opened_tab"].(string)
	closeOthersRes := mgr.browserTabsStructured(ctx, map[string]interface{}{"action": "close_others", "match": "keep=1"})
	if closeOthersRes.Status != "success" {
		t.Fatalf("tabs close_others failed: %+v", closeOthersRes)
	}
	if active, _ := closeOthersRes.Data["active_tab"].(string); active != keepID {
		t.Fatalf("active_tab after close_others = %q, want %s", active, keepID)
	}
	if target, _ := closeOthersRes.Data["target_tab"].(string); target != keepID {
		t.Fatalf("target_tab after close_others = %q, want %s", target, keepID)
	}
	if closedTabs, ok := closeOthersRes.Data["closed_tabs"].([]string); !ok || len(closedTabs) != 2 || closedTabs[0] != "tab-1" || closedTabs[1] != dropID {
		t.Fatalf("closed_tabs after close_others = %#v", closeOthersRes.Data["closed_tabs"])
	}
	tabs, ok = closeOthersRes.Data["tabs"].([]browser.TabInfo)
	if !ok || len(tabs) != 1 || tabs[0].ID != keepID || !tabs[0].Active {
		t.Fatalf("unexpected tabs after close_others: %#v", closeOthersRes.Data["tabs"])
	}
}

func TestBrowserStatusIncludesBuiltinSession(t *testing.T) {
	mgr := NewManager()
	rt := browser.NewBuiltinRuntime()
	if !rt.Status().Ready {
		t.Skipf("builtin browser unavailable: %s", rt.Status().LastError)
	}
	mgr.SetBrowserRuntime(rt)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Status Demo</title></head><body><h1>Status Demo</h1></body></html>`))
	}))
	defer srv.Close()

	ctx := WithWorkspaceRoot(WithTraceID(context.Background(), "rid-browser-status"), t.TempDir())
	if nav := mgr.browserNavigateStructured(ctx, map[string]interface{}{"url": srv.URL}); nav.Status != "success" {
		t.Fatalf("navigate failed: %+v", nav)
	}
	if newRes := mgr.browserTabsStructured(ctx, map[string]interface{}{"action": "new", "url": srv.URL + "/two", "activate": false}); newRes.Status != "success" {
		t.Fatalf("tabs new failed: %+v", newRes)
	}

	status := mgr.browserStatusStructured(ctx, map[string]interface{}{})
	if status.Status != "success" {
		t.Fatalf("browser status failed: %+v", status)
	}
	if !strings.Contains(status.Display, "builtin_ready=true") {
		t.Fatalf("status missing builtin readiness: %q", status.Display)
	}
	if !strings.Contains(status.Display, "trace_id=rid-browser-status") {
		t.Fatalf("status missing trace id: %q", status.Display)
	}
	if !strings.Contains(status.Display, "active_tab=tab-1") {
		t.Fatalf("status missing active tab: %q", status.Display)
	}
	current, ok := status.Data["current_session"].(map[string]interface{})
	if !ok {
		t.Fatalf("current_session missing: %#v", status.Data["current_session"])
	}
	if got, _ := current["tab_count"].(int); got != 2 {
		t.Fatalf("tab_count = %v, want 2", current["tab_count"])
	}
}
