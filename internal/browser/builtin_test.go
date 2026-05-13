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
	if got := strings.Join(status.Capabilities, ","); got != "navigate,snapshot,inspect,tabs,back,forward,click,hover,type,press_key,select,wait,scroll,screenshot,console,network" {
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
	snap, err := sess.Snapshot(ctx, SnapshotRequest{})
	if err != nil {
		t.Fatalf("initial snapshot failed: %v", err)
	}
	nameRef := mustFindSnapshotRef(t, snap.Data["elements"], "#name")
	hoverRef := mustFindSnapshotRef(t, snap.Data["elements"], "#hover-target")
	keyboxRef := mustFindSnapshotRef(t, snap.Data["elements"], "#keybox")
	regionRef := mustFindSnapshotRef(t, snap.Data["elements"], "#region")
	submitRef := mustFindSnapshotRef(t, snap.Data["elements"], "#submit")
	secondRef := mustFindSnapshotRef(t, snap.Data["elements"], "#to-second")
	if _, err := sess.Type(ctx, TypeRequest{Ref: nameRef, Text: "alice"}); err != nil {
		t.Fatalf("type failed: %v", err)
	}
	if _, err := sess.Hover(ctx, HoverRequest{Ref: hoverRef}); err != nil {
		t.Fatalf("hover failed: %v", err)
	}
	if _, err := sess.Type(ctx, TypeRequest{Ref: keyboxRef, Text: "z"}); err != nil {
		t.Fatalf("type keybox failed: %v", err)
	}
	if _, err := sess.PressKey(ctx, KeyRequest{Ref: keyboxRef, Keys: "\n"}); err != nil {
		t.Fatalf("press key failed: %v", err)
	}
	if _, err := sess.Select(ctx, SelectRequest{Ref: regionRef, Values: []string{"us"}}); err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if _, err := sess.Click(ctx, ClickRequest{Ref: submitRef}); err != nil {
		t.Fatalf("click failed: %v", err)
	}
	if _, err := sess.Wait(ctx, WaitRequest{Selector: "#output", Timeout: 3000}); err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if _, err := sess.Scroll(ctx, ScrollRequest{Y: 500}); err != nil {
		t.Fatalf("scroll failed: %v", err)
	}
	if _, err := sess.Click(ctx, ClickRequest{Ref: secondRef}); err != nil {
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

	snap, err = sess.Snapshot(ctx, SnapshotRequest{})
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if !strings.Contains(snap.Message, "[") {
		t.Fatalf("snapshot missing ref listing: %q", snap.Message)
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
	if elements, ok := snap.Data["elements"].([]SnapshotElement); !ok || len(elements) == 0 {
		t.Fatalf("snapshot missing structured elements: %#v", snap.Data["elements"])
	}
	if outline, ok := snap.Data["outline"].([]string); !ok || len(outline) == 0 {
		t.Fatalf("snapshot missing outline: %#v", snap.Data["outline"])
	}
	inspect, err := sess.Inspect(ctx, InspectRequest{Ref: submitRef})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	if !strings.Contains(inspect.Message, "ref="+submitRef) {
		t.Fatalf("inspect missing ref: %q", inspect.Message)
	}
	if detail, ok := inspect.Data["detail"].(InspectDetails); !ok || strings.TrimSpace(detail.Element.Ref) != submitRef {
		t.Fatalf("inspect missing structured detail: %#v", inspect.Data["detail"])
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

func mustFindSnapshotRef(t *testing.T, raw interface{}, selector string) string {
	t.Helper()
	elements, ok := raw.([]SnapshotElement)
	if !ok {
		t.Fatalf("elements type = %T, want []SnapshotElement", raw)
	}
	for _, el := range elements {
		if el.Selector == selector {
			return el.Ref
		}
	}
	t.Fatalf("missing snapshot ref for selector %s in %#v", selector, elements)
	return ""
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
	currentRes, err := sess.Tabs(ctx, TabsRequest{Action: "current"})
	if err != nil {
		t.Fatalf("current failed: %v", err)
	}
	if target, _ := currentRes.Data["target_tab"].(string); target != "tab-1" {
		t.Fatalf("target_tab for current = %q, want tab-1", target)
	}
	if currentInfo, ok := currentRes.Data["tab"].(TabInfo); !ok || currentInfo.ID != "tab-1" || !currentInfo.Active {
		t.Fatalf("unexpected current tab info: %#v", currentRes.Data["tab"])
	}
	lastRes, err := sess.Tabs(ctx, TabsRequest{Action: "activate_last"})
	if err != nil {
		t.Fatalf("activate_last failed: %v", err)
	}
	if target, _ := lastRes.Data["target_tab"].(string); target != "tab-3" {
		t.Fatalf("target_tab for activate_last = %q, want tab-3", target)
	}
	snap, err := sess.Snapshot(ctx, SnapshotRequest{})
	if err != nil {
		t.Fatalf("snapshot after switch failed: %v", err)
	}
	if !strings.Contains(snap.Message, "Two") {
		t.Fatalf("snapshot missing active tab content: %q", snap.Message)
	}
	if snapInfo, ok := snap.Data["tab"].(TabInfo); !ok || snapInfo.ID != "tab-3" || !snapInfo.Active {
		t.Fatalf("snapshot missing tab info: %#v", snap.Data["tab"])
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

	reopenLeftRes, err := sess.Tabs(ctx, TabsRequest{Action: "new", URL: srv.URL + "/two?a=1"})
	if err != nil {
		t.Fatalf("reopen second tab failed: %v", err)
	}
	leftID, _ := reopenLeftRes.Data["opened_tab"].(string)
	reopenRightRes, err := sess.Tabs(ctx, TabsRequest{Action: "new", URL: srv.URL + "/two?a=2", Activate: false, HasActivate: true})
	if err != nil {
		t.Fatalf("reopen third tab failed: %v", err)
	}
	rightID, _ := reopenRightRes.Data["opened_tab"].(string)
	rightRes, err := sess.Tabs(ctx, TabsRequest{Action: "close_right", Index: 0, HasIndex: true})
	if err != nil {
		t.Fatalf("close_right failed: %v", err)
	}
	if active, _ := rightRes.Data["active_tab"].(string); active != "tab-1" {
		t.Fatalf("active_tab after close_right = %q, want tab-1", active)
	}
	if closedTabs, ok := rightRes.Data["closed_tabs"].([]string); !ok || len(closedTabs) != 2 || closedTabs[0] != leftID || closedTabs[1] != rightID {
		t.Fatalf("closed_tabs after close_right = %#v", rightRes.Data["closed_tabs"])
	}
	tabs, ok = rightRes.Data["tabs"].([]TabInfo)
	if !ok || len(tabs) != 1 || tabs[0].ID != "tab-1" || !tabs[0].Active {
		t.Fatalf("unexpected tabs after close_right: %#v", rightRes.Data["tabs"])
	}

	keepRes, err := sess.Tabs(ctx, TabsRequest{Action: "new", URL: srv.URL + "/two?keep=1"})
	if err != nil {
		t.Fatalf("open keep tab failed: %v", err)
	}
	keepID, _ := keepRes.Data["opened_tab"].(string)
	dropRes, err := sess.Tabs(ctx, TabsRequest{Action: "new", URL: srv.URL + "/two?drop=1", Activate: false, HasActivate: true})
	if err != nil {
		t.Fatalf("open drop tab failed: %v", err)
	}
	dropID, _ := dropRes.Data["opened_tab"].(string)
	othersRes, err := sess.Tabs(ctx, TabsRequest{Action: "close_others", Query: "keep=1"})
	if err != nil {
		t.Fatalf("close_others failed: %v", err)
	}
	if active, _ := othersRes.Data["active_tab"].(string); active != keepID {
		t.Fatalf("active_tab after close_others = %q, want %s", active, keepID)
	}
	if target, _ := othersRes.Data["target_tab"].(string); target != keepID {
		t.Fatalf("target_tab after close_others = %q, want %s", target, keepID)
	}
	if closedTabs, ok := othersRes.Data["closed_tabs"].([]string); !ok || len(closedTabs) != 2 || closedTabs[0] != "tab-1" || closedTabs[1] != dropID {
		t.Fatalf("closed_tabs after close_others = %#v", othersRes.Data["closed_tabs"])
	}
	tabs, ok = othersRes.Data["tabs"].([]TabInfo)
	if !ok || len(tabs) != 1 || tabs[0].ID != keepID || !tabs[0].Active {
		t.Fatalf("unexpected tabs after close_others: %#v", othersRes.Data["tabs"])
	}
}
