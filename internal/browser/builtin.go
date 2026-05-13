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

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
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

type SnapshotElement struct {
	Ref         string `json:"ref"`
	Role        string `json:"role"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value,omitempty"`
	Selector    string `json:"selector"`
	Tag         string `json:"tag"`
	Type        string `json:"type"`
	Text        string `json:"text"`
	Source      string `json:"source"`
}

type TabsRequest struct {
	Action      string
	ID          string
	Index       int
	HasIndex    bool
	Query       string
	URL         string
	Activate    bool
	HasActivate bool
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
	Ref      string
}

type KeyRequest struct {
	Selector string
	Ref      string
	Keys     string
}

type ScrollRequest struct {
	Selector string
	Ref      string
	X        int
	Y        int
}

type ClickRequest struct {
	Selector string
	Ref      string
}

type TypeRequest struct {
	Selector string
	Ref      string
	Text     string
}

type SelectRequest struct {
	Selector string
	Ref      string
	Values   []string
}

type WaitRequest struct {
	Selector string
	Ref      string
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

type SessionSnapshot struct {
	TraceID   string    `json:"trace_id"`
	UpdatedAt time.Time `json:"updated_at"`
	ActiveTab string    `json:"active_tab"`
	TabCount  int       `json:"tab_count"`
	Tabs      []TabInfo `json:"tabs"`
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

	mu          sync.RWMutex
	createdAt   time.Time
	updatedAt   time.Time
	activatedAt time.Time
	lastURL     string
	lastTitle   string
	lastHTML    string
	elements    []SnapshotElement
	console     []string
	network     []string
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

func (r *BuiltinRuntime) SessionSnapshots() []SessionSnapshot {
	r.mu.RLock()
	sessions := make([]*session, 0, len(r.sessions))
	for _, sess := range r.sessions {
		if sess != nil {
			sessions = append(sessions, sess)
		}
	}
	r.mu.RUnlock()
	out := make([]SessionSnapshot, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, sess.snapshot())
	}
	return out
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
	elements, err := s.captureSnapshotElements(ctx, tab)
	if err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	return ActionResult{
		Message: formatSnapshotMessage(tab.id, url, title, elements, summarizeHTMLSnapshot(html)),
		Data: map[string]interface{}{
			"tab":           s.tabInfo(tab),
			"url":           url,
			"title":         title,
			"elements":      elements,
			"element_count": len(elements),
		},
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
	case "current":
		tab, err := s.mustActiveTab()
		if err != nil {
			return ActionResult{}, err
		}
		if _, _, _, err := s.refreshTabPageState(ctx, tab); err != nil && !errors.Is(err, ErrNoLoadedPage) {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		info := s.tabInfo(tab)
		res := s.tabsSnapshot()
		res.Message = fmt.Sprintf("current tab is %s", tab.id)
		res.Data["tab"] = info
		res.Data["target_tab"] = tab.id
		return res, nil
	case "new":
		tab, err := s.createTab(ctx, strings.TrimSpace(req.URL), req.activateRequested())
		if err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		res := s.tabsSnapshot()
		if req.activateRequested() {
			res.Message = fmt.Sprintf("opened %s as %s", tab.currentURL(), tab.id)
		} else {
			res.Message = fmt.Sprintf("opened background tab %s as %s", tab.currentURL(), tab.id)
		}
		res.Data["opened_tab"] = tab.id
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
		res.Data["target_tab"] = tab.id
		return res, nil
	case "activate_last":
		tab, err := s.activateLastTab()
		if err != nil {
			return ActionResult{}, err
		}
		if _, _, _, err := s.refreshTabPageState(ctx, tab); err != nil && !errors.Is(err, ErrNoLoadedPage) {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		res := s.tabsSnapshot()
		res.Message = fmt.Sprintf("activated last tab %s", tab.id)
		res.Data["target_tab"] = tab.id
		return res, nil
	case "close_others":
		target, err := s.resolveTab(req)
		if err != nil {
			return ActionResult{}, err
		}
		closed, err := s.closeOtherTabs(ctx, target.id)
		if err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		res := s.tabsSnapshot()
		res.Message = fmt.Sprintf("closed %d other tabs around %s", len(closed), target.id)
		res.Data["target_tab"] = target.id
		res.Data["closed_tabs"] = closed
		return res, nil
	case "close_right":
		target, err := s.resolveTab(req)
		if err != nil {
			return ActionResult{}, err
		}
		closed, err := s.closeTabsRightOf(ctx, target.id)
		if err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		res := s.tabsSnapshot()
		res.Message = fmt.Sprintf("closed %d tabs to the right of %s", len(closed), target.id)
		res.Data["target_tab"] = target.id
		res.Data["closed_tabs"] = closed
		return res, nil
	case "close":
		current, err := s.resolveTab(req)
		if err != nil {
			return ActionResult{}, err
		}
		if s.tabCount() == 1 {
			if _, err := s.createTab(ctx, "about:blank", true); err != nil {
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
		res.Data["closed_tab"] = current.id
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
		selector, err = s.resolveActionSelector(ctx, tab, req.Ref)
		if err != nil {
			return ActionResult{}, err
		}
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
		selector, err = s.resolveActionSelector(ctx, tab, req.Ref)
		if err != nil {
			return ActionResult{}, err
		}
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
		selector, err = s.resolveActionSelector(ctx, tab, req.Ref)
		if err != nil {
			return ActionResult{}, err
		}
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
	if selector == "" && strings.TrimSpace(req.Ref) != "" {
		selector, err = s.resolveActionSelector(ctx, tab, req.Ref)
		if err != nil {
			return ActionResult{}, err
		}
	}
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
		selector, err = s.resolveActionSelector(ctx, tab, req.Ref)
		if err != nil {
			return ActionResult{}, err
		}
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
	if selector == "" && strings.TrimSpace(req.Ref) != "" {
		selector, err = s.resolveActionSelector(waitCtx, tab, req.Ref)
		if err != nil {
			return ActionResult{}, err
		}
	}
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
	if selector == "" && strings.TrimSpace(req.Ref) != "" {
		selector, err = s.resolveActionSelector(ctx, tab, req.Ref)
		if err != nil {
			return ActionResult{}, err
		}
	}
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

func formatSnapshotMessage(tabID, url, title string, elements []SnapshotElement, summary string) string {
	lines := []string{
		fmt.Sprintf("tab=%s", tabID),
		fmt.Sprintf("url=%s", url),
		fmt.Sprintf("title=%s", title),
	}
	if len(elements) > 0 {
		lines = append(lines, "refs:")
		for _, el := range elements {
			label := strings.TrimSpace(el.Name)
			if label == "" {
				label = strings.TrimSpace(el.Value)
			}
			if label == "" {
				label = strings.TrimSpace(el.Text)
			}
			if label == "" {
				label = "(unnamed)"
			}
			role := strings.TrimSpace(el.Role)
			if role == "" {
				role = strings.TrimSpace(el.Tag)
			}
			source := strings.TrimSpace(el.Source)
			if source == "" {
				source = "dom"
			}
			lines = append(lines, fmt.Sprintf("- [%s] %s %q {%s}", el.Ref, role, label, source))
		}
	}
	lines = append(lines, "snapshot="+summary)
	return strings.Join(lines, "\n")
}

type axDOMBinding struct {
	Ref      string `json:"ref"`
	Selector string `json:"selector"`
	Tag      string `json:"tag"`
	Type     string `json:"type"`
	Text     string `json:"text"`
	Name     string `json:"name"`
}

func bindAXNodeToDOM(ctx context.Context, backendNodeID cdp.BackendNodeID) (axDOMBinding, error) {
	if backendNodeID == 0 {
		return axDOMBinding{}, fmt.Errorf("missing backend node id")
	}
	object, err := dom.ResolveNode().WithBackendNodeID(backendNodeID).Do(ctx)
	if err != nil {
		return axDOMBinding{}, err
	}
	if object == nil || object.ObjectID == "" {
		return axDOMBinding{}, fmt.Errorf("unable to resolve backend node %d", backendNodeID)
	}
	const fn = `function() {
		const normalize = (value) => String(value || '').replace(/\s+/g, ' ').trim();
		const cssEscape = (value) => {
			if (window.CSS && typeof window.CSS.escape === 'function') return window.CSS.escape(value);
			return String(value).replace(/["\\]/g, '\\$&');
		};
		const root = document.documentElement;
		let ref = this.getAttribute && this.getAttribute('data-eos-ref');
		if (!ref) {
			let counter = Number(root && root.getAttribute('data-eos-ref-counter') || '0') + 1;
			ref = 'e' + String(counter);
			if (root) root.setAttribute('data-eos-ref-counter', String(counter));
			if (this.setAttribute) this.setAttribute('data-eos-ref', ref);
		}
		let selector = '';
		if (this.id) selector = '#' + cssEscape(this.id);
		else selector = '[data-eos-ref="' + cssEscape(ref) + '"]';
		return {
			ref,
			selector,
			tag: normalize(this.tagName || '').toLowerCase(),
			type: normalize(this.getAttribute && this.getAttribute('type')),
			text: normalize(this.innerText || this.textContent),
			name: normalize((this.getAttribute && this.getAttribute('aria-label')) || '')
		};
	}`
	result, exceptionDetails, err := cdruntime.CallFunctionOn(fn).
		WithObjectID(object.ObjectID).
		WithReturnByValue(true).
		WithAwaitPromise(true).
		Do(ctx)
	if err != nil {
		return axDOMBinding{}, err
	}
	if exceptionDetails != nil {
		return axDOMBinding{}, fmt.Errorf("failed to bind backend node %d", backendNodeID)
	}
	var binding axDOMBinding
	if err := json.Unmarshal(result.Value, &binding); err != nil {
		return axDOMBinding{}, err
	}
	if strings.TrimSpace(binding.Ref) == "" || strings.TrimSpace(binding.Selector) == "" {
		return axDOMBinding{}, fmt.Errorf("invalid binding for backend node %d", backendNodeID)
	}
	return binding, nil
}

func isInterestingAXNode(node *accessibility.Node) bool {
	if node == nil || node.Ignored || node.BackendDOMNodeID == 0 {
		return false
	}
	role := strings.ToLower(strings.TrimSpace(axValueString(node.Role)))
	if role == "" || role == "none" || role == "generic" || role == "section" || role == "strong" || role == "statictext" || role == "inlineTextBox" {
		return false
	}
	if axPropertyTruthy(node.Properties, accessibility.PropertyNameFocusable) ||
		axPropertyTruthy(node.Properties, accessibility.PropertyNameEditable) ||
		axPropertyTruthy(node.Properties, accessibility.PropertyNameSettable) {
		return true
	}
	if strings.TrimSpace(axValueString(node.Name)) != "" || strings.TrimSpace(axValueString(node.Value)) != "" {
		return true
	}
	switch role {
	case "button", "link", "textbox", "searchbox", "combobox", "checkbox", "radio", "switch", "menuitem", "menuitemcheckbox", "menuitemradio", "option", "tab", "slider", "spinbutton":
		return true
	default:
		return false
	}
}

func axValueString(v *accessibility.Value) string {
	if v == nil || len(v.Value) == 0 {
		return ""
	}
	var raw any
	if err := json.Unmarshal(v.Value, &raw); err == nil {
		return strings.TrimSpace(fmt.Sprint(raw))
	}
	return strings.TrimSpace(v.Value.String())
}

func axPropertyValue(props []*accessibility.Property, name accessibility.PropertyName) string {
	for _, prop := range props {
		if prop != nil && prop.Name == name {
			return axValueString(prop.Value)
		}
	}
	return ""
}

func axPropertyTruthy(props []*accessibility.Property, name accessibility.PropertyName) bool {
	value := strings.ToLower(strings.TrimSpace(axPropertyValue(props, name)))
	return value == "true" || value == "1"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
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
	if _, err := sess.createTab(context.Background(), "about:blank", true); err != nil {
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
			tab.mu.Lock()
			tab.activatedAt = time.Now()
			tab.mu.Unlock()
			s.updatedAt = time.Now()
			return
		}
	}
}

func (s *session) activateLastTab() (*tab, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	currentID := s.activeTabID
	targetID := s.preferredActiveTabIDLocked(currentID)
	if strings.TrimSpace(targetID) == "" {
		if current := s.activeTabLocked(); current != nil {
			return current, nil
		}
		return nil, ErrNoLoadedPage
	}
	for _, tab := range s.tabs {
		if tab != nil && tab.id == targetID {
			s.activeTabID = targetID
			tab.mu.Lock()
			tab.activatedAt = time.Now()
			tab.mu.Unlock()
			s.updatedAt = time.Now()
			return tab, nil
		}
	}
	return nil, ErrNoLoadedPage
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
	if query := strings.ToLower(strings.TrimSpace(req.Query)); query != "" {
		if tab, err := s.findTabByQueryLocked(query); err == nil {
			return tab, nil
		} else {
			return nil, err
		}
	}
	tab := s.activeTabLocked()
	if tab == nil {
		return nil, ErrNoLoadedPage
	}
	return tab, nil
}

func (s *session) findTabByQueryLocked(query string) (*tab, error) {
	matches := make([]*tab, 0, len(s.tabs))
	exact := make([]*tab, 0, len(s.tabs))
	for _, tab := range s.tabs {
		if tab == nil {
			continue
		}
		tab.mu.RLock()
		id := strings.ToLower(strings.TrimSpace(tab.id))
		title := strings.ToLower(strings.TrimSpace(tab.lastTitle))
		url := strings.ToLower(strings.TrimSpace(tab.lastURL))
		tab.mu.RUnlock()
		if id == query || title == query || url == query {
			exact = append(exact, tab)
			continue
		}
		if strings.Contains(id, query) || strings.Contains(title, query) || strings.Contains(url, query) {
			matches = append(matches, tab)
		}
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(exact) > 1 {
		return s.chooseMatchedTabLocked(exact, query)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no tab matched %q", query)
	}
	return s.chooseMatchedTabLocked(matches, query)
}

func (s *session) chooseMatchedTabLocked(candidates []*tab, query string) (*tab, error) {
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if s.activeTabID != "" {
		for _, tab := range candidates {
			if tab != nil && tab.id == s.activeTabID {
				return tab, nil
			}
		}
	}
	ids := make([]string, 0, len(candidates))
	for _, tab := range candidates {
		if tab != nil {
			ids = append(ids, tab.id)
		}
	}
	return nil, fmt.Errorf("multiple tabs matched %q: %s", query, strings.Join(ids, ", "))
}

func (s *session) createTab(ctx context.Context, targetURL string, activate bool) (*tab, error) {
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		targetURL = "about:blank"
	}
	tabCtx, cancel := chromedp.NewContext(s.ctx)
	tab := &tab{
		id:          s.nextTabName(),
		ctx:         tabCtx,
		cancel:      cancel,
		createdAt:   time.Now(),
		updatedAt:   time.Now(),
		activatedAt: time.Now(),
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
	s.updatedAt = time.Now()
	if activate || s.activeTabID == "" {
		s.activeTabID = tab.id
	} else {
		tab.mu.Lock()
		tab.activatedAt = time.Time{}
		tab.mu.Unlock()
	}
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
		s.activeTabID = s.preferredActiveTabIDLocked("")
	}
	s.updatedAt = time.Now()
	return nil
}

func (s *session) closeOtherTabs(ctx context.Context, keepID string) ([]string, error) {
	s.mu.RLock()
	ids := make([]string, 0, len(s.tabs))
	for _, tab := range s.tabs {
		if tab != nil && tab.id != keepID {
			ids = append(ids, tab.id)
		}
	}
	s.mu.RUnlock()
	if err := s.closeTabsByID(ctx, ids, keepID); err != nil {
		return nil, err
	}
	return ids, nil
}

func (s *session) closeTabsRightOf(ctx context.Context, targetID string) ([]string, error) {
	s.mu.RLock()
	targetIndex := -1
	ids := make([]string, 0, len(s.tabs))
	for i, tab := range s.tabs {
		if tab == nil {
			continue
		}
		if tab.id == targetID {
			targetIndex = i
		}
	}
	if targetIndex >= 0 {
		for i := targetIndex + 1; i < len(s.tabs); i++ {
			tab := s.tabs[i]
			if tab != nil {
				ids = append(ids, tab.id)
			}
		}
	}
	s.mu.RUnlock()
	if targetIndex < 0 {
		return nil, fmt.Errorf("unknown tab id %q", targetID)
	}
	if err := s.closeTabsByID(ctx, ids, targetID); err != nil {
		return nil, err
	}
	return ids, nil
}

func (s *session) closeTabsByID(ctx context.Context, ids []string, preferredActiveID string) error {
	closedSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		closedSet[id] = struct{}{}
	}
	if len(closedSet) == 0 {
		if strings.TrimSpace(preferredActiveID) != "" {
			s.setActiveTab(preferredActiveID)
		}
		return nil
	}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if err := s.closeTab(ctx, id); err != nil {
			return err
		}
	}
	if strings.TrimSpace(preferredActiveID) != "" {
		if _, ok := closedSet[preferredActiveID]; !ok {
			s.setActiveTab(preferredActiveID)
		}
	}
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
			"count":      len(infos),
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
		info := s.tabInfoLocked(tab, i)
		tab.mu.RUnlock()
		out = append(out, info)
	}
	return out
}

func (s *session) tabInfo(tab *tab) TabInfo {
	if tab == nil {
		return TabInfo{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i, candidate := range s.tabs {
		if candidate == tab {
			candidate.mu.RLock()
			defer candidate.mu.RUnlock()
			return s.tabInfoLocked(candidate, i)
		}
	}
	tab.mu.RLock()
	defer tab.mu.RUnlock()
	return TabInfo{
		ID:     tab.id,
		Index:  -1,
		URL:    strings.TrimSpace(tab.lastURL),
		Title:  strings.TrimSpace(tab.lastTitle),
		Active: tab.id == s.activeTabID,
	}
}

func (s *session) tabInfoLocked(tab *tab, index int) TabInfo {
	return TabInfo{
		ID:     tab.id,
		Index:  index,
		URL:    strings.TrimSpace(tab.lastURL),
		Title:  strings.TrimSpace(tab.lastTitle),
		Active: tab.id == s.activeTabID,
	}
}

func (s *session) snapshot() SessionSnapshot {
	s.mu.RLock()
	traceID := s.traceID
	updatedAt := s.updatedAt
	activeTab := s.activeTabID
	s.mu.RUnlock()
	tabs := s.tabInfos()
	return SessionSnapshot{
		TraceID:   traceID,
		UpdatedAt: updatedAt,
		ActiveTab: activeTab,
		TabCount:  len(tabs),
		Tabs:      tabs,
	}
}

func (s *session) preferredActiveTabIDLocked(excludeID string) string {
	var (
		chosenID string
		bestAt   time.Time
	)
	for _, tab := range s.tabs {
		if tab == nil || tab.id == excludeID {
			continue
		}
		tab.mu.RLock()
		activatedAt := tab.activatedAt
		tab.mu.RUnlock()
		if chosenID == "" || activatedAt.After(bestAt) {
			chosenID = tab.id
			bestAt = activatedAt
		}
	}
	return chosenID
}

func (r TabsRequest) activateRequested() bool {
	if r.HasActivate {
		return r.Activate
	}
	return true
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

func (s *session) captureSnapshotElements(ctx context.Context, tab *tab) ([]SnapshotElement, error) {
	elements, err := s.captureAXSnapshotElements(ctx, tab)
	if err == nil && len(elements) > 0 {
		tab.mu.Lock()
		tab.elements = append([]SnapshotElement(nil), elements...)
		tab.mu.Unlock()
		return elements, nil
	}
	return s.captureDOMSnapshotElements(ctx, tab)
}

func (s *session) captureDOMSnapshotElements(ctx context.Context, tab *tab) ([]SnapshotElement, error) {
	if tab == nil {
		return nil, ErrNoLoadedPage
	}
	const script = `(() => {
		const root = document.documentElement;
		if (!root) {
			return [];
		}
		const interesting = Array.from(document.querySelectorAll('a,button,input,select,textarea,summary,[role],[contenteditable=""],[contenteditable="true"],[tabindex]'));
		const isVisible = (el) => {
			const style = window.getComputedStyle(el);
			if (!style || style.visibility === 'hidden' || style.display === 'none') return false;
			const rect = el.getBoundingClientRect();
			return rect.width > 0 && rect.height > 0;
		};
		const normalize = (value) => String(value || '').replace(/\s+/g, ' ').trim();
		const cssEscape = (value) => {
			if (window.CSS && typeof window.CSS.escape === 'function') return window.CSS.escape(value);
			return String(value).replace(/["\\]/g, '\\$&');
		};
		const roleOf = (el) => {
			const explicit = normalize(el.getAttribute('role'));
			if (explicit) return explicit;
			const tag = el.tagName.toLowerCase();
			switch (tag) {
				case 'a': return 'link';
				case 'button': return 'button';
				case 'input': {
					const type = normalize(el.getAttribute('type')) || 'text';
					if (type === 'checkbox') return 'checkbox';
					if (type === 'radio') return 'radio';
					if (type === 'submit' || type === 'button' || type === 'reset') return 'button';
					return 'textbox';
				}
				case 'select': return 'combobox';
				case 'textarea': return 'textbox';
				default: return tag;
			}
		};
		const nameOf = (el) => {
			const aria = normalize(el.getAttribute('aria-label'));
			if (aria) return aria;
			const labelledBy = normalize(el.getAttribute('aria-labelledby'));
			if (labelledBy) {
				const text = labelledBy.split(/\s+/).map((id) => {
					const node = document.getElementById(id);
					return normalize(node ? node.textContent : '');
				}).filter(Boolean).join(' ');
				if (text) return text;
			}
			if (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || el instanceof HTMLSelectElement) {
				const placeholder = normalize(el.getAttribute('placeholder'));
				if (placeholder) return placeholder;
				const value = normalize(el.value);
				if (value) return value;
			}
			return normalize(el.innerText || el.textContent);
		};
		const selectorOf = (el, ref) => {
			if (el.id) return '#' + cssEscape(el.id);
			return '[data-eos-ref="' + cssEscape(ref) + '"]';
		};
		let counter = Number(root.getAttribute('data-eos-ref-counter') || '0');
		const out = [];
		for (const el of interesting) {
			if (!(el instanceof HTMLElement) || !isVisible(el)) continue;
			let ref = normalize(el.getAttribute('data-eos-ref'));
			if (!ref) {
				counter += 1;
				ref = 'e' + String(counter);
				el.setAttribute('data-eos-ref', ref);
			}
			out.push({
				ref,
				role: roleOf(el),
				name: nameOf(el),
				description: '',
				value: '',
				selector: selectorOf(el, ref),
				tag: el.tagName.toLowerCase(),
				type: normalize(el.getAttribute('type')),
				text: normalize(el.innerText || el.textContent),
				source: 'dom',
			});
		}
		root.setAttribute('data-eos-ref-counter', String(counter));
		return out;
	})()`
	var elements []SnapshotElement
	if err := tab.run(ctx, chromedp.Evaluate(script, &elements)); err != nil {
		return nil, err
	}
	tab.mu.Lock()
	tab.elements = append([]SnapshotElement(nil), elements...)
	tab.mu.Unlock()
	return elements, nil
}

func (s *session) captureAXSnapshotElements(ctx context.Context, tab *tab) ([]SnapshotElement, error) {
	if tab == nil {
		return nil, ErrNoLoadedPage
	}
	var elements []SnapshotElement
	err := tab.withTargetContext(ctx, func(opCtx context.Context) error {
		if err := accessibility.Enable().Do(opCtx); err != nil {
			return err
		}
		nodes, err := accessibility.GetFullAXTree().Do(opCtx)
		if err != nil {
			return err
		}
		elements = make([]SnapshotElement, 0, len(nodes))
		seenBackend := make(map[cdp.BackendNodeID]struct{}, len(nodes))
		for _, node := range nodes {
			if !isInterestingAXNode(node) {
				continue
			}
			if _, ok := seenBackend[node.BackendDOMNodeID]; ok {
				continue
			}
			binding, err := bindAXNodeToDOM(opCtx, node.BackendDOMNodeID)
			if err != nil {
				continue
			}
			element := SnapshotElement{
				Ref:         binding.Ref,
				Role:        axValueString(node.Role),
				Name:        firstNonEmpty(axValueString(node.Name), binding.Name, binding.Text),
				Description: axValueString(node.Description),
				Value:       axValueString(node.Value),
				Selector:    binding.Selector,
				Tag:         binding.Tag,
				Type:        binding.Type,
				Text:        binding.Text,
				Source:      "ax",
			}
			if strings.TrimSpace(element.Role) == "" {
				element.Role = strings.TrimSpace(binding.Tag)
			}
			if strings.TrimSpace(element.Name) == "" && strings.TrimSpace(element.Value) == "" && strings.TrimSpace(element.Text) == "" {
				continue
			}
			seenBackend[node.BackendDOMNodeID] = struct{}{}
			elements = append(elements, element)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return elements, nil
}

func (s *session) resolveActionSelector(ctx context.Context, tab *tab, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("missing selector or ref")
	}
	if selector := tab.selectorForRef(ref); selector != "" {
		return selector, nil
	}
	if _, err := s.captureSnapshotElements(ctx, tab); err != nil {
		return "", err
	}
	if selector := tab.selectorForRef(ref); selector != "" {
		return selector, nil
	}
	return "", fmt.Errorf("unknown element ref %q; run browser_snapshot again", ref)
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

func (t *tab) selectorForRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, el := range t.elements {
		if strings.TrimSpace(el.Ref) == ref {
			return strings.TrimSpace(el.Selector)
		}
	}
	return ""
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

func (t *tab) withTargetContext(parent context.Context, fn func(context.Context) error) error {
	t.opMu.Lock()
	defer t.opMu.Unlock()
	opCtx, cleanup := t.operationContext(parent)
	defer cleanup()
	return fn(opCtx)
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
