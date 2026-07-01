package cli

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

// print.go 提供 headless 一次性查询入口。
//
// 引擎为 Rust-only：通过 pkg/coreapi/sidecar + engineprovider 启动
// eos-core --app-server --stdio 子进程。失败时直接返回 error，不存在
// Go 内核回退路径。
//
// 旧的 pkg/core.Runtime（Eino/Go 内核）路径已整体退役删除。

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/pkg/coreapi"
	"github.com/dreamSailing/eos/pkg/coreapi/engineprovider"
	"github.com/dreamSailing/eos/pkg/coreapi/sidecar"
	sidecarclient "github.com/dreamSailing/eos/pkg/coreapi/sidecar/client"
	"github.com/dreamSailing/eos/pkg/protocol"
)

const rustCoreStoreDirEnv = "EOS_CORE_STORE_DIR"

// PrintOptions holds options for headless print mode
type PrintOptions struct {
	Query           string
	OutputFormat    string // "text", "json", "stream-json"
	AccessMode      string
	ApprovalMode    string
	SandboxMode     string
	SkipPermissions bool
	Workspace       string // optional workspace path; empty = use default
}

// PrintResult holds the result of a print mode execution
type PrintResult struct {
	Content     string   `json:"content"`
	Model       string   `json:"model"`
	InputTokens *int     `json:"input_tokens,omitempty"`
	ReplyTokens *int     `json:"reply_tokens,omitempty"`
	TotalTokens *int     `json:"total_tokens,omitempty"`
	DurationMs  int      `json:"duration_ms"`
	CostUSD     *float64 `json:"cost_usd,omitempty"`
}

// RunPrintMode executes a single query in headless mode and outputs the result.
//
// Production path: starts an eos-core sidecar via engineprovider.Select (ModeAuto
// → Rust-only). On Rust startup failure or missing required methods, returns
// an error; does NOT silently fall back to Go sharedcore.
func RunPrintMode(opts PrintOptions) error {
	if opts.OutputFormat == "" {
		opts.OutputFormat = "text"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	startedAt := time.Now()
	selected, err := startRustOnlyEngine(ctx, "print", printModeEnv(opts))
	if err != nil {
		return err
	}
	defer selected.Close()

	engine := selected.Engine
	if err := applyPrintModeEnv(ctx, engine, opts); err != nil {
		return err
	}

	content, err := runSingleTurn(ctx, engine, opts.Query, opts.OutputFormat, startedAt)
	if err != nil {
		writePrintError(opts.OutputFormat, err)
		return err
	}

	usage, err := engine.Usage().Summary(ctx)
	if err != nil {
		usage = coreapi.UsageSummary{}
	}
	modelName, _ := resolveActiveModelName(ctx, engine)

	result := PrintResult{
		Content:     content,
		Model:       modelName,
		InputTokens: usage.InputTokens,
		ReplyTokens: usage.ReplyTokens,
		TotalTokens: usage.TotalTokens,
		DurationMs:  int(time.Since(startedAt).Milliseconds()),
		CostUSD:     usage.CostUSD,
	}
	return emitPrintResult(opts.OutputFormat, result, startedAt, usage, modelName)
}

// RunPrintModeStream is a streaming variant of RunPrintMode: writes TextFinal
// payloads to w as they arrive. Used by callers that want incremental output.
func RunPrintModeStream(ctx context.Context, query string, w io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	selected, err := startRustOnlyEngine(ctx, "print-stream", nil)
	if err != nil {
		return err
	}
	defer selected.Close()

	engine := selected.Engine
	if err := applyPrintModeEnv(ctx, engine, PrintOptions{}); err != nil {
		return err
	}

	session, err := ensureHeadlessSession(ctx, engine)
	if err != nil {
		return err
	}

	turnID := fmt.Sprintf("print_stream_%d", time.Now().UnixNano())
	events, unsubscribe := subscribeTurnEvents(ctx, engine, session.ID, turnID)
	defer unsubscribe()

	startDone := startTurnAsync(ctx, engine, coreapi.StartTurnRequest{
		SessionID: session.ID,
		TurnID:    turnID,
		Input:     query,
	})

	var content string
	eventsCh := events
	startDoneCh := startDone
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case result := <-startDoneCh:
			startDoneCh = nil
			if result.err != nil {
				return result.err
			}
			// turn/start is non-blocking; rely on request.done/failed to terminate.
		case ev, ok := <-eventsCh:
			if !ok {
				eventsCh = nil
				if startDoneCh == nil {
					return nil
				}
				continue
			}
			eventType, payload, rawEventType := normalizePrintEvent(ev)
			switch eventType {
			case protocol.EventTypeItemDelta:
				if dt := firstNonEmpty("", payload, "delta_type"); dt != "" && dt != "text" {
					continue // skip reasoning/tool_args deltas in print mode
				}
				text := firstNonEmpty("", payload, "delta", "text", "message")
				content += text
				fmt.Fprint(w, text)
			case protocol.EventTypeItemCompleted:
				// AgentMessage completion carries full text; use it if non-empty.
				if item, ok := payload["item"].(map[string]any); ok {
					if k, _ := item["kind"].(string); k == "agent_message" {
						if text, _ := item["text"].(string); text != "" {
							content = text
						}
					}
				}
			case protocol.EventTypeTextFinal:
				text := firstNonEmpty("", payload, "text", "message")
				if text != "" {
					content = text
					fmt.Fprint(w, text)
				}
				return nil
			case protocol.EventTypeRequestDone:
				if content != "" {
					fmt.Fprintln(w)
				}
				return nil
			case protocol.EventTypeRequestFailed:
				msg := printFailureMessage(rawEventType, payload)
				return fmt.Errorf("%s", msg)
			}
		}
	}
}

// printModeEnv 透传 headless mode 配置到 eos-core 子进程环境变量。
// 与 internal/ui/adapter.tuiOptionEnv 字段保持一致。
func printModeEnv(opts PrintOptions) map[string]string {
	env := map[string]string{}
	if v := strings.TrimSpace(opts.AccessMode); v != "" {
		env["EOS_ACCESS_MODE"] = v
	}
	if v := strings.TrimSpace(opts.ApprovalMode); v != "" {
		env["EOS_APPROVAL_MODE"] = v
	}
	if v := strings.TrimSpace(opts.SandboxMode); v != "" {
		env["EOS_SANDBOX_MODE"] = v
	}
	if ws := strings.TrimSpace(opts.Workspace); ws != "" {
		env["EOS_WORKSPACE_ROOT"] = ws
		env["EOS_SANDBOX_WORKSPACE_ROOT"] = ws
	} else if cwd, err := os.Getwd(); err == nil {
		if cwd := strings.TrimSpace(cwd); cwd != "" {
			env["EOS_WORKSPACE_ROOT"] = cwd
			env["EOS_SANDBOX_WORKSPACE_ROOT"] = cwd
		}
	}
	if opts.SkipPermissions {
		env["EOS_SKIP_PERMISSIONS"] = "1"
	}
	return env
}

// applyPrintModeEnv 把 startup options 推送到已 handshake 的 engine。
// 工作区切换 / execution mode 都走 coreapi.Workspaces / Modes 接口。
func applyPrintModeEnv(ctx context.Context, engine coreapi.Engine, opts PrintOptions) error {
	if engine == nil {
		return fmt.Errorf("core engine unavailable")
	}
	if ws := strings.TrimSpace(opts.Workspace); ws != "" {
		if err := engine.Workspaces().SetForeground(ctx, coreapi.WorkspacePathRequest{Path: ws}); err != nil {
			return fmt.Errorf("set foreground workspace: %w", err)
		}
	}
	return nil
}

func writePrintError(format string, err error) {
	if format == "json" {
		bs, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintln(os.Stdout, string(bs))
		return
	}
	fmt.Fprintln(os.Stderr, "Error:", err.Error())
}

func emitPrintResult(format string, result PrintResult, started time.Time, usage coreapi.UsageSummary, modelName string) error {
	switch format {
	case "json":
		bs, err := json.Marshal(result)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(bs))
	case "stream-json":
		events := []map[string]interface{}{
			{"type": "start", "model": modelName, "timestamp": started.Unix()},
			{"type": "content", "text": result.Content},
			buildDoneEvent(time.Since(started), usage),
		}
		for _, evt := range events {
			bs, _ := json.Marshal(evt)
			fmt.Fprintln(os.Stdout, string(bs))
		}
	default:
		// text 格式：runSingleTurn 已在收到 text_delta 时实时打印到 stdout，
		// 这里只补一行换行 + 元数据页脚到 stderr，避免重复输出正文。
		fmt.Fprintln(os.Stdout)
		parts := []string{
			fmt.Sprintf("Model: %s", modelName),
			fmt.Sprintf("Duration: %v", time.Since(started).Round(time.Millisecond)),
		}
		if usage.TotalTokens != nil {
			parts = append(parts, fmt.Sprintf("Tokens: %d", *usage.TotalTokens))
		}
		if usage.CostUSD != nil {
			parts = append(parts, fmt.Sprintf("Cost: $%.6f", *usage.CostUSD))
		}
		fmt.Fprintf(os.Stderr, "\n---\n%s\n", strings.Join(parts, " | "))
	}
	return nil
}

// buildDoneEvent renders the closing "done" event for stream-json output.
// 接受 coreapi.UsageSummary，与 TUI 路径保持一致。
func buildDoneEvent(elapsed time.Duration, usage coreapi.UsageSummary) map[string]interface{} {
	event := map[string]interface{}{
		"type":        "done",
		"duration_ms": elapsed.Milliseconds(),
	}
	if usage.TotalTokens != nil {
		event["tokens"] = *usage.TotalTokens
	}
	if usage.CostUSD != nil {
		event["cost_usd"] = *usage.CostUSD
	}
	return event
}

// --- shared helpers used by print.go and exec.go ---

// startRustOnlyEngine 启动一次 eos-core sidecar。引擎已收敛为 Rust-only：
// 失败即返回 error，不再有 Go 内核回退路径。
func startRustOnlyEngine(ctx context.Context, callerLabel string, env map[string]string) (engineprovider.Selection, error) {
	_ = callerLabel
	processOpts := productionSidecarProcessOptions(env)
	selection, err := engineprovider.Select(ctx, engineprovider.Options{
		Mode:            engineprovider.ModeAuto,
		Sidecar:         processOpts,
		RequiredMethods: sidecarclient.RequiredMethods,
	})
	if err != nil {
		return engineprovider.Selection{}, fmt.Errorf("start eos-core sidecar (rust-only): %w", err)
	}
	return selection, nil
}

func productionSidecarProcessOptions(env map[string]string) sidecar.ProcessOptions {
	nextEnv := make(map[string]string, len(env)+1)
	for key, value := range env {
		nextEnv[key] = value
	}
	if value, ok := nextEnv[rustCoreStoreDirEnv]; !ok || strings.TrimSpace(value) == "" {
		if value, ok := os.LookupEnv(rustCoreStoreDirEnv); !ok || strings.TrimSpace(value) == "" {
			if dir := headlessRustCoreStoreDir(); dir != "" {
				nextEnv[rustCoreStoreDirEnv] = dir
			}
		}
	}
	return sidecar.ProcessOptions{
		Env:              nextEnv,
		VerifyChecksum:   true,
		RequireSignature: true,
		Stderr:           coreStderrWriter(),
	}
}

func coreStderrWriter() io.Writer {
	dir := filepath.Join(config.ConfiguredLogDir(), "core")
	_ = os.MkdirAll(dir, 0755)
	f, err := os.OpenFile(filepath.Join(dir, "eos-core.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return io.Discard
	}
	return f
}

func headlessRustCoreStoreDir() string {
	if dir, err := os.UserHomeDir(); err == nil && strings.TrimSpace(dir) != "" {
		return filepath.Join(dir, ".eos", "core")
	}
	return ""
}

// runSingleTurn 启动一个 turn，订阅事件流直到 request.done/failed，返回 final 文本。
//
// 对 text 输出格式，每个 text_delta 实时写入 stdout（逐 chunk 涌现，对齐 codex 体验）；
// json/stream-json 仍只在 turn 结束后输出完整结构，保持机器可读契约不变。
func runSingleTurn(ctx context.Context, engine coreapi.Engine, query, outputFormat string, started time.Time) (string, error) {
	if engine == nil {
		return "", fmt.Errorf("core engine unavailable")
	}
	if strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("query is required")
	}
	session, err := ensureHeadlessSession(ctx, engine)
	if err != nil {
		return "", err
	}

	turnID := fmt.Sprintf("cli_turn_%d", time.Now().UnixNano())
	events, unsubscribe := subscribeTurnEvents(ctx, engine, session.ID, turnID)
	defer unsubscribe()

	startDone := startTurnAsync(ctx, engine, coreapi.StartTurnRequest{
		SessionID: session.ID,
		TurnID:    turnID,
		Input:     query,
	})

	// text 格式实时打印 delta；其它格式只累积，turn 结束后统一输出。
	streamLive := strings.EqualFold(strings.TrimSpace(outputFormat), "text")
	var content string
	eventsCh := events
	startDoneCh := startDone
	for {
		select {
		case <-ctx.Done():
			return content, ctx.Err()
		case result := <-startDoneCh:
			startDoneCh = nil
			if result.err != nil {
				return content, result.err
			}
			// turn/start is non-blocking; the turn runs on a background thread
			// and delivers results via events (item.delta / item.completed /
			// request.completed). No fallback timer needed — request.done or
			// request.failed is the termination signal.
		case ev, ok := <-eventsCh:
			if !ok {
				eventsCh = nil
				if startDoneCh == nil {
					return content, nil
				}
				continue
			}
			eventType, payload, rawEventType := normalizePrintEvent(ev)
			switch eventType {
			case protocol.EventTypeItemDelta:
				if dt := firstNonEmpty("", payload, "delta_type"); dt != "" && dt != "text" {
					continue // skip reasoning/tool_args deltas
				}
				text := firstNonEmpty("", payload, "delta", "text", "message")
				content += text
				if streamLive && text != "" {
					fmt.Fprint(os.Stdout, text)
				}
			case protocol.EventTypeItemCompleted:
				if item, ok := payload["item"].(map[string]any); ok {
					if k, _ := item["kind"].(string); k == "agent_message" {
						if text, _ := item["text"].(string); text != "" {
							content = text
						}
					}
				}
			case protocol.EventTypeTextFinal:
				if text := firstNonEmpty("", payload, "text", "message"); text != "" {
					content = text
				}
			case protocol.EventTypeRequestDone:
				return content, nil
			case protocol.EventTypeRequestFailed:
				return content, fmt.Errorf("%s", printFailureMessage(rawEventType, payload))
			}
		}
	}
}

type asyncTurnStartResult struct {
	turn coreapi.Turn
	err  error
}

func startTurnAsync(ctx context.Context, engine coreapi.Engine, req coreapi.StartTurnRequest) <-chan asyncTurnStartResult {
	ch := make(chan asyncTurnStartResult, 1)
	go func() {
		if engine == nil {
			ch <- asyncTurnStartResult{err: fmt.Errorf("core engine unavailable")}
			return
		}
		turn, err := engine.Turns().Start(ctx, req)
		ch <- asyncTurnStartResult{turn: turn, err: err}
	}()
	return ch
}

func ensureHeadlessSession(ctx context.Context, engine coreapi.Engine) (coreapi.Session, error) {
	if engine == nil {
		return coreapi.Session{}, fmt.Errorf("core engine unavailable")
	}
	workspaceRoot := headlessWorkspaceRoot(ctx, engine)
	session, err := engine.Sessions().Current(ctx, coreapi.CurrentSessionRequest{WorkspaceRoot: workspaceRoot})
	if err == nil && strings.TrimSpace(session.ID) != "" {
		return session, nil
	}
	session, err = engine.Sessions().Create(ctx, coreapi.CreateSessionRequest{
		WorkspaceRoot: workspaceRoot,
		Title:         "Headless session",
		Metadata:      map[string]any{"source": "cli"},
	})
	if err != nil {
		return coreapi.Session{}, fmt.Errorf("create headless session: %w", err)
	}
	if strings.TrimSpace(session.ID) == "" {
		return coreapi.Session{}, fmt.Errorf("create headless session returned empty id")
	}
	return session, nil
}

func headlessWorkspaceRoot(ctx context.Context, engine coreapi.Engine) string {
	if engine != nil {
		if snapshot, err := engine.State().Snapshot(ctx, coreapi.StateSnapshotRequest{}); err == nil {
			if root := strings.TrimSpace(snapshot.ForegroundWorkspace); root != "" {
				return root
			}
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return ""
}

// subscribeTurnEvents 复用 engine 的事件总线，filter 当前 session + turn ID。
// 返回的 channel 在 sidecar 进程退出或 ctx cancel 时自动关闭。
func subscribeTurnEvents(ctx context.Context, engine coreapi.Engine, sessionID, turnID string) (<-chan protocol.Envelope, func()) {
	noop := func() {}
	if engine == nil {
		ch := make(chan protocol.Envelope)
		close(ch)
		return ch, noop
	}
	ch, err := engine.Events().Subscribe(ctx, coreapi.EventFilter{SessionID: sessionID, TurnID: turnID})
	if err != nil {
		// Subscribe 失败：返回已关闭的 channel，调用方在 turn.Start 失败时能直接感知。
		closed := make(chan protocol.Envelope)
		close(closed)
		return closed, noop
	}
	return ch, noop
}

func resolveActiveModelName(ctx context.Context, engine coreapi.Engine) (string, error) {
	if engine == nil {
		return "", nil
	}
	items, err := engine.Models().List(ctx)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item.Active {
			return strings.TrimSpace(item.Model), nil
		}
	}
	return "", nil
}

func firstNonEmpty(fallback string, data map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := data[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return strings.TrimSpace(fallback)
}

func normalizePrintEvent(ev protocol.Envelope) (protocol.EventType, map[string]any, protocol.EventType) {
	payload := protocol.ClonePayload(ev.Payload)
	if payload == nil {
		payload = map[string]any{}
	}
	rawEventType := ev.EventType
	eventType := protocol.NormalizeEventType(rawEventType)
	if rawEventType != eventType {
		payload["original_event_type"] = string(rawEventType)
	}
	if eventType == protocol.EventTypeRequestFailed && firstNonEmpty("", payload, "error", "summary", "message", "text") == "" {
		switch rawEventType {
		case protocol.EventTypeTurnCancelled:
			payload["error"] = "request cancelled"
		case protocol.EventTypeTurnInterrupted:
			payload["error"] = "request interrupted"
		}
	}
	return eventType, payload, rawEventType
}

func printFailureMessage(rawEventType protocol.EventType, payload map[string]any) string {
	msg := firstNonEmpty("", payload, "error", "summary", "message", "text")
	if msg != "" {
		return msg
	}
	switch rawEventType {
	case protocol.EventTypeTurnCancelled:
		return "request cancelled"
	case protocol.EventTypeTurnInterrupted:
		return "request interrupted"
	default:
		return "request failed"
	}
}
