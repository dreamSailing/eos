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

func (m *Manager) browserInspectStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	ref, _ := params["ref"].(string)
	selector, _ := params["selector"].(string)
	return m.runBrowserAction(ctx, ToolBrowserInspect, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Inspect(ctx, browser.InspectRequest{Ref: strings.TrimSpace(ref), Selector: strings.TrimSpace(selector)})
	})
}

func (m *Manager) browserTabsStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	action, _ := params["action"].(string)
	id, _ := params["id"].(string)
	query, _ := params["match"].(string)
	url, _ := params["url"].(string)
	index, hasIndex := intParamWithPresence(params["index"])
	activate, hasActivate := boolParamWithPresence(params["activate"])
	return m.runBrowserAction(ctx, ToolBrowserTabs, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Tabs(ctx, browser.TabsRequest{
			Action:      strings.TrimSpace(action),
			ID:          strings.TrimSpace(id),
			Index:       index,
			HasIndex:    hasIndex,
			Query:       strings.TrimSpace(query),
			URL:         strings.TrimSpace(url),
			Activate:    activate,
			HasActivate: hasActivate,
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
	ref, _ := params["ref"].(string)
	return m.runBrowserAction(ctx, ToolBrowserClick, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Click(ctx, browser.ClickRequest{Selector: strings.TrimSpace(selector), Ref: strings.TrimSpace(ref)})
	})
}

func (m *Manager) browserHoverStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	selector, _ := params["selector"].(string)
	ref, _ := params["ref"].(string)
	return m.runBrowserAction(ctx, ToolBrowserHover, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Hover(ctx, browser.HoverRequest{Selector: strings.TrimSpace(selector), Ref: strings.TrimSpace(ref)})
	})
}

func (m *Manager) browserTypeStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	selector, _ := params["selector"].(string)
	ref, _ := params["ref"].(string)
	text, _ := params["text"].(string)
	return m.runBrowserAction(ctx, ToolBrowserType, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Type(ctx, browser.TypeRequest{Selector: strings.TrimSpace(selector), Ref: strings.TrimSpace(ref), Text: text})
	})
}

func (m *Manager) browserPressKeyStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	selector, _ := params["selector"].(string)
	ref, _ := params["ref"].(string)
	keys, _ := params["keys"].(string)
	return m.runBrowserAction(ctx, ToolBrowserPressKey, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.PressKey(ctx, browser.KeyRequest{Selector: strings.TrimSpace(selector), Ref: strings.TrimSpace(ref), Keys: keys})
	})
}

func (m *Manager) browserSelectStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	selector, _ := params["selector"].(string)
	ref, _ := params["ref"].(string)
	values := stringSliceFromParam(params["values"])
	return m.runBrowserAction(ctx, ToolBrowserSelect, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Select(ctx, browser.SelectRequest{Selector: strings.TrimSpace(selector), Ref: strings.TrimSpace(ref), Values: values})
	})
}

func (m *Manager) browserWaitStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	selector, _ := params["selector"].(string)
	ref, _ := params["ref"].(string)
	timeout := intParam(params["timeout"])
	return m.runBrowserAction(ctx, ToolBrowserWait, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Wait(ctx, browser.WaitRequest{Selector: strings.TrimSpace(selector), Ref: strings.TrimSpace(ref), Timeout: timeout})
	})
}

func (m *Manager) browserScrollStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	selector, _ := params["selector"].(string)
	ref, _ := params["ref"].(string)
	x := intParam(params["x"])
	y := intParam(params["y"])
	return m.runBrowserAction(ctx, ToolBrowserScroll, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Scroll(ctx, browser.ScrollRequest{Selector: strings.TrimSpace(selector), Ref: strings.TrimSpace(ref), X: x, Y: y})
	})
}

func (m *Manager) browserScreenshotStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	path, _ := params["path"].(string)
	fullPage, _ := params["full_page"].(bool)
	resolvedPath := ""
	if strings.TrimSpace(path) != "" {
		resolved := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), path)
		if resolved.ErrMsg != "" || !resolved.IsValid {
			errMsg := strings.TrimSpace(resolved.ErrMsg)
			if errMsg == "" {
				errMsg = "invalid screenshot path"
			}
			return ToolResult{Type: "tool_result", Tool: ToolBrowserScreenshot, Status: "error", Error: errMsg}
		}
		resolvedPath = resolved.AbsPath
		if err := sandboxWriteError(ctx, resolvedPath); err != nil {
			return ToolResult{Type: "tool_result", Tool: ToolBrowserScreenshot, Status: "error", Error: err.Error(), Display: "错误：" + err.Error()}
		}
	}
	return m.runBrowserAction(ctx, ToolBrowserScreenshot, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Screenshot(ctx, browser.ScreenshotRequest{Path: resolvedPath, FullPage: fullPage})
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

func (m *Manager) browserReloadStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	return m.runBrowserAction(ctx, ToolBrowserReload, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Reload(ctx, browser.ReloadRequest{})
	})
}

func (m *Manager) browserViewportStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	action, _ := params["action"].(string)
	if reset, _ := params["reset"].(bool); reset {
		action = "reset"
	}
	width := intParam(params["width"])
	height := intParam(params["height"])
	return m.runBrowserAction(ctx, ToolBrowserViewport, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Viewport(ctx, browser.ViewportRequest{Action: strings.TrimSpace(action), Width: width, Height: height})
	})
}

func (m *Manager) browserVisibilityStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	action, _ := params["action"].(string)
	visible, hasVisible := boolParamWithPresence(params["visible"])
	if strings.TrimSpace(action) == "" && hasVisible {
		if visible {
			action = "show"
		} else {
			action = "hide"
		}
	}
	return m.runBrowserAction(ctx, ToolBrowserVisibility, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Visibility(ctx, browser.VisibilityRequest{Action: strings.TrimSpace(action), Visible: visible})
	})
}

func (m *Manager) browserClipboardStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	action, _ := params["action"].(string)
	text, _ := params["text"].(string)
	return m.runBrowserAction(ctx, ToolBrowserClipboard, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Clipboard(ctx, browser.ClipboardRequest{Action: strings.TrimSpace(action), Text: text})
	})
}

func (m *Manager) browserCUAStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	action, _ := params["action"].(string)
	text, _ := params["text"].(string)
	button, _ := params["button"].(string)
	return m.runBrowserAction(ctx, ToolBrowserCUA, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.CUA(ctx, browser.CUARequest{
			Action:  strings.TrimSpace(action),
			X:       intParam(params["x"]),
			Y:       intParam(params["y"]),
			ScrollX: intParam(params["scroll_x"]),
			ScrollY: intParam(params["scroll_y"]),
			Text:    text,
			Keys:    keysFromParam(params["keys"]),
			Button:  strings.TrimSpace(button),
		})
	})
}

func (m *Manager) browserDOMCUAStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	action, _ := params["action"].(string)
	nodeID, _ := params["node_id"].(string)
	text, _ := params["text"].(string)
	return m.runBrowserAction(ctx, ToolBrowserDOMCUA, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.DOMCUA(ctx, browser.DOMCUARequest{
			Action: strings.TrimSpace(action),
			NodeID: strings.TrimSpace(nodeID),
			X:      intParam(params["x"]),
			Y:      intParam(params["y"]),
			Text:   text,
			Keys:   keysFromParam(params["keys"]),
		})
	})
}

func (m *Manager) browserLocatorStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	action, _ := params["action"].(string)
	selector, _ := params["selector"].(string)
	text, _ := params["text"].(string)
	attribute, _ := params["attribute"].(string)
	value, _ := params["value"].(string)
	state, _ := params["state"].(string)
	checked, _ := params["checked"].(bool)
	return m.runBrowserAction(ctx, ToolBrowserLocator, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Locator(ctx, browser.LocatorRequest{
			Action:    strings.TrimSpace(action),
			Selector:  strings.TrimSpace(selector),
			Text:      text,
			Attribute: strings.TrimSpace(attribute),
			Value:     value,
			State:     strings.TrimSpace(state),
			Timeout:   intParam(params["timeout"]),
			Checked:   checked,
		})
	})
}

func (m *Manager) browserDevLogsStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	limit := intParam(params["limit"])
	return m.runBrowserAction(ctx, ToolBrowserDevLogs, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.DevLogs(ctx, browser.ConsoleRequest{Limit: limit})
	})
}

func (m *Manager) browserDownloadsStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	limit := intParam(params["limit"])
	return m.runBrowserAction(ctx, ToolBrowserDownloads, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.Downloads(ctx, browser.DownloadsRequest{Limit: limit})
	})
}

func (m *Manager) browserUserTabsStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	return m.runBrowserAction(ctx, ToolBrowserUserTabs, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.UserTabs(ctx, browser.UserTabsRequest{})
	})
}

func (m *Manager) browserSessionNameStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	name, _ := params["name"].(string)
	return m.runBrowserAction(ctx, ToolBrowserSessionName, params, func(sess browser.SessionBackend) (browser.ActionResult, error) {
		return sess.SetSessionName(ctx, browser.SessionNameRequest{Name: strings.TrimSpace(name)})
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

func keysFromParam(raw interface{}) []string {
	if key, ok := raw.(string); ok {
		if key = strings.TrimSpace(key); key != "" {
			return []string{key}
		}
		return nil
	}
	return stringSliceFromParam(raw)
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

func boolParamWithPresence(raw interface{}) (bool, bool) {
	if raw == nil {
		return false, false
	}
	switch v := raw.(type) {
	case bool:
		return v, true
	default:
		return false, false
	}
}
