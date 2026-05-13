package browser

import (
	"context"
	"encoding/base64"
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
	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	cdruntime "github.com/chromedp/cdproto/runtime"
	cdptarget "github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

var (
	ErrRuntimeClosed    = errors.New("builtin browser runtime is closed")
	ErrNoLoadedPage     = errors.New("builtin browser session has no loaded page")
	titlePattern        = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	tagPattern          = regexp.MustCompile(`(?s)<[^>]+>`)
	defaultOpTimeout    = 20 * time.Second
	DefaultCapabilities = []string{
		"navigate", "snapshot", "inspect", "tabs", "back", "forward", "reload",
		"click", "hover", "type", "press_key", "select", "wait", "scroll",
		"screenshot", "console", "network", "viewport", "visibility",
		"clipboard", "dev_logs", "user_tabs", "locator", "cua", "dom_cua",
		"downloads", "session_name",
	}
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
	Level       int    `json:"level"`
}

type InspectRequest struct {
	Ref      string
	Selector string
}

type InspectDetails struct {
	Element     SnapshotElement    `json:"element"`
	States      map[string]bool    `json:"states"`
	Attributes  map[string]string  `json:"attributes"`
	Bounds      map[string]float64 `json:"bounds"`
	AX          map[string]string  `json:"ax"`
	Ancestors   []string           `json:"ancestors,omitempty"`
	Diagnostics map[string]any     `json:"diagnostics,omitempty"`
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
	Path     string
	FullPage bool
}

type ConsoleRequest struct {
	Limit int
}

type NetworkRequest struct {
	Limit int
}

type ReloadRequest struct{}

type ViewportRequest struct {
	Action string
	Width  int
	Height int
}

type ViewportInfo struct {
	Width  int  `json:"width"`
	Height int  `json:"height"`
	Custom bool `json:"custom"`
}

type VisibilityRequest struct {
	Action  string
	Visible bool
}

type ClipboardRequest struct {
	Action string
	Text   string
}

type CUARequest struct {
	Action  string
	X       int
	Y       int
	ScrollX int
	ScrollY int
	Text    string
	Keys    []string
	Button  string
	Path    []map[string]int
}

type DOMCUARequest struct {
	Action string
	NodeID string
	X      int
	Y      int
	Text   string
	Keys   []string
}

type LocatorRequest struct {
	Action    string
	Selector  string
	Text      string
	Attribute string
	Value     string
	State     string
	Timeout   int
	Checked   bool
}

type DownloadsRequest struct {
	Limit int
}

type UserTabsRequest struct{}

type SessionNameRequest struct {
	Name string
}

type ActionResult struct {
	Message string
	Data    map[string]interface{}
}

type SessionBackend interface {
	Capabilities() []string
	Navigate(context.Context, NavigateRequest) (ActionResult, error)
	Snapshot(context.Context, SnapshotRequest) (ActionResult, error)
	Inspect(context.Context, InspectRequest) (ActionResult, error)
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
	Reload(context.Context, ReloadRequest) (ActionResult, error)
	Viewport(context.Context, ViewportRequest) (ActionResult, error)
	Visibility(context.Context, VisibilityRequest) (ActionResult, error)
	Clipboard(context.Context, ClipboardRequest) (ActionResult, error)
	CUA(context.Context, CUARequest) (ActionResult, error)
	DOMCUA(context.Context, DOMCUARequest) (ActionResult, error)
	Locator(context.Context, LocatorRequest) (ActionResult, error)
	DevLogs(context.Context, ConsoleRequest) (ActionResult, error)
	Downloads(context.Context, DownloadsRequest) (ActionResult, error)
	UserTabs(context.Context, UserTabsRequest) (ActionResult, error)
	SetSessionName(context.Context, SessionNameRequest) (ActionResult, error)
}

type RuntimeStatus struct {
	Ready        bool
	LastError    string
	Capabilities []string
	Backend      string
	ExecPath     string
	Visible      bool
	Viewport     ViewportInfo
}

type SessionSnapshot struct {
	TraceID   string    `json:"trace_id"`
	Name      string    `json:"name,omitempty"`
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
	visible   bool
	viewport  ViewportInfo
	launchOK  bool
	checked   bool
}

type session struct {
	runtime *BuiltinRuntime
	traceID string
	name    string

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
	downloads   []string
}

func NewBuiltinRuntime() *BuiltinRuntime {
	execPath := findBrowserBinary()
	rt := &BuiltinRuntime{
		sessions: make(map[string]*session),
		execPath: execPath,
		viewport: ViewportInfo{Width: 1440, Height: 1024, Custom: false},
	}
	if execPath == "" {
		rt.lastError = "no supported Chrome/Chromium executable found"
	}
	return rt
}

func (r *BuiltinRuntime) viewportOrDefaultLocked() ViewportInfo {
	if r.viewport.Width <= 0 || r.viewport.Height <= 0 {
		return ViewportInfo{Width: 1440, Height: 1024, Custom: false}
	}
	return r.viewport
}

func (r *BuiltinRuntime) Status() RuntimeStatus {
	r.ensureLaunchChecked()
	r.mu.RLock()
	defer r.mu.RUnlock()
	ready := !r.closed && strings.TrimSpace(r.execPath) != "" && r.launchOK
	return RuntimeStatus{
		Ready:        ready,
		LastError:    strings.TrimSpace(r.lastError),
		Capabilities: append([]string(nil), DefaultCapabilities...),
		Backend:      "builtin_headless",
		ExecPath:     strings.TrimSpace(r.execPath),
		Visible:      r.visible,
		Viewport:     r.viewportOrDefaultLocked(),
	}
}

func (r *BuiltinRuntime) ensureLaunchChecked() {
	r.mu.RLock()
	checked := r.checked
	execPath := strings.TrimSpace(r.execPath)
	closed := r.closed
	r.mu.RUnlock()
	if checked || closed {
		return
	}
	ok, err := probeChromedp(execPath)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.checked {
		return
	}
	r.checked = true
	r.launchOK = ok
	if err != nil {
		r.lastError = strings.TrimSpace(err.Error())
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
	r.ensureLaunchChecked()
	r.mu.Lock()
	if r.closed {
		r.lastError = ErrRuntimeClosed.Error()
		r.mu.Unlock()
		return nil, ErrRuntimeClosed
	}
	if !r.launchOK {
		err := fmt.Errorf("builtin browser runtime unavailable: %s", strings.TrimSpace(r.lastError))
		r.mu.Unlock()
		return nil, err
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
			"outline":       snapshotOutline(elements),
		},
	}, nil
}

func (s *session) Inspect(ctx context.Context, req InspectRequest) (ActionResult, error) {
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	selector := strings.TrimSpace(req.Selector)
	ref := strings.TrimSpace(req.Ref)
	if selector == "" && ref != "" {
		selector, err = s.resolveActionSelector(ctx, tab, ref)
		if err != nil {
			return ActionResult{}, err
		}
	}
	if selector == "" {
		return ActionResult{}, fmt.Errorf("missing selector or ref")
	}
	if _, err := s.captureSnapshotElements(ctx, tab); err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	var element SnapshotElement
	if ref != "" {
		if cached, ok := tab.elementByRef(ref); ok {
			element = cached
		}
	}
	if element.Ref == "" {
		if matched, ok := tab.elementBySelector(selector); ok {
			element = matched
			ref = matched.Ref
		}
	}
	details, err := s.inspectDetails(ctx, tab, ref, selector, element)
	if err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	return ActionResult{
		Message: formatInspectMessage(details),
		Data: map[string]interface{}{
			"ref":      details.Element.Ref,
			"selector": selector,
			"detail":   details,
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
	url, err := s.navigateHistory(ctx, tab, -1)
	if err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	title := tab.currentTitle()
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
	url, err := s.navigateHistory(ctx, tab, 1)
	if err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	title := tab.currentTitle()
	return ActionResult{Message: fmt.Sprintf("navigated %s forward to %s (title=%q)", tab.id, url, title)}, nil
}

func (s *session) navigateHistory(ctx context.Context, tab *tab, delta int64) (string, error) {
	startURL := tab.currentURL()
	call := "history.back()"
	if delta > 0 {
		call = "history.forward()"
	}
	if err := tab.run(ctx, chromedp.Evaluate(call, nil)); err != nil {
		return "", fmt.Errorf("history %d: %w", delta, err)
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(200 * time.Millisecond):
	}
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var lastErr error
	var lastURL string
	for {
		var url string
		err := tab.run(waitCtx, chromedp.Location(&url))
		url = strings.TrimSpace(url)
		lastURL = url
		if err == nil && strings.TrimSpace(url) != "" && strings.TrimSpace(url) != strings.TrimSpace(startURL) {
			select {
			case <-waitCtx.Done():
				return url, waitCtx.Err()
			case <-time.After(200 * time.Millisecond):
			}
			tab.mu.Lock()
			tab.lastURL = url
			tab.updatedAt = time.Now()
			tab.mu.Unlock()
			_ = s.reattachTab(tab)
			return url, nil
		}
		if err != nil {
			lastErr = fmt.Errorf("read location: %w", err)
		}
		select {
		case <-waitCtx.Done():
			if lastErr != nil {
				return lastURL, fmt.Errorf("wait for history change from %s (last %s): %w", startURL, lastURL, lastErr)
			}
			return lastURL, fmt.Errorf("wait for history change from %s (last %s): %w", startURL, lastURL, waitCtx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (s *session) reattachTab(tab *tab) error {
	if tab == nil {
		return ErrNoLoadedPage
	}
	chromeCtx := chromedp.FromContext(tab.ctx)
	if chromeCtx == nil || chromeCtx.Target == nil || chromeCtx.Target.TargetID == "" {
		return nil
	}
	tabCtx, cancel := chromedp.NewContext(s.ctx, chromedp.WithTargetID(chromeCtx.Target.TargetID))
	if err := chromedp.Run(tabCtx, network.Enable(), cdruntime.Enable(), page.Enable()); err != nil {
		cancel()
		return err
	}
	tab.ctx = tabCtx
	tab.cancel = cancel
	s.attachListeners(tab)
	return nil
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
		chromedp.Evaluate(fillScript(selector, req.Text), nil),
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
	if keys == "" {
		return ActionResult{}, fmt.Errorf("missing keys")
	}
	selector := strings.TrimSpace(req.Selector)
	if selector == "" && strings.TrimSpace(req.Ref) != "" {
		selector, err = s.resolveActionSelector(ctx, tab, req.Ref)
		if err != nil {
			return ActionResult{}, err
		}
	}
	keyEvent := normalizeKeyEvent(keys)
	if selector != "" {
		err = tab.run(ctx,
			chromedp.WaitVisible(selector, chromedp.ByQuery),
			chromedp.Focus(selector, chromedp.ByQuery),
			chromedp.SendKeys(selector, keyEvent, chromedp.ByQuery),
		)
	} else {
		err = tab.run(ctx, chromedp.KeyEvent(keyEvent))
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
	if !tab.hasPage() {
		return ActionResult{}, ErrNoLoadedPage
	}
	var buf []byte
	if req.FullPage || path != "" {
		err = tab.run(ctx, chromedp.FullScreenshot(&buf, 90))
	} else {
		err = tab.run(ctx, chromedp.CaptureScreenshot(&buf))
	}
	if err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	data := map[string]interface{}{
		"mime":       "image/png",
		"bytes":      len(buf),
		"png_base64": base64.StdEncoding.EncodeToString(buf),
		"full_page":  req.FullPage || path != "",
	}
	if path == "" {
		return ActionResult{Message: fmt.Sprintf("captured screenshot for %s (%d bytes)", tab.id, len(buf)), Data: data}, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ActionResult{}, err
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return ActionResult{}, err
	}
	data["path"] = path
	return ActionResult{Message: fmt.Sprintf("saved screenshot for %s to %s", tab.id, path), Data: data}, nil
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

func (s *session) Reload(ctx context.Context, req ReloadRequest) (ActionResult, error) {
	_ = req
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	if !tab.hasPage() {
		return ActionResult{}, ErrNoLoadedPage
	}
	if err := tab.run(ctx, chromedp.Reload(), chromedp.WaitReady("html", chromedp.ByQuery)); err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	url, title, _, err := s.refreshTabPageState(ctx, tab)
	if err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	return ActionResult{Message: fmt.Sprintf("reloaded %s at %s (title=%q)", tab.id, url, title)}, nil
}

func (s *session) Viewport(ctx context.Context, req ViewportRequest) (ActionResult, error) {
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "get"
	}
	s.runtime.mu.Lock()
	current := s.runtime.viewportOrDefaultLocked()
	s.runtime.mu.Unlock()
	switch action {
	case "get":
	case "reset":
		current = ViewportInfo{Width: 1440, Height: 1024, Custom: false}
		s.runtime.mu.Lock()
		s.runtime.viewport = current
		s.runtime.mu.Unlock()
		if err := tab.run(ctx, chromedp.EmulateViewport(int64(current.Width), int64(current.Height))); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
	case "set":
		if req.Width <= 0 || req.Height <= 0 {
			return ActionResult{}, fmt.Errorf("viewport width and height must be positive")
		}
		current = ViewportInfo{Width: req.Width, Height: req.Height, Custom: true}
		s.runtime.mu.Lock()
		s.runtime.viewport = current
		s.runtime.mu.Unlock()
		if err := tab.run(ctx, chromedp.EmulateViewport(int64(req.Width), int64(req.Height))); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
	default:
		return ActionResult{}, fmt.Errorf("unknown viewport action %q", req.Action)
	}
	return ActionResult{
		Message: fmt.Sprintf("viewport %s: %dx%d custom=%t", action, current.Width, current.Height, current.Custom),
		Data: map[string]interface{}{
			"viewport": current,
		},
	}, nil
}

func (s *session) Visibility(ctx context.Context, req VisibilityRequest) (ActionResult, error) {
	_ = ctx
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "get"
	}
	s.runtime.mu.Lock()
	switch action {
	case "get":
	case "set", "show", "hide":
		switch action {
		case "show":
			s.runtime.visible = true
		case "hide":
			s.runtime.visible = false
		default:
			s.runtime.visible = req.Visible
		}
	default:
		s.runtime.mu.Unlock()
		return ActionResult{}, fmt.Errorf("unknown visibility action %q", req.Action)
	}
	visible := s.runtime.visible
	s.runtime.mu.Unlock()
	return ActionResult{
		Message: fmt.Sprintf("browser visibility=%t (builtin_headless records state only)", visible),
		Data:    map[string]interface{}{"visible": visible, "backend": "builtin_headless"},
	}, nil
}

func (s *session) Clipboard(ctx context.Context, req ClipboardRequest) (ActionResult, error) {
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "read_text"
	}
	switch action {
	case "read", "read_text":
		var text string
		if err := tab.run(ctx, chromedp.Evaluate(`navigator.clipboard && navigator.clipboard.readText ? navigator.clipboard.readText().catch(() => "") : ""`, &text)); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		return ActionResult{Message: text, Data: map[string]interface{}{"text": text}}, nil
	case "write", "write_text":
		textJSON, _ := json.Marshal(req.Text)
		script := fmt.Sprintf(`navigator.clipboard && navigator.clipboard.writeText ? navigator.clipboard.writeText(%s) : Promise.reject(new Error("clipboard API unavailable"))`, string(textJSON))
		if err := tab.run(ctx, chromedp.Evaluate(script, nil)); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		return ActionResult{Message: fmt.Sprintf("wrote %d clipboard characters", len(req.Text)), Data: map[string]interface{}{"text": req.Text}}, nil
	default:
		return ActionResult{}, fmt.Errorf("unknown clipboard action %q", req.Action)
	}
}

func (s *session) CUA(ctx context.Context, req CUARequest) (ActionResult, error) {
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	switch action {
	case "click", "double_click":
		count := 1
		if action == "double_click" {
			count = 2
		}
		if err := tab.run(ctx, chromedp.MouseClickXY(float64(req.X), float64(req.Y), chromedp.ClickCount(count))); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
	case "move":
		if err := tab.run(ctx, chromedp.ActionFunc(func(opCtx context.Context) error {
			return input.DispatchMouseEvent(input.MouseMoved, float64(req.X), float64(req.Y)).Do(opCtx)
		})); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
	case "scroll":
		script := fmt.Sprintf(`window.scrollBy(%d, %d)`, req.ScrollX, req.ScrollY)
		if err := tab.run(ctx, chromedp.Evaluate(script, nil)); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
	case "type":
		if req.Text == "" {
			return ActionResult{}, fmt.Errorf("missing text")
		}
		if err := tab.run(ctx, chromedp.SendKeys("body", req.Text, chromedp.ByQuery)); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
	case "keypress":
		keys := strings.Join(req.Keys, "")
		if keys == "" {
			return ActionResult{}, fmt.Errorf("missing keys")
		}
		if err := tab.run(ctx, chromedp.KeyEvent(normalizeKeyEvent(keys))); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
	default:
		return ActionResult{}, fmt.Errorf("unknown cua action %q", req.Action)
	}
	_, _, _, _ = s.refreshTabPageState(ctx, tab)
	return ActionResult{Message: fmt.Sprintf("cua %s completed on %s", action, tab.id)}, nil
}

func (s *session) DOMCUA(ctx context.Context, req DOMCUARequest) (ActionResult, error) {
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "get_visible_dom" || action == "snapshot" || action == "" {
		elements, err := s.captureSnapshotElements(ctx, tab)
		if err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		return ActionResult{
			Message: strings.Join(snapshotOutline(elements), "\n"),
			Data:    map[string]interface{}{"nodes": elements},
		}, nil
	}
	selector, err := s.resolveActionSelector(ctx, tab, req.NodeID)
	if err != nil {
		return ActionResult{}, err
	}
	switch action {
	case "click", "double_click":
		if action == "double_click" {
			if err := tab.run(ctx, chromedp.Evaluate(fmt.Sprintf(`document.querySelector(%s).dispatchEvent(new MouseEvent("dblclick", {bubbles: true}))`, mustJSON(selector)), nil)); err != nil {
				s.runtime.setLastError(err)
				return ActionResult{}, err
			}
		} else if err := tab.run(ctx, chromedp.Click(selector, chromedp.ByQuery)); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
	case "type":
		if err := tab.run(ctx, chromedp.SendKeys(selector, req.Text, chromedp.ByQuery)); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
	case "keypress":
		keys := strings.Join(req.Keys, "")
		if keys == "" {
			return ActionResult{}, fmt.Errorf("missing keys")
		}
		if err := tab.run(ctx, chromedp.Focus(selector, chromedp.ByQuery), chromedp.KeyEvent(normalizeKeyEvent(keys))); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
	case "scroll":
		return s.Scroll(ctx, ScrollRequest{Selector: selector, X: req.X, Y: req.Y})
	default:
		return ActionResult{}, fmt.Errorf("unknown dom_cua action %q", req.Action)
	}
	_, _, _, _ = s.refreshTabPageState(ctx, tab)
	return ActionResult{Message: fmt.Sprintf("dom_cua %s completed on %s", action, req.NodeID)}, nil
}

func (s *session) Locator(ctx context.Context, req LocatorRequest) (ActionResult, error) {
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	selector := strings.TrimSpace(req.Selector)
	if selector == "" {
		return ActionResult{}, fmt.Errorf("missing selector")
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "count"
	}
	switch action {
	case "count":
		count, err := s.locatorCount(ctx, tab, selector)
		if err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		return ActionResult{Message: fmt.Sprintf("%s matched %d element(s)", selector, count), Data: map[string]interface{}{"count": count}}, nil
	case "click":
		if err := tab.run(ctx, chromedp.Click(selector, chromedp.ByQuery)); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
	case "fill":
		if err := tab.run(ctx, chromedp.Evaluate(fillScript(selector, req.Text), nil)); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
	case "type":
		if err := tab.run(ctx, chromedp.SendKeys(selector, req.Text, chromedp.ByQuery)); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
	case "press":
		if req.Text == "" {
			return ActionResult{}, fmt.Errorf("missing text")
		}
		if err := tab.run(ctx, chromedp.Focus(selector, chromedp.ByQuery), chromedp.KeyEvent(normalizeKeyEvent(req.Text))); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
	case "select":
		selected, err := s.selectValues(ctx, tab, selector, []string{req.Value})
		if err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		return ActionResult{Message: fmt.Sprintf("selected %s on %s", strings.Join(selected, ","), selector)}, nil
	case "set_checked", "check", "uncheck":
		checked := req.Checked || action == "check"
		if action == "uncheck" {
			checked = false
		}
		script := fmt.Sprintf(`(() => { const el = document.querySelector(%s); if (!el) throw new Error("selector not found"); el.checked = %t; el.dispatchEvent(new Event("input", {bubbles:true})); el.dispatchEvent(new Event("change", {bubbles:true})); return !!el.checked; })()`, mustJSON(selector), checked)
		var actual bool
		if err := tab.run(ctx, chromedp.Evaluate(script, &actual)); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		return ActionResult{Message: fmt.Sprintf("%s checked=%t", selector, actual), Data: map[string]interface{}{"checked": actual}}, nil
	case "text", "inner_text":
		var text string
		script := fmt.Sprintf(`(() => { const el = document.querySelector(%s); return el ? String(el.innerText || el.textContent || "") : ""; })()`, mustJSON(selector))
		if err := tab.run(ctx, chromedp.Evaluate(script, &text)); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		return ActionResult{Message: text, Data: map[string]interface{}{"text": text}}, nil
	case "attribute":
		var value any
		script := fmt.Sprintf(`(() => { const el = document.querySelector(%s); return el ? el.getAttribute(%s) : null; })()`, mustJSON(selector), mustJSON(req.Attribute))
		if err := tab.run(ctx, chromedp.Evaluate(script, &value)); err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		return ActionResult{Message: fmt.Sprintf("%s=%v", req.Attribute, value), Data: map[string]interface{}{"attribute": req.Attribute, "value": value}}, nil
	case "is_visible", "is_enabled", "state":
		state := strings.TrimSpace(req.State)
		if state == "" {
			state = strings.TrimPrefix(action, "is_")
		}
		value, err := s.locatorState(ctx, tab, selector, state)
		if err != nil {
			s.runtime.setLastError(err)
			return ActionResult{}, err
		}
		return ActionResult{Message: fmt.Sprintf("%s %s=%t", selector, state, value), Data: map[string]interface{}{"state": state, "value": value}}, nil
	case "wait":
		timeout := req.Timeout
		if timeout <= 0 {
			timeout = 1000
		}
		return s.Wait(ctx, WaitRequest{Selector: selector, Timeout: timeout})
	default:
		return ActionResult{}, fmt.Errorf("unknown locator action %q", req.Action)
	}
	_, _, _, _ = s.refreshTabPageState(ctx, tab)
	return ActionResult{Message: fmt.Sprintf("locator %s completed on %s", action, selector)}, nil
}

func (s *session) DevLogs(ctx context.Context, req ConsoleRequest) (ActionResult, error) {
	return s.Console(ctx, req)
}

func (s *session) Downloads(ctx context.Context, req DownloadsRequest) (ActionResult, error) {
	_ = ctx
	tab, err := s.mustActiveTab()
	if err != nil {
		return ActionResult{}, err
	}
	tab.mu.RLock()
	defer tab.mu.RUnlock()
	if len(tab.downloads) == 0 {
		return ActionResult{Message: "no downloads recorded", Data: map[string]interface{}{"downloads": []string{}}}, nil
	}
	limit := req.Limit
	if limit <= 0 || limit > len(tab.downloads) {
		limit = len(tab.downloads)
	}
	items := append([]string(nil), tab.downloads[len(tab.downloads)-limit:]...)
	return ActionResult{Message: strings.Join(items, "\n"), Data: map[string]interface{}{"downloads": items}}, nil
}

func (s *session) UserTabs(ctx context.Context, req UserTabsRequest) (ActionResult, error) {
	_ = ctx
	_ = req
	tabs := s.tabInfos()
	return ActionResult{Message: formatTabSummary(tabs), Data: map[string]interface{}{"tabs": tabs}}, nil
}

func (s *session) SetSessionName(ctx context.Context, req SessionNameRequest) (ActionResult, error) {
	_ = ctx
	name := strings.TrimSpace(req.Name)
	s.mu.Lock()
	s.name = name
	s.updatedAt = time.Now()
	s.mu.Unlock()
	return ActionResult{Message: fmt.Sprintf("browser session named %q", name), Data: map[string]interface{}{"name": name}}, nil
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
		lines = append(lines, snapshotOutline(elements)...)
	}
	lines = append(lines, "snapshot="+summary)
	return strings.Join(lines, "\n")
}

func snapshotOutline(elements []SnapshotElement) []string {
	lines := make([]string, 0, len(elements))
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
		level := el.Level
		if level < 0 {
			level = 0
		}
		if level > 6 {
			level = 6
		}
		indent := strings.Repeat("  ", level)
		lines = append(lines, fmt.Sprintf("%s- [%s] %s %q {%s}", indent, el.Ref, role, label, source))
	}
	return lines
}

func formatInspectMessage(detail InspectDetails) string {
	el := detail.Element
	lines := []string{
		fmt.Sprintf("ref=%s", strings.TrimSpace(el.Ref)),
		fmt.Sprintf("role=%s", strings.TrimSpace(el.Role)),
		fmt.Sprintf("name=%s", strings.TrimSpace(el.Name)),
		fmt.Sprintf("selector=%s", strings.TrimSpace(el.Selector)),
		fmt.Sprintf("source=%s", strings.TrimSpace(el.Source)),
	}
	if strings.TrimSpace(el.Value) != "" {
		lines = append(lines, "value="+strings.TrimSpace(el.Value))
	}
	if strings.TrimSpace(el.Text) != "" {
		lines = append(lines, "text="+strings.TrimSpace(el.Text))
	}
	if len(detail.Ancestors) > 0 {
		lines = append(lines, "ancestors="+strings.Join(detail.Ancestors, " > "))
	}
	if len(detail.States) > 0 {
		stateParts := make([]string, 0, len(detail.States))
		for _, key := range []string{"visible", "disabled", "editable", "checked", "selected", "expanded"} {
			if value, ok := detail.States[key]; ok {
				stateParts = append(stateParts, fmt.Sprintf("%s=%t", key, value))
			}
		}
		if len(stateParts) > 0 {
			lines = append(lines, "states="+strings.Join(stateParts, ", "))
		}
	}
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
	ctx, cancelInit := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelInit()
	if err := chromedp.Run(browserCtx,
		network.Enable(),
		cdruntime.Enable(),
		page.Enable(),
		chromedp.Navigate("about:blank"),
	); err != nil {
		sess.Close()
		return nil, err
	}
	tab := &tab{
		id:          sess.nextTabName(),
		ctx:         browserCtx,
		cancel:      func() {},
		createdAt:   time.Now(),
		updatedAt:   time.Now(),
		activatedAt: time.Now(),
	}
	sess.attachListeners(tab)
	if _, _, _, err := sess.refreshTabPageState(ctx, tab); err != nil {
		sess.Close()
		return nil, err
	}
	sess.mu.Lock()
	sess.tabs = append(sess.tabs, tab)
	sess.activeTabID = tab.id
	sess.updatedAt = time.Now()
	sess.mu.Unlock()
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

func probeChromedp(execPath string) (bool, error) {
	execPath = strings.TrimSpace(execPath)
	if execPath == "" {
		return false, fmt.Errorf("builtin browser runtime unavailable: %s", noBrowserMessage())
	}
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocatorOptions(execPath)...)
	defer allocCancel()
	browserCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, timeoutCancel := context.WithTimeout(browserCtx, 8*time.Second)
	defer timeoutCancel()
	if err := chromedp.Run(ctx, chromedp.Navigate("about:blank")); err != nil {
		return false, fmt.Errorf("builtin browser launch probe failed for %s: %w", execPath, err)
	}
	return true, nil
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
	root := chromedp.FromContext(s.ctx)
	if root == nil || root.Browser == nil {
		return nil, fmt.Errorf("browser context is not initialized")
	}
	createCtx, createCancel := context.WithTimeout(ctx, defaultOpTimeout)
	defer createCancel()
	targetID, err := cdptarget.CreateTarget("about:blank").WithBackground(!activate).Do(cdp.WithExecutor(createCtx, root.Browser))
	if err != nil {
		return nil, fmt.Errorf("create target: %w", err)
	}
	tabCtx, cancel := chromedp.NewContext(s.ctx, chromedp.WithTargetID(targetID))
	tab := &tab{
		id:          s.nextTabName(),
		ctx:         tabCtx,
		cancel:      cancel,
		createdAt:   time.Now(),
		updatedAt:   time.Now(),
		activatedAt: time.Now(),
	}
	s.attachListeners(tab)
	if err := chromedp.Run(tabCtx,
		network.Enable(),
		cdruntime.Enable(),
		page.Enable(),
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("html", chromedp.ByQuery),
	); err != nil {
		cancel()
		return nil, fmt.Errorf("attach target: %w", err)
	}
	if _, _, _, err := s.refreshTabPageState(ctx, tab); err != nil {
		cancel()
		return nil, fmt.Errorf("refresh tab state: %w", err)
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
	name := s.name
	updatedAt := s.updatedAt
	activeTab := s.activeTabID
	s.mu.RUnlock()
	tabs := s.tabInfos()
	return SessionSnapshot{
		TraceID:   traceID,
		Name:      name,
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
	var state struct {
		URL   string `json:"url"`
		Title string `json:"title"`
		HTML  string `json:"html"`
	}
	if err := tab.run(ctx, chromedp.Evaluate(`(() => ({
		url: String(location.href || ""),
		title: String(document.title || ""),
		html: document.documentElement ? document.documentElement.outerHTML : ""
	}))()`, &state)); err != nil {
		return "", "", "", err
	}
	url = state.URL
	title = state.Title
	html = state.HTML
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
		const interesting = Array.from(document.querySelectorAll('a,button,input,select,textarea,summary,[id],[role],[contenteditable=""],[contenteditable="true"],[tabindex],[onclick],[onmouseover],[onmouseenter]'));
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
		levels := axNodeLevels(nodes)
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
				Level:       levels[node.NodeID],
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

func (s *session) inspectDetails(ctx context.Context, tab *tab, ref, selector string, cached SnapshotElement) (InspectDetails, error) {
	selectorJSON, _ := json.Marshal(selector)
	script := fmt.Sprintf(`(() => {
		const selector = %s;
		const el = document.querySelector(selector);
		if (!el) {
			throw new Error("selector not found: " + selector);
		}
		const rect = el.getBoundingClientRect();
		const normalize = (value) => String(value || '').replace(/\s+/g, ' ').trim();
		const visible = (() => {
			const style = window.getComputedStyle(el);
			return !!style && style.visibility !== 'hidden' && style.display !== 'none' && rect.width > 0 && rect.height > 0;
		})();
		const attrs = {};
		for (const name of ['id', 'class', 'role', 'type', 'name', 'placeholder', 'href', 'aria-label', 'aria-expanded', 'aria-controls']) {
			const value = el.getAttribute && el.getAttribute(name);
			if (value !== null && value !== undefined && String(value) !== '') attrs[name] = String(value);
		}
		const ancestors = [];
		let current = el.parentElement;
		while (current && ancestors.length < 6) {
			const label = normalize((current.getAttribute && (current.getAttribute('aria-label') || current.getAttribute('role'))) || current.tagName);
			if (label) ancestors.push(label);
			current = current.parentElement;
		}
		return {
			text: normalize(el.innerText || el.textContent),
			value: normalize(el.value),
			name: normalize((el.getAttribute && el.getAttribute('aria-label')) || ''),
			attributes: attrs,
			states: {
				visible,
				disabled: !!el.disabled || el.getAttribute('aria-disabled') === 'true',
				editable: !(el.readOnly) && (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || el.isContentEditable),
				checked: !!el.checked || el.getAttribute('aria-checked') === 'true',
				selected: !!el.selected || el.getAttribute('aria-selected') === 'true',
				expanded: el.getAttribute('aria-expanded') === 'true'
			},
			bounds: {x: rect.x, y: rect.y, width: rect.width, height: rect.height},
			ancestors
		};
	})()`, string(selectorJSON))
	var live struct {
		Text       string             `json:"text"`
		Value      string             `json:"value"`
		Name       string             `json:"name"`
		Attributes map[string]string  `json:"attributes"`
		States     map[string]bool    `json:"states"`
		Bounds     map[string]float64 `json:"bounds"`
		Ancestors  []string           `json:"ancestors"`
	}
	if err := tab.run(ctx, chromedp.Evaluate(script, &live)); err != nil {
		return InspectDetails{}, err
	}
	element := cached
	if strings.TrimSpace(element.Ref) == "" {
		element.Ref = strings.TrimSpace(ref)
	}
	if strings.TrimSpace(element.Selector) == "" {
		element.Selector = selector
	}
	if strings.TrimSpace(element.Text) == "" {
		element.Text = strings.TrimSpace(live.Text)
	}
	if strings.TrimSpace(element.Value) == "" {
		element.Value = strings.TrimSpace(live.Value)
	}
	if strings.TrimSpace(element.Name) == "" {
		element.Name = strings.TrimSpace(live.Name)
	}
	details := InspectDetails{
		Element:    element,
		States:     live.States,
		Attributes: live.Attributes,
		Bounds:     live.Bounds,
		Ancestors:  live.Ancestors,
		AX: map[string]string{
			"role":        strings.TrimSpace(element.Role),
			"name":        strings.TrimSpace(element.Name),
			"description": strings.TrimSpace(element.Description),
			"value":       strings.TrimSpace(element.Value),
			"source":      strings.TrimSpace(element.Source),
		},
		Diagnostics: map[string]any{
			"level": element.Level,
		},
	}
	return details, nil
}

func (s *session) locatorCount(ctx context.Context, tab *tab, selector string) (int, error) {
	script := fmt.Sprintf(`(() => document.querySelectorAll(%s).length)()`, mustJSON(selector))
	var count int
	if err := tab.run(ctx, chromedp.Evaluate(script, &count)); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *session) locatorState(ctx context.Context, tab *tab, selector, state string) (bool, error) {
	state = strings.ToLower(strings.TrimSpace(state))
	if state == "" {
		state = "visible"
	}
	script := fmt.Sprintf(`(() => {
		const el = document.querySelector(%s);
		if (!el) return false;
		const style = window.getComputedStyle(el);
		const rect = el.getBoundingClientRect();
		switch (%s) {
		case "visible":
			return !!style && style.visibility !== "hidden" && style.display !== "none" && rect.width > 0 && rect.height > 0;
		case "enabled":
			return !el.disabled && el.getAttribute("aria-disabled") !== "true";
		case "checked":
			return !!el.checked || el.getAttribute("aria-checked") === "true";
		case "editable":
			return !el.readOnly && (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || el.isContentEditable);
		case "attached":
			return true;
		default:
			return false;
		}
	})()`, mustJSON(selector), mustJSON(state))
	var value bool
	if err := tab.run(ctx, chromedp.Evaluate(script, &value)); err != nil {
		return false, err
	}
	return value, nil
}

func mustJSON(value string) string {
	b, _ := json.Marshal(value)
	return string(b)
}

func fillScript(selector, text string) string {
	return fmt.Sprintf(`(() => {
		const el = document.querySelector(%s);
		if (!el) throw new Error("selector not found: " + %s);
		el.focus && el.focus();
		if ("value" in el) {
			el.value = %s;
		} else {
			el.textContent = %s;
		}
		el.dispatchEvent(new Event("input", {bubbles: true}));
		el.dispatchEvent(new Event("change", {bubbles: true}));
		return true;
	})()`, mustJSON(selector), mustJSON(selector), mustJSON(text), mustJSON(text))
}

func normalizeKeyEvent(keys string) string {
	if keys == "" {
		return ""
	}
	if keys == "\n" || keys == "\r" {
		return kb.Enter
	}
	switch strings.ToLower(strings.TrimSpace(keys)) {
	case "", "enter", "return":
		return kb.Enter
	case "tab":
		return kb.Tab
	case "escape", "esc":
		return kb.Escape
	case "backspace":
		return kb.Backspace
	case "delete", "del":
		return kb.Delete
	case "arrowup", "up":
		return kb.ArrowUp
	case "arrowdown", "down":
		return kb.ArrowDown
	case "arrowleft", "left":
		return kb.ArrowLeft
	case "arrowright", "right":
		return kb.ArrowRight
	case "home":
		return kb.Home
	case "end":
		return kb.End
	case "pagedown":
		return kb.PageDown
	case "pageup":
		return kb.PageUp
	default:
		return keys
	}
}

func axNodeLevels(nodes []*accessibility.Node) map[accessibility.NodeID]int {
	levels := make(map[accessibility.NodeID]int, len(nodes))
	parents := make(map[accessibility.NodeID]accessibility.NodeID, len(nodes))
	for _, node := range nodes {
		if node != nil {
			parents[node.NodeID] = node.ParentID
		}
	}
	var levelOf func(accessibility.NodeID) int
	levelOf = func(id accessibility.NodeID) int {
		if level, ok := levels[id]; ok {
			return level
		}
		parent := parents[id]
		if parent == "" || parent == id {
			levels[id] = 0
			return 0
		}
		level := levelOf(parent) + 1
		levels[id] = level
		return level
	}
	for id := range parents {
		levelOf(id)
	}
	return levels
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

func (t *tab) elementByRef(ref string) (SnapshotElement, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return SnapshotElement{}, false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, el := range t.elements {
		if strings.TrimSpace(el.Ref) == ref {
			return el, true
		}
	}
	return SnapshotElement{}, false
}

func (t *tab) elementBySelector(selector string) (SnapshotElement, bool) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return SnapshotElement{}, false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, el := range t.elements {
		if strings.TrimSpace(el.Selector) == selector {
			return el, true
		}
	}
	return SnapshotElement{}, false
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
		case *cdpbrowser.EventDownloadWillBegin:
			tab.appendDownload(fmt.Sprintf("%s %s", strings.TrimSpace(evt.GUID), strings.TrimSpace(evt.URL)))
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

func (t *tab) currentTitle() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return strings.TrimSpace(t.lastTitle)
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
			opCtx, cancel = context.WithTimeout(t.ctx, defaultOpTimeout)
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

func (t *tab) appendDownload(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.downloads = appendBounded(t.downloads, line, 100)
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
			"msedge",
			"msedge.exe",
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
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
	return "install Chrome/Chromium/Edge or set CHROME_BIN/CHROMIUM_BIN/BROWSER_BIN"
}
