package serve

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	toolapiimpl "github.com/dreamSailing/eos/internal/toolapi/impl"
	"github.com/dreamSailing/eos/pkg/protocol"
)

type serveHarness struct {
	t      *testing.T
	inW    *io.PipeWriter
	outW   *io.PipeWriter
	reader *bufio.Reader
	cancel context.CancelFunc
	done   chan error
}

func newServeHarness(t *testing.T, opts Options) *serveHarness {
	t.Helper()

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	srv, err := NewServer(opts, inR, outW, io.Discard, toolapiimpl.NewServices())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Run(ctx)
	}()

	return &serveHarness{
		t:      t,
		inW:    inW,
		outW:   outW,
		reader: bufio.NewReader(outR),
		cancel: cancel,
		done:   done,
	}
}

func (h *serveHarness) close() {
	h.t.Helper()
	h.cancel()
	_ = h.inW.Close()
	_ = h.outW.Close()
	<-h.done
}

func (h *serveHarness) write(obj any) {
	h.t.Helper()
	raw, err := json.Marshal(obj)
	if err != nil {
		h.t.Fatalf("marshal request: %v", err)
	}
	if _, err := h.inW.Write(append(raw, '\n')); err != nil {
		h.t.Fatalf("write request: %v", err)
	}
}

func (h *serveHarness) readLine(timeout time.Duration) map[string]any {
	h.t.Helper()
	type res struct {
		line string
		err  error
	}
	ch := make(chan res, 1)
	go func() {
		line, err := h.reader.ReadString('\n')
		ch <- res{line: line, err: err}
	}()

	select {
	case item := <-ch:
		if item.err != nil {
			h.t.Fatalf("read line: %v", item.err)
		}
		out := map[string]any{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(item.line)), &out); err != nil {
			h.t.Fatalf("unmarshal response: %v", err)
		}
		return out
	case <-time.After(timeout):
		h.t.Fatalf("timeout reading output")
		return nil
	}
}

func (h *serveHarness) readResponseAndEvents(id float64, timeout time.Duration) (map[string]any, []map[string]any) {
	h.t.Helper()
	deadline := time.Now().Add(timeout)
	events := make([]map[string]any, 0, 4)
	for {
		remain := time.Until(deadline)
		if remain <= 0 {
			h.t.Fatalf("timeout waiting response id=%v", id)
		}
		item := h.readLine(remain)
		if item["method"] == "event" {
			events = append(events, item)
			continue
		}
		mid, ok := item["id"].(float64)
		if !ok || mid != id {
			continue
		}
		return item, events
	}
}

func validateProtocolEvents(t *testing.T, events []map[string]any) {
	t.Helper()
	for _, item := range events {
		params, ok := item["params"]
		if !ok {
			t.Fatalf("event missing params: %v", item)
		}
		raw, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal event params: %v", err)
		}
		var env protocol.Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("decode protocol envelope: %v", err)
		}
		if err := protocol.ValidateEnvelope(env); err != nil {
			t.Fatalf("invalid protocol envelope: %v; envelope=%+v", err, env)
		}
	}
}

func findEventByType(events []map[string]any, target string) map[string]any {
	for _, item := range events {
		params, ok := item["params"].(map[string]any)
		if !ok {
			continue
		}
		eventType, _ := params["event_type"].(string)
		if eventType == target {
			return params
		}
	}
	return nil
}

func TestServeSessionStore_RestoresPendingApprovalAcrossRestart(t *testing.T) {
	workspace := t.TempDir()
	storePath := filepath.Join(workspace, ".eos", "serve", "sessions.json")
	targetFile := filepath.Join(workspace, "persist.txt")
	if err := os.WriteFile(targetFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	opts := Options{
		Transport:             "stdio",
		DefaultWorkspacePath:  workspace,
		DefaultAllowedTools:   []string{"edit"},
		DefaultSandboxMode:    "full_access",
		RequireApprovalDigest: true,
	}

	first := newServeHarness(t, opts)

	first.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{"client": map[string]any{"name": "test", "version": "0.0.1"}, "protocolVersion": "1.0"},
	})
	if resp, _ := first.readResponseAndEvents(1, 2*time.Second); resp["result"] == nil {
		t.Fatalf("expected initialize result, got: %v", resp)
	}

	first.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "session.create",
		"params": map[string]any{
			"workspacePath": workspace,
			"options": map[string]any{
				"title":                  "Persistent Session",
				"allowedTools":           []any{"edit"},
				"accessMode":             "danger-full-access",
				"requireApprovalDigest":  true,
				"maxConcurrentToolCalls": 1,
				"sandboxMode":            "full_access",
			},
		},
	})
	resp, events := first.readResponseAndEvents(2, 2*time.Second)
	validateProtocolEvents(t, events)
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected session.create result, got: %v", resp)
	}
	sessionID, _ := result["sessionID"].(string)
	if sessionID == "" {
		t.Fatalf("missing sessionID: %v", resp)
	}

	first.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tool.preflight",
		"params": map[string]any{
			"sessionID": sessionID,
			"call": map[string]any{
				"id":   "c_restart_1",
				"tool": "edit",
				"parameters": map[string]any{
					"mode":    "single",
					"file":    targetFile,
					"find":    "hello",
					"replace": "persisted",
				},
			},
		},
	})
	resp, events = first.readResponseAndEvents(3, 2*time.Second)
	validateProtocolEvents(t, events)
	preflight, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected preflight result, got: %v", resp)
	}
	requestID, _ := preflight["requestID"].(string)
	digest, _ := preflight["approvalDigest"].(string)
	if requestID == "" || digest == "" {
		t.Fatalf("expected requestID and approvalDigest, got: %v", resp)
	}
	if ev := findEventByType(events, "approval.required"); ev == nil {
		t.Fatalf("expected approval.required, got: %v", events)
	}

	first.close()

	if _, err := os.Stat(storePath); err != nil {
		t.Fatalf("expected persisted session store at %s: %v", storePath, err)
	}

	second := newServeHarness(t, opts)
	defer second.close()

	second.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      11,
		"method":  "initialize",
		"params":  map[string]any{"client": map[string]any{"name": "test", "version": "0.0.1"}, "protocolVersion": "1.0"},
	})
	if resp, _ := second.readResponseAndEvents(11, 2*time.Second); resp["result"] == nil {
		t.Fatalf("expected initialize result after restart, got: %v", resp)
	}

	second.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      12,
		"method":  "session.list",
	})
	resp, _ = second.readResponseAndEvents(12, 2*time.Second)
	result, ok = resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected session.list result, got: %v", resp)
	}
	items, ok := result["sessions"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("expected restored sessions, got: %v", resp)
	}
	var restored map[string]any
	for _, item := range items {
		entry, _ := item.(map[string]any)
		if got, _ := entry["session_id"].(string); got == sessionID {
			restored = entry
			break
		}
	}
	if restored == nil {
		t.Fatalf("expected restored session %q, got: %v", sessionID, items)
	}
	if got, _ := restored["title"].(string); got != "Persistent Session" {
		t.Fatalf("title=%q, want Persistent Session", got)
	}
	if got, _ := restored["status"].(string); got != "waiting_input" {
		t.Fatalf("status=%q, want waiting_input", got)
	}
	if got, _ := restored["preview"].(string); !strings.Contains(got, "edit:") {
		t.Fatalf("preview=%q, want edit summary", got)
	}
	meta, _ := restored["metadata"].(map[string]any)
	if got, _ := meta["access_mode"].(string); got != "danger-full-access" {
		t.Fatalf("metadata.access_mode=%q, want danger-full-access", got)
	}
	if got, _ := meta["approval_mode"].(string); got != "on-request" {
		t.Fatalf("metadata.approval_mode=%q, want on-request", got)
	}

	second.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      13,
		"method":  "approval.resolve",
		"params": map[string]any{
			"sessionID":      sessionID,
			"approvalID":     requestID,
			"decision":       "allow_once",
			"approvalDigest": digest,
		},
	})
	resp, events = second.readResponseAndEvents(13, 2*time.Second)
	validateProtocolEvents(t, events)
	if resp["result"] == nil || findEventByType(events, "approval.resolved") == nil {
		t.Fatalf("expected approval resolve flow after restart, got resp=%v events=%v", resp, events)
	}

	second.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      14,
		"method":  "request.start",
		"params": map[string]any{
			"sessionID": sessionID,
			"call": map[string]any{
				"id":   "c_restart_1",
				"tool": "edit",
				"parameters": map[string]any{
					"mode":    "single",
					"file":    targetFile,
					"find":    "hello",
					"replace": "persisted",
				},
			},
		},
	})
	resp, events = second.readResponseAndEvents(14, 5*time.Second)
	validateProtocolEvents(t, events)
	started := findEventByType(events, "request.started")
	if started == nil {
		t.Fatalf("expected request.started after restart, got: %v", events)
	}
	startedPayload, _ := started["payload"].(map[string]any)
	if got, _ := startedPayload["input_kind"].(string); got != "tool" {
		t.Fatalf("request.started input_kind=%q, want tool", got)
	}
	if got, _ := startedPayload["mode"].(string); got == "" {
		t.Fatalf("request.started mode should not be empty: %v", started)
	}
	completed := findEventByType(events, "request.completed")
	if completed == nil {
		t.Fatalf("expected request.completed after restart, got: %v", events)
	}
	completedPayload, _ := completed["payload"].(map[string]any)
	if got, _ := completedPayload["summary"].(string); !strings.Contains(got, "已编辑") {
		t.Fatalf("request.completed summary=%q, want edit summary", got)
	}
	result, ok = resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected request.start result, got: %v", resp)
	}
	if got, _ := result["status"].(string); got != "success" {
		t.Fatalf("status=%q, want success", got)
	}
	if body, err := os.ReadFile(targetFile); err != nil {
		t.Fatalf("read updated file: %v", err)
	} else if !strings.Contains(string(body), "persisted") {
		t.Fatalf("expected file content to be updated, got %q", string(body))
	}
}
