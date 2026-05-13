package browser

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	ErrRuntimeClosed     = errors.New("builtin browser runtime is closed")
	ErrNoLoadedPage      = errors.New("builtin browser session has no loaded page")
	ErrUnsupportedAction = errors.New("builtin browser runtime does not support this action yet")
	titlePattern         = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	tagPattern           = regexp.MustCompile(`(?s)<[^>]+>`)
	DefaultCapabilities  = []string{"navigate", "snapshot", "wait", "network"}
)

type NavigateRequest struct {
	URL string
}

type SnapshotRequest struct{}

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
}

type SessionBackend interface {
	Capabilities() []string
	Navigate(context.Context, NavigateRequest) (ActionResult, error)
	Snapshot(context.Context, SnapshotRequest) (ActionResult, error)
	Click(context.Context, ClickRequest) (ActionResult, error)
	Type(context.Context, TypeRequest) (ActionResult, error)
	Select(context.Context, SelectRequest) (ActionResult, error)
	Wait(context.Context, WaitRequest) (ActionResult, error)
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
	client    *http.Client
	sessions  map[string]*session
	lastError string
	closed    bool
}

type session struct {
	runtime *BuiltinRuntime
	traceID string

	mu        sync.RWMutex
	createdAt time.Time
	updatedAt time.Time
	lastURL   string
	lastTitle string
	lastHTML  string
	network   []string
}

func NewBuiltinRuntime() *BuiltinRuntime {
	return &BuiltinRuntime{
		client:   &http.Client{Timeout: 20 * time.Second},
		sessions: make(map[string]*session),
	}
}

func (r *BuiltinRuntime) Status() RuntimeStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return RuntimeStatus{
		Ready:        !r.closed && r.client != nil,
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
	defer r.mu.Unlock()
	if r.closed {
		r.lastError = ErrRuntimeClosed.Error()
		return nil, ErrRuntimeClosed
	}
	if sess, ok := r.sessions[traceID]; ok {
		return sess, nil
	}
	now := time.Now()
	sess := &session{
		runtime:   r,
		traceID:   traceID,
		createdAt: now,
		updatedAt: now,
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
	delete(r.sessions, traceID)
	r.mu.Unlock()
}

func (r *BuiltinRuntime) SessionCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}

func (r *BuiltinRuntime) Close() {
	r.mu.Lock()
	r.closed = true
	r.sessions = make(map[string]*session)
	r.mu.Unlock()
}

func (s *session) Capabilities() []string {
	return append([]string(nil), DefaultCapabilities...)
}

func (s *session) Navigate(ctx context.Context, req NavigateRequest) (ActionResult, error) {
	target := strings.TrimSpace(req.URL)
	if target == "" {
		return ActionResult{}, fmt.Errorf("missing url")
	}
	parsed, err := url.Parse(target)
	if err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		err = fmt.Errorf("url must include scheme and host: %s", target)
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	httpReq.Header.Set("User-Agent", "eos-builtin-browser/0.1")
	resp, err := s.runtime.client.Do(httpReq)
	if err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		s.runtime.setLastError(err)
		return ActionResult{}, err
	}
	title := extractHTMLTitle(string(body))

	s.mu.Lock()
	s.updatedAt = time.Now()
	s.lastURL = parsed.String()
	s.lastTitle = title
	s.lastHTML = string(body)
	s.network = append(s.network, fmt.Sprintf("%s %s -> %d", http.MethodGet, parsed.String(), resp.StatusCode))
	s.mu.Unlock()

	return ActionResult{Message: fmt.Sprintf("navigated to %s (status=%d, title=%q)", parsed.String(), resp.StatusCode, title)}, nil
}

func (s *session) Snapshot(ctx context.Context, req SnapshotRequest) (ActionResult, error) {
	_ = ctx
	_ = req
	s.mu.RLock()
	defer s.mu.RUnlock()
	if strings.TrimSpace(s.lastURL) == "" {
		return ActionResult{}, ErrNoLoadedPage
	}
	return ActionResult{
		Message: fmt.Sprintf("url=%s\ntitle=%s\nsnapshot=%s", s.lastURL, s.lastTitle, summarizeHTMLSnapshot(s.lastHTML)),
	}, nil
}

func (s *session) Click(ctx context.Context, req ClickRequest) (ActionResult, error) {
	_ = ctx
	_ = req
	return ActionResult{}, fmt.Errorf("%w: click requires a DOM-capable backend", ErrUnsupportedAction)
}

func (s *session) Type(ctx context.Context, req TypeRequest) (ActionResult, error) {
	_ = ctx
	_ = req
	return ActionResult{}, fmt.Errorf("%w: type requires a DOM-capable backend", ErrUnsupportedAction)
}

func (s *session) Select(ctx context.Context, req SelectRequest) (ActionResult, error) {
	_ = ctx
	_ = req
	return ActionResult{}, fmt.Errorf("%w: select requires a DOM-capable backend", ErrUnsupportedAction)
}

func (s *session) Wait(ctx context.Context, req WaitRequest) (ActionResult, error) {
	s.mu.RLock()
	hasPage := strings.TrimSpace(s.lastURL) != ""
	s.mu.RUnlock()
	if !hasPage {
		return ActionResult{}, ErrNoLoadedPage
	}
	timeout := time.Duration(req.Timeout) * time.Millisecond
	if timeout <= 0 {
		timeout = 250 * time.Millisecond
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ActionResult{}, ctx.Err()
	case <-timer.C:
	}
	if strings.TrimSpace(req.Selector) != "" {
		return ActionResult{Message: fmt.Sprintf("waited %s without DOM probing; selector=%s", timeout, strings.TrimSpace(req.Selector))}, nil
	}
	return ActionResult{Message: fmt.Sprintf("waited %s", timeout)}, nil
}

func (s *session) Screenshot(ctx context.Context, req ScreenshotRequest) (ActionResult, error) {
	_ = ctx
	path := strings.TrimSpace(req.Path)
	if path == "" {
		return ActionResult{}, fmt.Errorf("%w: screenshot path is required", ErrUnsupportedAction)
	}
	s.mu.RLock()
	html := s.lastHTML
	hasPage := strings.TrimSpace(s.lastURL) != ""
	s.mu.RUnlock()
	if !hasPage {
		return ActionResult{}, ErrNoLoadedPage
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ActionResult{}, err
	}
	if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Message: fmt.Sprintf("builtin minimal backend wrote HTML snapshot to %s instead of a raster screenshot", path)}, nil
}

func (s *session) Console(ctx context.Context, req ConsoleRequest) (ActionResult, error) {
	_ = ctx
	_ = req
	return ActionResult{}, fmt.Errorf("%w: console inspection requires script execution", ErrUnsupportedAction)
}

func (s *session) Network(ctx context.Context, req NetworkRequest) (ActionResult, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.network) == 0 {
		return ActionResult{}, ErrNoLoadedPage
	}
	limit := req.Limit
	if limit <= 0 || limit > len(s.network) {
		limit = len(s.network)
	}
	return ActionResult{Message: strings.Join(s.network[len(s.network)-limit:], "\n")}, nil
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
