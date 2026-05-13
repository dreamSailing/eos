package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/dreamSailing/eos/internal/browser"
	"github.com/dreamSailing/eos/internal/pkg/utils"
	"github.com/google/uuid"
)

func (m *Manager) browserNavigateStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	url, _ := params["url"].(string)
	return m.runBrowserAction(ctx, ToolBrowserNavigate, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Navigate(ctx, browser.NavigateRequest{URL: strings.TrimSpace(url)})
	})
}

func (m *Manager) browserSnapshotStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	return m.runBrowserAction(ctx, ToolBrowserSnapshot, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Snapshot(ctx, browser.SnapshotRequest{})
	})
}

func (m *Manager) browserTabsStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	action, _ := params["action"].(string)
	id, _ := params["id"].(string)
	url, _ := params["url"].(string)
	index, hasIndex := intParamWithPresence(params["index"])
	return m.runBrowserAction(ctx, ToolBrowserTabs, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Tabs(ctx, browser.TabsRequest{
			Action:   strings.TrimSpace(action),
			ID:       strings.TrimSpace(id),
			Index:    index,
			HasIndex: hasIndex,
			URL:      strings.TrimSpace(url),
		})
	})
}

func (m *Manager) browserBackStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	return m.runBrowserAction(ctx, ToolBrowserBack, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Back(ctx)
	})
}

func (m *Manager) browserForwardStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	return m.runBrowserAction(ctx, ToolBrowserForward, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Forward(ctx)
	})
}

func (m *Manager) browserClickStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	selector, _ := params["selector"].(string)
	return m.runBrowserAction(ctx, ToolBrowserClick, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Click(ctx, browser.ClickRequest{Selector: strings.TrimSpace(selector)})
	})
}

func (m *Manager) browserHoverStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	selector, _ := params["selector"].(string)
	return m.runBrowserAction(ctx, ToolBrowserHover, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Hover(ctx, browser.HoverRequest{Selector: strings.TrimSpace(selector)})
	})
}

func (m *Manager) browserTypeStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	selector, _ := params["selector"].(string)
	text, _ := params["text"].(string)
	return m.runBrowserAction(ctx, ToolBrowserType, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Type(ctx, browser.TypeRequest{Selector: strings.TrimSpace(selector), Text: text})
	})
}

func (m *Manager) browserPressKeyStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	selector, _ := params["selector"].(string)
	keys, _ := params["keys"].(string)
	return m.runBrowserAction(ctx, ToolBrowserPressKey, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.PressKey(ctx, browser.KeyRequest{Selector: strings.TrimSpace(selector), Keys: keys})
	})
}

func (m *Manager) browserSelectStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	selector, _ := params["selector"].(string)
	values := stringSliceFromParam(params["values"])
	return m.runBrowserAction(ctx, ToolBrowserSelect, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Select(ctx, browser.SelectRequest{Selector: strings.TrimSpace(selector), Values: values})
	})
}

func (m *Manager) browserWaitStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	selector, _ := params["selector"].(string)
	timeout := intParam(params["timeout"])
	return m.runBrowserAction(ctx, ToolBrowserWait, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Wait(ctx, browser.WaitRequest{Selector: strings.TrimSpace(selector), Timeout: timeout})
	})
}

func (m *Manager) browserScrollStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	selector, _ := params["selector"].(string)
	x := intParam(params["x"])
	y := intParam(params["y"])
	return m.runBrowserAction(ctx, ToolBrowserScroll, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Scroll(ctx, browser.ScrollRequest{Selector: strings.TrimSpace(selector), X: x, Y: y})
	})
}

func (m *Manager) browserScreenshotStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	path, _ := params["path"].(string)
	resolved := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), path)
	if resolved.ErrMsg != "" || !resolved.IsValid {
		errMsg := strings.TrimSpace(resolved.ErrMsg)
		if errMsg == "" {
			errMsg = "invalid screenshot path"
		}
		return ToolResult{Type: "tool_result", Tool: ToolBrowserScreenshot, Status: "error", Error: errMsg}
	}
	return m.runBrowserAction(ctx, ToolBrowserScreenshot, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Screenshot(ctx, browser.ScreenshotRequest{Path: resolved.AbsPath})
	})
}

func (m *Manager) browserConsoleStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	limit := intParam(params["limit"])
	return m.runBrowserAction(ctx, ToolBrowserConsole, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Console(ctx, browser.ConsoleRequest{Limit: limit})
	})
}

func (m *Manager) browserNetworkStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	limit := intParam(params["limit"])
	return m.runBrowserAction(ctx, ToolBrowserNetwork, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Network(ctx, browser.NetworkRequest{Limit: limit})
	})
}

func (m *Manager) runBrowserAction(ctx context.Context, toolName string, params map[string]interface{}, fn func(browser.SessionBackend) (browser.ActionResult, error)) ToolResult {
	sess, traceID, cleanup, err := m.browserSession(ctx)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    toolName,
			Status:  "error",
			Error:   err.Error(),
			Display: err.Error(),
		}
	}
	out, err := fn(sess)
	if err != nil {
		return ToolResult{
			Type:   "tool_result",
			Tool:   toolName,
			Status: "error",
			Error:  err.Error(),
			Data: map[string]interface{}{
				"trace_id": traceID,
			},
			Display: err.Error(),
		}
	}
	data := map[string]interface{}{
		"trace_id": traceID,
		"message":  out.Message,
		"params":   params,
	}
	for k, v := range out.Data {
		data[k] = v
	}
	return ToolResult{
		Type:    "tool_result",
		Tool:    toolName,
		Status:  "success",
		Data:    data,
		Display: out.Message,
	}
}

func (m *Manager) browserSession(ctx context.Context) (browser.SessionBackend, string, func(), error) {
	if m.browserRT == nil {
		return nil, "", nil, fmt.Errorf("builtin browser runtime unavailable")
	}
	traceID := strings.TrimSpace(TraceIDFromContext(ctx))
	ephemeral := false
	if traceID == "" {
		traceID = "browser-" + uuid.NewString()[:8]
		ephemeral = true
	}
	m.browserRT.StartTrace(traceID)
	sess, err := m.browserRT.Session(traceID)
	if err != nil {
		return nil, traceID, nil, err
	}
	cleanup := func() {}
	if ephemeral {
		cleanup = func() { m.browserRT.ReleaseTrace(traceID) }
	}
	return sess, traceID, cleanup, nil
}

func stringSliceFromParam(raw interface{}) []string {
	switch v := raw.(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	default:
		return nil
	}
}

func intParam(raw interface{}) int {
	switch v := raw.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func intParamWithPresence(raw interface{}) (int, bool) {
	if raw == nil {
		return 0, false
	}
	switch v := raw.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}
