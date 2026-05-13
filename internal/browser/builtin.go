package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	cdruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

var (
	ErrRuntimeClosed    = errors.New("builtin browser runtime is closed")
	ErrNoLoadedPage     = errors.New("builtin browser session has no loaded page")
	titlePattern        = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	tagPattern          = regexp.MustCompile(`(?s)<[^>]+>`)
	DefaultCapabilities = []string{"navigate", "snapshot", "tabs", "back", "forward", "click", "hover", "type", "press_key", "select", "wait", "scroll", "screenshot", "console", "network"}
)

type NavigateRequest struct {
	URL string
}

type SnapshotRequest struct{}

type TabsRequest struct {
	Action   string
	ID       string
	Index    int
	HasIndex bool
	URL      string
}

type TabInfo struct {
	ID     string `json:"id"`
	Index  int    `json:"index"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	Active bool   `json:"active"`
}

type HoverRequest struct {
	Selector string
}

type KeyRequest struct {
	Selector string
	Keys     string
}

type ScrollRequest struct {
	Selector string
	X        int
	Y        int
}

type ClickRequest struct {
	Selector string
}

type TypeRequest struct {
	Selector string
	Text     string
}

type SelectRequest struct {
	Selector string
	Values   []string
}

type WaitRequest struct {
	Selector string
	Timeout  int
}

type ScreenshotRequest struct {
	Path string
}

type ConsoleRequest struct {
	Limit int
}

type NetworkRequest struct {
	Limit int
}

type ActionResult struct {
	Message string
	Data    map[string]interface{}
}

type SessionBackend interface {
	Capabilities() []string
	Navigate(context.Context, NavigateRequest) (ActionResult, error)
	Snapshot(context.Context, SnapshotRequest) (ActionResult, error)
	Tabs(context.Context, TabsRequest) (ActionResult, error)
	Back(context.Context) (ActionResult, error)
	Forward(context.Context) (ActionResult, error)
	Click(context.Context, ClickRequest) (ActionResult, error)
	Hover(context.Context, HoverRequest) (ActionResult, error)
	Type(context.Context, TypeRequest) (ActionResult, error)
	PressKey(context.Context, KeyRequest) (ActionResult, error)
	Select(context.Context, SelectRequest) (ActionResult, error)
	Wait(context.Context, WaitRequest) (ActionResult, error)
	Scroll(context.Context, ScrollRequest) (ActionResult, error)
	Screenshot(context.Context, ScreenshotRequest) (ActionResult, error)
	Console(context.Context, ConsoleRequest) (ActionResult, error)
	Network(context.Context, NetworkRequest) (ActionResult, error)
}

type RuntimeStatus struct {
	Ready        bool
	LastError    string
	Capabilities []string
}

type BuiltinRuntime struct {
	mu        sync.RWMutex
	sessions  map[string]*session
	execPath  string
	lastError string
	closed    bool
}

type session struct {
	runtime *BuiltinRuntime
	traceID string

	allocCancel context.CancelFunc
	cancel      context.CancelFunc
	ctx         context.Context

	mu          sync.RWMutex
	createdAt   time.Time
	updatedAt   time.Time
	nextTabID   int
	activeTabID string
	tabs        []*tab
}

type tab struct {
	id     string
	ctx    context.Context
	cancel context.CancelFunc
	opMu   sync.Mutex

	mu        sync.RWMutex
	createdAt time.Time
	updatedAt time.Time
	lastURL   string
	lastTitle string
	lastHTML  string
	console   []string
	network   []string
}

func NewBuiltinRuntime() *BuiltinRuntime {
	execPath := findBrowserBinary()
	rt := &BuiltinRuntime{
		sessions: make(map[string]*session),
		execPath: execPath,
	}
	if execPath == "" {
		rt.lastError = "no supported Chrome/Chromium executable found"
	}
	return rt
}

func (r *BuiltinRuntime) Status() RuntimeStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return RuntimeStatus{
		Ready:        !r.closed && strings.TrimSpace(r.execPath) != "",
		LastError:    strings.TrimSpace(r.lastError),
		Capabilities: append([]string(nil), DefaultCapabilities...),
	}
}

func (r *BuiltinRuntime) StartTrace(traceID string) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return
	}
	_, _ = r.Session(traceID)
}

func (r *BuiltinRuntime) Session(traceID string) (SessionBackend, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return nil, fmt.Errorf("missing trace id")
	}
	r.mu.Lock()
	if r.closed {
		r.lastError = ErrRuntimeClosed.Error()
		r.mu.Unlock()
		return nil, ErrRuntimeClosed
	}
	if sess, ok := r.sessions[traceID]; ok {
		r.mu.Unlock()
		return sess, nil
	}
	execPath := strings.TrimSpace(r.execPath)
	r.mu.Unlock()
	if execPath == "" {
		err := fmt.Errorf("builtin browser runtime unavailable: %s", noBrowserMessage())
		r.setLastError(err)
		return nil, err
	}

	sess, err := newSession(r, traceID, execPath)
	if err != nil {
		r.setLastError(err)
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		sess.Close()
		r.lastError = ErrRuntimeClosed.Error()
		return nil, ErrRuntimeClosed
	}
	if existing, ok := r.sessions[traceID]; ok {
		sess.Close()
		return existing, nil
	}
	r.sessions[traceID] = sess
	return sess, nil
}

func (r *BuiltinRuntime) ReleaseTrace(traceID string) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return
	}
	r.mu.Lock()
	sess := r.sessions[traceID]
	delete(r.sessions, traceID)
	r.mu.Unlock()
	if sess != nil {
		sess.Close()
	}
}

func (r *BuiltinRuntime) SessionCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}

func (r *BuiltinRuntime) Close() {
	r.mu.Lock()
	r.closed = true
	sessions := r.sessions
	r.sessions = make(map[string]*session)
	r.mu.Unlock()
	for _, sess := range sessions {
		sess.Close()
	}
}

func (s *session) Capabilities() []string {
	return append([]string(nil), DefaultCapabilities...)
}

func (s *session) Navigate(ctx context.Context, req NavigateRequest) (ActionResult, error) {
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	target := strings.TrimSpace(req.URL)
	if target == "" {
		return ActionResult{}, fmt.Errorf("missing url")
	}
	err = tab.run(ctx,
		network.Enable(),
		cdruntime.Enable(),
		page.Enable(),
		chromedp.Navigate(target),
		chromedp.WaitReady("html", chromedp.ByQuery),
	)
	if err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	url, title, _, err := s.refreshTabPageState(ctx, tab)
	if err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	return ActionResult{Message: fmt.Sprintf("navigated %s to %s (title=%q)", tab.id, url, title)}, nil
}

func (s *session) Snapshot(ctx context.Context, req SnapshotRequest) (ActionResult, error) {
	_ = req
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	url, title, html, err := s.refreshTabPageState(ctx, tab)
	if err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	if strings.TrimSpace(url) == "" {
		return ActionResult{}, ErrNoLoadedPage
	}
	return ActionResult{
		Message: fmt.Sprintf("tab=%s\nurl=%s\ntitle=%s\nsnapshot=%s", tab.id, url, title, summarizeHTMLSnapshot(html)),
	}, nil
}

func (s *session) Tabs(ctx context.Context, req TabsRequest) (ActionResult, error) {
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "list"
	}
	switch action {
	case "list":
		return s.tabsResult("listed tabs"), nil
	case "new":
		tab, err := s.createTab(ctx, strings.TrimSpace(req.URL))
		if err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		res := s.tabsSnapshot()
		res.Message = fmt.Sprintf("opened %s as %s", tab.currentURL(), tab.id)
		return res, nil
	case "switch":
		tab, err := s.resolveTab(req)
		if err != nil {
			return ActionResult{}, err
		}
		if _, _, _, err := s.refreshTabPageState(ctx, tab); err != nil && !errors.Is(err, ErrNoLoadedPage) {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		s.setActiveTab(tab.id)
		res := s.tabsSnapshot()
		res.Message = fmt.Sprintf("switched to %s", tab.id)
		return res, nil
	case "close":
		current, err := s.resolveTab(req)
		if err != nil {
			return ActionResult{}, err
		}
		if s.tabCount() == 1 {
			if _, err := s.createTab(ctx, "about:blank"); err != nil {
				s.runtime.setLastError(err)
				return ActionResult{}, err
			}
		}
		if err := s.closeTab(ctx, current.id); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		res := s.tabsSnapshot()
		res.Message = fmt.Sprintf("closed %s", current.id)
		return res, nil
	default:
		return ActionResult{}, fmt.Errorf("unsupported tabs action %q", action)
	}
}

func (s *session) Back(ctx context.Context) (ActionResult, error) {
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	if !tab.hasPage() {
		return ActionResult{}, ErrNoLoadedPage
	}
	if err := tab.run(ctx,
		chromedp.NavigateBack(),
		chromedp.WaitReady("html", chromedp.ByQuery),
	); err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	url, title, _, err := s.refreshTabPageState(ctx, tab)
	if err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	return ActionResult{Message: fmt.Sprintf("navigated %s back to %s (title=%q)", tab.id, url, title)}, nil
}

func (s *session) Forward(ctx context.Context) (ActionResult, error) {
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	if !tab.hasPage() {
		return ActionResult{}, ErrNoLoadedPage
	}
	if err := tab.run(ctx,
		chromedp.NavigateForward(),
		chromedp.WaitReady("html", chromedp.ByQuery),
	); err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	url, title, _, err := s.refreshTabPageState(ctx, tab)
	if err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	return ActionResult{Message: fmt.Sprintf("navigated %s forward to %s (title=%q)", tab.id, url, title)}, nil
}

func (s *session) Click(ctx context.Context, req ClickRequest) (ActionResult, error) {
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	selector := strings.TrimSpace(req.Selector)
	if selector == "" {
		return ActionResult{}, fmt.Errorf("missing selector")
	}
	if err := tab.run(ctx, chromedp.Click(selector, chromedp.ByQuery)); err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	if _, _, _, err := s.refreshTabPageState(ctx, tab); err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	return ActionResult{Message: fmt.Sprintf("clicked %s on %s", selector, tab.id)}, nil
}

func (s *session) Hover(ctx context.Context, req HoverRequest) (ActionResult, error) {
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	selector := strings.TrimSpace(req.Selector)
	if selector == "" {
		return ActionResult{}, fmt.Errorf("missing selector")
	}
	selectorJSON, _ := json.Marshal(selector)
	script := fmt.Sprintf(`(() => {
		const el = document.querySelector(%s);
		if (!el) {
			throw new Error("selector not found: " + %s);
		}
		el.scrollIntoView({block: "center", inline: "center"});
		for (const type of ["mouseover", "mouseenter", "mousemove"]) {
			el.dispatchEvent(new MouseEvent(type, {bubbles: true, cancelable: true, view: window}));
		}
		return true;
	})()`, string(selectorJSON), string(selectorJSON))
	if err := tab.run(ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Evaluate(script, nil),
	); err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	if _, _, _, err := s.refreshTabPageState(ctx, tab); err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	return ActionResult{Message: fmt.Sprintf("hovered %s on %s", selector, tab.id)}, nil
}

func (s *session) Type(ctx context.Context, req TypeRequest) (ActionResult, error) {
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	selector := strings.TrimSpace(req.Selector)
	if selector == "" {
		return ActionResult{}, fmt.Errorf("missing selector")
	}
	if err := tab.run(ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.SetValue(selector, "", chromedp.ByQuery),
		chromedp.SendKeys(selector, req.Text, chromedp.ByQuery),
	); err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	if _, _, _, err := s.refreshTabPageState(ctx, tab); err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	return ActionResult{Message: fmt.Sprintf("typed into %s on %s", selector, tab.id)}, nil
}

func (s *session) PressKey(ctx context.Context, req KeyRequest) (ActionResult, error) {
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	keys := req.Keys
	if strings.TrimSpace(keys) == "" {
		return ActionResult{}, fmt.Errorf("missing keys")
	}
	selector := strings.TrimSpace(req.Selector)
	if selector != "" {
		err = tab.run(ctx,
			chromedp.WaitVisible(selector, chromedp.ByQuery),
			chromedp.Focus(selector, chromedp.ByQuery),
			chromedp.SendKeys(selector, keys, chromedp.ByQuery),
		)
	} else {
		err = tab.run(ctx, chromedp.KeyEvent(keys))
	}
	if err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	if _, _, _, err := s.refreshTabPageState(ctx, tab); err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	if selector != "" {
		return ActionResult{Message: fmt.Sprintf("sent keys to %s on %s", selector, tab.id)}, nil
	}
	return ActionResult{Message: fmt.Sprintf("sent keys %q on %s", keys, tab.id)}, nil
}

func (s *session) Select(ctx context.Context, req SelectRequest) (ActionResult, error) {
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	selector := strings.TrimSpace(req.Selector)
	if selector == "" {
		return ActionResult{}, fmt.Errorf("missing selector")
	}
	values := compactValues(req.Values)
	if len(values) == 0 {
		return ActionResult{}, fmt.Errorf("missing values")
	}
	selected, err := s.selectValues(ctx, tab, selector, values)
	if err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	if _, _, _, err := s.refreshTabPageState(ctx, tab); err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	return ActionResult{Message: fmt.Sprintf("selected %s on %s (%s)", strings.Join(selected, ","), selector, tab.id)}, nil
}

func (s *session) Wait(ctx context.Context, req WaitRequest) (ActionResult, error) {
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	if !tab.hasPage() {
		return ActionResult{}, ErrNoLoadedPage
	}
	timeout := time.Duration(req.Timeout) * time.Millisecond
	if timeout <= 0 {
		timeout = time.Second
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	selector := strings.TrimSpace(req.Selector)
	if selector != "" {
		if err := tab.run(waitCtx, chromedp.WaitVisible(selector, chromedp.ByQuery)); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		if _, _, _, err := s.refreshTabPageState(waitCtx, tab); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		return ActionResult{Message: fmt.Sprintf("waited for %s on %s", selector, tab.id)}, nil
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-waitCtx.Done():
		return ActionResult{}, waitCtx.Err()
	case <-timer.C:
	}
	return ActionResult{Message: fmt.Sprintf("waited %s on %s", timeout, tab.id)}, nil
}

func (s *session) Scroll(ctx context.Context, req ScrollRequest) (ActionResult, error) {
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	if !tab.hasPage() {
		return ActionResult{}, ErrNoLoadedPage
	}
	selector := strings.TrimSpace(req.Selector)
	if selector != "" {
		selectorJSON, _ := json.Marshal(selector)
		script := fmt.Sprintf(`(() => {
			const el = document.querySelector(%s);
			if (!el) {
				throw new Error("selector not found: " + %s);
			}
			el.scrollIntoView({block: "center", inline: "center"});
			return true;
		})()`, string(selectorJSON), string(selectorJSON))
		if err := tab.run(ctx, chromedp.Evaluate(script, nil)); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		if _, _, _, err := s.refreshTabPageState(ctx, tab); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		return ActionResult{Message: fmt.Sprintf("scrolled %s into view on %s", selector, tab.id)}, nil
	}
	script := fmt.Sprintf(`(() => { window.scrollBy(%d, %d); return window.scrollY; })()`, req.X, req.Y)
	var scrollY float64
	if err := tab.run(ctx, chromedp.Evaluate(script, &scrollY)); err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	if _, _, _, err := s.refreshTabPageState(ctx, tab); err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	return ActionResult{Message: fmt.Sprintf("scrolled %s to y=%.0f", tab.id, scrollY)}, nil
}

func (s *session) Screenshot(ctx context.Context, req ScreenshotRequest) (ActionResult, error) {
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	path := strings.TrimSpace(req.Path)
	if path == "" {
		return ActionResult{}, fmt.Errorf("missing screenshot path")
	}
	if !tab.hasPage() {
		return ActionResult{}, ErrNoLoadedPage
	}
	var buf []byte
	if err := tab.run(ctx, chromedp.FullScreenshot(&buf, 90)); err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ActionResult{}, err
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Message: fmt.Sprintf("saved screenshot for %s to %s", tab.id, path)}, nil
}

func (s *session) Console(ctx context.Context, req ConsoleRequest) (ActionResult, error) {
	_ = ctx
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	tab.mu.RLock()
	defer tab.mu.RUnlock()
	if len(tab.console) == 0 {
		return ActionResult{Message: "no console messages recorded"}, nil
	}
	limit := req.Limit
	if limit <= 0 || limit > len(tab.console) {
		limit = len(tab.console)
	}
	return ActionResult{Message: strings.Join(tab.console[len(tab.console)-limit:], "\n")}, nil
}

func (s *session) Network(ctx context.Context, req NetworkRequest) (ActionResult, error) {
	_ = ctx
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	tab.mu.RLock()
	defer tab.mu.RUnlock()
	if len(tab.network) == 0 {
		return ActionResult{}, ErrNoLoadedPage
	}
	limit := req.Limit
	if limit <= 0 || limit > len(tab.network) {
		limit = len(tab.network)
	}
	return ActionResult{Message: strings.Join(tab.network[len(tab.network)-limit:], "\n")}, nil
}

func (r *BuiltinRuntime) setLastError(err error) {
	if err == nil {
		return
	}
	r.mu.Lock()
	r.lastError = strings.TrimSpace(err.Error())
	r.mu.Unlock()
}

func extractHTMLTitle(html string) string {
	m := titlePattern.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(tagPattern.ReplaceAllString(m[1], " "))
}

func summarizeHTMLSnapshot(html string) string {
	text := tagPattern.ReplaceAllString(html, " ")
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 320 {
		text = text[:320] + "..."
	}
	return strings.TrimSpace(text)
}

func newSession(rt *BuiltinRuntime, traceID, execPath string) (*session, error) {
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocatorOptions(execPath)...)
	browserCtx, cancel := chromedp.NewContext(allocCtx)
	sess := &session{
		runtime:     rt,
		traceID:     traceID,
		allocCancel: allocCancel,
		cancel:      cancel,
		ctx:         browserCtx,
		createdAt:   time.Now(),
		updatedAt:   time.Now(),
	}
	initCtx, initCancel := context.WithTimeout(browserCtx, 20*time.Second)
	defer initCancel()
	if err := chromedp.Run(initCtx, chromedp.ActionFunc(func(context.Context) error { return nil })); err != nil {
		sess.Close()
		return nil, err
	}
	if _, err := sess.createTab(context.Background(), "about:blank"); err != nil {
		sess.Close()
		return nil, err
	}
	return sess, nil
}

func allocatorOptions(execPath string) []chromedp.ExecAllocatorOption {
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts,
		chromedp.ExecPath(execPath),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("window-size", "1440,1024"),
	)
	return opts
}

func (s *session) Close() {
	s.mu.Lock()
	tabs := append([]*tab(nil), s.tabs...)
	s.tabs = nil
	s.activeTabID = ""
	s.mu.Unlock()
	for _, tab := range tabs {
		if tab != nil {
			tab.cancel()
		}
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.allocCancel != nil {
		s.allocCancel()
	}
}

func (s *session) mustActiveTab() (*tab, error) {
	tab := s.activeTab()
	if tab == nil {
		return nil, ErrNoLoadedPage
	}
	return tab, nil
}

func (s *session) activeTab() *tab {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeTabLocked()
}

func (s *session) activeTabLocked() *tab {
	if s.activeTabID != "" {
		for _, tab := range s.tabs {
			if tab != nil && tab.id == s.activeTabID {
				return tab
			}
		}
	}
	if len(s.tabs) == 0 {
		return nil
	}
	return s.tabs[0]
}

func (s *session) setActiveTab(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, tab := range s.tabs {
		if tab != nil && tab.id == id {
			s.activeTabID = id
			s.updatedAt = time.Now()
			return
		}
	}
}

func (s *session) tabCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tabs)
}

func (s *session) resolveTab(req TabsRequest) (*tab, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if req.ID != "" {
		for _, tab := range s.tabs {
			if tab != nil && tab.id == strings.TrimSpace(req.ID) {
				return tab, nil
			}
		}
		return nil, fmt.Errorf("unknown tab id %q", strings.TrimSpace(req.ID))
	}
	if req.HasIndex {
		if req.Index < 0 || req.Index >= len(s.tabs) {
			return nil, fmt.Errorf("tab index %d out of range", req.Index)
		}
		if s.tabs[req.Index] == nil {
			return nil, fmt.Errorf("tab index %d unavailable", req.Index)
		}
		return s.tabs[req.Index], nil
	}
	tab := s.activeTabLocked()
	if tab == nil {
		return nil, ErrNoLoadedPage
	}
	return tab, nil
}

func (s *session) createTab(ctx context.Context, targetURL string) (*tab, error) {
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		targetURL = "about:blank"
	}
	tabCtx, cancel := chromedp.NewContext(s.ctx)
	tab := &tab{
		id:        s.nextTabName(),
		ctx:       tabCtx,
		cancel:    cancel,
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}
	s.attachListeners(tab)
	if err := tab.run(ctx,
		network.Enable(),
		cdruntime.Enable(),
		page.Enable(),
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("html", chromedp.ByQuery),
	); err != nil {
		cancel()
		return nil, err
	}
	if _, _, _, err := s.refreshTabPageState(ctx, tab); err != nil {
		cancel()
		return nil, err
	}
	s.mu.Lock()
	s.tabs = append(s.tabs, tab)
	s.activeTabID = tab.id
	s.updatedAt = time.Now()
	s.mu.Unlock()
	return tab, nil
}

func (s *session) nextTabName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextTabID++
	return fmt.Sprintf("tab-%d", s.nextTabID)
}

func (s *session) closeTab(ctx context.Context, id string) error {
	s.mu.RLock()
	idx := -1
	var victim *tab
	for i, tab := range s.tabs {
		if tab != nil && tab.id == id {
			idx = i
			victim = tab
			break
		}
	}
	s.mu.RUnlock()
	if idx < 0 || victim == nil {
		return fmt.Errorf("unknown tab id %q", id)
	}
	if err := victim.run(ctx, page.Close()); err != nil {
		return err
	}
	victim.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tabs = append(s.tabs[:idx], s.tabs[idx+1:]...)
	if s.activeTabID == id {
		s.activeTabID = ""
		if len(s.tabs) > 0 {
			next := idx
			if next >= len(s.tabs) {
				next = len(s.tabs) - 1
			}
			s.activeTabID = s.tabs[next].id
		}
	}
	s.updatedAt = time.Now()
	return nil
}

func (s *session) tabsSnapshot() ActionResult {
	infos := s.tabInfos()
	active := ""
	for _, info := range infos {
		if info.Active {
			active = info.ID
			break
		}
	}
	return ActionResult{
		Message: formatTabSummary(infos),
		Data: map[string]interface{}{
			"tabs":       infos,
			"active_tab": active,
		},
	}
}

func (s *session) tabsResult(message string) ActionResult {
	res := s.tabsSnapshot()
	if strings.TrimSpace(message) != "" {
		res.Message = message + "\n" + res.Message
	}
	return res
}

func (s *session) tabInfos() []TabInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TabInfo, 0, len(s.tabs))
	for i, tab := range s.tabs {
		if tab == nil {
			continue
		}
		tab.mu.RLock()
		info := TabInfo{
			ID:     tab.id,
			Index:  i,
			URL:    strings.TrimSpace(tab.lastURL),
			Title:  strings.TrimSpace(tab.lastTitle),
			Active: tab.id == s.activeTabID,
		}
		tab.mu.RUnlock()
		out = append(out, info)
	}
	return out
}

func formatTabSummary(infos []TabInfo) string {
	if len(infos) == 0 {
		return "no tabs"
	}
	lines := make([]string, 0, len(infos))
	for _, info := range infos {
		prefix := "-"
		if info.Active {
			prefix = "*"
		}
		title := strings.TrimSpace(info.Title)
		if title == "" {
			title = "(untitled)"
		}
		url := strings.TrimSpace(info.URL)
		if url == "" {
			url = "about:blank"
		}
		lines = append(lines, fmt.Sprintf("%s [%d] %s %s %s", prefix, info.Index, info.ID, title, url))
	}
	return strings.Join(lines, "\n")
}

func (s *session) refreshTabPageState(ctx context.Context, tab *tab) (string, string, string, error) {
	if tab == nil {
		return "", "", "", ErrNoLoadedPage
	}
	var (
		url   string
		title string
		html  string
	)
	if err := tab.run(ctx,
		chromedp.Location(&url),
		chromedp.Title(&title),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	); err != nil {
		return "", "", "", err
	}
	url = strings.TrimSpace(url)
	if url == "" {
		return "", "", "", ErrNoLoadedPage
	}
	tab.mu.Lock()
	tab.updatedAt = time.Now()
	tab.lastURL = url
	tab.lastTitle = strings.TrimSpace(title)
	tab.lastHTML = html
	tab.mu.Unlock()
	s.mu.Lock()
	s.updatedAt = time.Now()
	s.mu.Unlock()
	return url, strings.TrimSpace(title), html, nil
}

func (s *session) selectValues(ctx context.Context, tab *tab, selector string, values []string) ([]string, error) {
	selectorJSON, _ := json.Marshal(selector)
	valuesJSON, _ := json.Marshal(values)
	script := fmt.Sprintf(`(() => {
		const selector = %s;
		const values = %s.map(String);
		const el = document.querySelector(selector);
		if (!el) {
			throw new Error("selector not found: " + selector);
		}
		if (!(el instanceof HTMLSelectElement)) {
			throw new Error("selector is not a select element: " + selector);
		}
		const wanted = new Set(values);
		for (const option of Array.from(el.options)) {
			option.selected = wanted.has(String(option.value)) || wanted.has(String(option.text));
		}
		el.dispatchEvent(new Event("input", { bubbles: true }));
		el.dispatchEvent(new Event("change", { bubbles: true }));
		return Array.from(el.selectedOptions).map((option) => String(option.value || option.text));
	})()`, string(selectorJSON), string(valuesJSON))
	var selected []string
	if err := tab.run(ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Evaluate(script, &selected),
	); err != nil {
		return nil, err
	}
	return selected, nil
}

func (s *session) attachListeners(tab *tab) {
	chromedp.ListenTarget(tab.ctx, func(ev any) {
		switch evt := ev.(type) {
		case *cdruntime.EventConsoleAPICalled:
			tab.appendConsole(formatConsoleEvent(evt))
		case *cdruntime.EventExceptionThrown:
			if evt.ExceptionDetails != nil {
				tab.appendConsole(strings.TrimSpace(evt.ExceptionDetails.Error()))
			}
		case *network.EventRequestWillBeSent:
			if evt.Request != nil {
				tab.appendNetwork(fmt.Sprintf("%s %s", evt.Request.Method, strings.TrimSpace(evt.Request.URL)))
			}
		case *network.EventResponseReceived:
			if evt.Response != nil {
				tab.appendNetwork(fmt.Sprintf("%s -> %d", strings.TrimSpace(evt.Response.URL), int(evt.Response.Status)))
			}
		}
	})
}

func (t *tab) hasPage() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return strings.TrimSpace(t.lastURL) != ""
}

func (t *tab) currentURL() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if strings.TrimSpace(t.lastURL) == "" {
		return "about:blank"
	}
	return strings.TrimSpace(t.lastURL)
}

func (t *tab) run(parent context.Context, actions ...chromedp.Action) error {
	t.opMu.Lock()
	defer t.opMu.Unlock()
	opCtx, cleanup := t.operationContext(parent)
	defer cleanup()
	return chromedp.Run(opCtx, actions...)
}

func (t *tab) operationContext(parent context.Context) (context.Context, func()) {
	var (
		opCtx  context.Context
		cancel context.CancelFunc
	)
	if parent != nil {
		if deadline, ok := parent.Deadline(); ok {
			opCtx, cancel = context.WithDeadline(t.ctx, deadline)
		} else {
			opCtx, cancel = context.WithCancel(t.ctx)
		}
		stop := make(chan struct{})
		go func() {
			select {
			case <-parent.Done():
				cancel()
			case <-stop:
			}
		}()
		return opCtx, func() {
			close(stop)
			cancel()
		}
	}
	opCtx, cancel = context.WithCancel(t.ctx)
	return opCtx, cancel
}

func (t *tab) appendConsole(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.console = appendBounded(t.console, line, 200)
}

func (t *tab) appendNetwork(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.network = appendBounded(t.network, line, 400)
}

func appendBounded(lines []string, line string, max int) []string {
	lines = append(lines, line)
	if max > 0 && len(lines) > max {
		lines = append([]string(nil), lines[len(lines)-max:]...)
	}
	return lines
}

func formatConsoleEvent(evt *cdruntime.EventConsoleAPICalled) string {
	if evt == nil {
		return ""
	}
	parts := make([]string, 0, len(evt.Args))
	for _, arg := range evt.Args {
		parts = append(parts, formatRemoteObject(arg))
	}
	if len(parts) == 0 {
		return string(evt.Type)
	}
	return fmt.Sprintf("%s: %s", evt.Type, strings.Join(parts, " "))
}

func formatRemoteObject(obj *cdruntime.RemoteObject) string {
	if obj == nil {
		return ""
	}
	if len(obj.Value) > 0 {
		var v any
		if err := json.Unmarshal(obj.Value, &v); err == nil {
			return strings.TrimSpace(fmt.Sprint(v))
		}
		return strings.TrimSpace(obj.Value.String())
	}
	if obj.UnserializableValue != "" {
		return strings.TrimSpace(string(obj.UnserializableValue))
	}
	if strings.TrimSpace(obj.Description) != "" {
		return strings.TrimSpace(obj.Description)
	}
	if strings.TrimSpace(obj.ClassName) != "" {
		return strings.TrimSpace(obj.ClassName)
	}
	return strings.TrimSpace(string(obj.Type))
}

func compactValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func findBrowserBinary() string {
	candidates := []string{
		os.Getenv("CHROME_BIN"),
		os.Getenv("CHROMIUM_BIN"),
		os.Getenv("BROWSER_BIN"),
	}
	switch goruntime.GOOS {
	case "darwin":
		candidates = append(candidates,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		)
	case "windows":
		candidates = append(candidates,
			"chrome",
			"chrome.exe",
			"chromium.exe",
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		)
	default:
		candidates = append(candidates,
			"headless_shell",
			"headless-shell",
			"chromium",
			"chromium-browser",
			"google-chrome",
			"google-chrome-stable",
			"google-chrome-beta",
			"chrome",
			"/usr/bin/google-chrome",
			"/snap/bin/chromium",
		)
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if fullPath, err := exec.LookPath(candidate); err == nil {
			return fullPath
		}
	}
	return ""
}

func noBrowserMessage() string {
	return "install Chrome/Chromium or set CHROME_BIN/CHROMIUM_BIN"
}
