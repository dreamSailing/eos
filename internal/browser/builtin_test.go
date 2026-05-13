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
	if got := strings.Join(status.Capabilities, ","); got != "navigate,snapshot,click,type,select,wait,screenshot,console,network" {
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

	ctx := context.Background()
	if _, err := sess.Navigate(ctx, NavigateRequest{URL: srv.URL}); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if _, err := sess.Type(ctx, TypeRequest{Selector: "#name", Text: "alice"}); err != nil {
		t.Fatalf("type failed: %v", err)
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

	snap, err := sess.Snapshot(ctx, SnapshotRequest{})
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if !strings.Contains(snap.Message, "alice-us") {
		t.Fatalf("snapshot missing updated DOM content: %q", snap.Message)
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
