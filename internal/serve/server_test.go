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

func TestStdioFlow_HandshakeSessionListPreflightApproveExecute(t *testing.T) {
	workspace := t.TempDir()
	targetFile := filepath.Join(workspace, "x.txt")
	if err := os.WriteFile(targetFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	srv, err := NewServer(Options{
		Transport:             "stdio",
		DefaultWorkspacePath:  workspace,
		DefaultAllowedTools:   []string{"read", "edit"},
		DefaultSandboxMode:    "full_access",
		RequireApprovalDigest: true,
	}, inR, outW, io.Discard, toolapiimpl.NewServices())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- srv.Run(ctx)
	}()

	rd := bufio.NewReader(outR)
	write := func(obj any) {
		b, _ := json.Marshal(obj)
		_, _ = inW.Write(append(b, '\n'))
	}

	readLine := func(timeout time.Duration) map[string]any {
		type res struct {
			line string
			err  error
		}
		ch := make(chan res, 1)
		go func() {
			l, e := rd.ReadString('\n')
			ch <- res{line: l, err: e}
		}()
		select {
		case r := <-ch:
			if r.err != nil {
				t.Fatalf("read: %v", r.err)
			}
			m := map[string]any{}
			if err := json.Unmarshal([]byte(strings.TrimSpace(r.line)), &m); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			return m
		case <-time.After(timeout):
			t.Fatalf("timeout reading output")
			return nil
		}
	}

	readResponseAndEvents := func(id float64, timeout time.Duration) (map[string]any, []map[string]any) {
		deadline := time.Now().Add(timeout)
		events := make([]map[string]any, 0, 4)
		for {
			remain := time.Until(deadline)
			if remain <= 0 {
				t.Fatalf("timeout waiting response id=%v", id)
			}
			m := readLine(remain)
			if m["method"] == "event" {
				events = append(events, m)
				continue
			}
			if m["id"] == nil {
				continue
			}
			mid, ok := m["id"].(float64)
			if !ok {
				continue
			}
			if mid == id {
				return m, events
			}
		}
	}

	readResponse := func(id float64, timeout time.Duration) map[string]any {
		resp, _ := readResponseAndEvents(id, timeout)
		return resp
	}

	eventTypes := func(events []map[string]any) []string {
		out := make([]string, 0, len(events))
		for _, ev := range events {
			params, ok := ev["params"].(map[string]any)
			if !ok {
				continue
			}
			eventType, _ := params["event_type"].(string)
			if eventType != "" {
				out = append(out, eventType)
			}
		}
		return out
	}

	findEvent := func(events []map[string]any, target string) map[string]any {
		for _, ev := range events {
			params, ok := ev["params"].(map[string]any)
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

	validateEvents := func(events []map[string]any) {
		for _, ev := range events {
			params, ok := ev["params"]
			if !ok {
				t.Fatalf("event missing params: %v", ev)
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

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tool.list",
		"params":  map[string]any{"sessionID": "s_none"},
	})
	resp := readResponse(1, 2*time.Second)
	if resp["error"] == nil {
		t.Fatalf("expected error, got: %v", resp)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "initialize",
		"params":  map[string]any{"client": map[string]any{"name": "test", "version": "0.0.1"}, "protocolVersion": "1.0"},
	})
	resp = readResponse(2, 2*time.Second)
	if resp["result"] == nil {
		t.Fatalf("expected result, got: %v", resp)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "session.create",
		"params": map[string]any{
			"workspacePath": workspace,
			"options": map[string]any{
				"allowedTools":          []any{"read", "edit"},
				"requireApprovalDigest": true,
				"sandboxMode":           "full_access",
			},
		},
	})
	resp, events := readResponseAndEvents(3, 2*time.Second)
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got: %v", resp)
	}
	sessionID, _ := result["sessionID"].(string)
	if sessionID == "" {
		t.Fatalf("expected sessionID, got: %v", resp)
	}
	validateEvents(events)
	if params := findEvent(events, "session.updated"); params == nil {
		t.Fatalf("expected session.updated event, got: %v", eventTypes(events))
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      31,
		"method":  "session.get",
		"params":  map[string]any{"sessionID": sessionID},
	})
	resp = readResponse(31, 2*time.Second)
	result, ok = resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected session.get result object, got: %v", resp)
	}
	sessionObj, ok := result["session"].(map[string]any)
	if !ok {
		t.Fatalf("expected session object, got: %v", resp)
	}
	if got, _ := sessionObj["session_id"].(string); got != sessionID {
		t.Fatalf("expected session_id %q, got: %v", sessionID, sessionObj)
	}
	if got, _ := sessionObj["status"].(string); got != "idle" {
		t.Fatalf("expected idle session status, got: %v", sessionObj)
	}
	if got, _ := sessionObj["title"].(string); got != filepath.Base(workspace) {
		t.Fatalf("expected session title %q, got: %v", filepath.Base(workspace), sessionObj)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      32,
		"method":  "session.list",
	})
	resp = readResponse(32, 2*time.Second)
	result, ok = resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected session.list result object, got: %v", resp)
	}
	sessionsAny, ok := result["sessions"].([]any)
	if !ok || len(sessionsAny) == 0 {
		t.Fatalf("expected sessions list, got: %v", resp)
	}
	foundSession := false
	for _, item := range sessionsAny {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if got, _ := entry["session_id"].(string); got == sessionID {
			foundSession = true
			break
		}
	}
	if !foundSession {
		t.Fatalf("expected session %q in list, got: %v", sessionID, sessionsAny)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tool.list",
		"params":  map[string]any{"sessionID": sessionID},
	})
	resp = readResponse(4, 2*time.Second)
	result, ok = resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got: %v", resp)
	}
	toolsAny, ok := result["tools"].([]any)
	if !ok || len(toolsAny) == 0 {
		t.Fatalf("expected tools list, got: %v", resp)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tool.preflight",
		"params": map[string]any{
			"sessionID": sessionID,
			"call": map[string]any{
				"id":   "c_1",
				"tool": "edit",
				"parameters": map[string]any{
					"mode":    "single",
					"file":    targetFile,
					"find":    "hello",
					"replace": "hi",
				},
			},
		},
	})
	resp, events = readResponseAndEvents(5, 2*time.Second)
	validateEvents(events)
	p := findEvent(events, "approval.required")
	if p == nil {
		t.Fatalf("expected approval.required event, got: %v", eventTypes(events))
	}
	if ver, _ := p["version"].(string); ver != "v1" {
		t.Fatalf("expected protocol v1 event, got: %v", p)
	}
	if findEvent(events, "session.updated") == nil {
		t.Fatalf("expected session.updated event after preflight, got: %v", eventTypes(events))
	}
	pre, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got: %v", resp)
	}
	requestID, _ := pre["requestID"].(string)
	digest, _ := pre["approvalDigest"].(string)
	if requestID == "" || digest == "" {
		t.Fatalf("expected requestID and approvalDigest, got: %v", resp)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      51,
		"method":  "session.get",
		"params":  map[string]any{"sessionID": sessionID},
	})
	resp = readResponse(51, 2*time.Second)
	result, ok = resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected session.get result object after preflight, got: %v", resp)
	}
	sessionObj, ok = result["session"].(map[string]any)
	if !ok {
		t.Fatalf("expected session object after preflight, got: %v", resp)
	}
	if got, _ := sessionObj["status"].(string); got != "waiting_input" {
		t.Fatalf("expected waiting_input status after preflight, got: %v", sessionObj)
	}
	if got, _ := sessionObj["preview"].(string); !strings.Contains(got, "edit:") {
		t.Fatalf("expected session preview to mention pending edit, got: %v", sessionObj)
	}
	pendingApprovals, ok := sessionObj["pending_approvals"].([]any)
	if !ok || len(pendingApprovals) == 0 {
		t.Fatalf("expected pending approvals after preflight, got: %v", sessionObj)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "approval.resolve",
		"params": map[string]any{
			"sessionID":      sessionID,
			"approvalID":     requestID,
			"decision":       "allow_once",
			"approvalDigest": digest,
			"policyID":       "test",
		},
	})
	resp, events = readResponseAndEvents(6, 2*time.Second)
	validateEvents(events)
	if resp["result"] == nil {
		t.Fatalf("expected result, got: %v", resp)
	}
	resolved := findEvent(events, "approval.resolved")
	if resolved == nil {
		t.Fatalf("expected approval.resolved event, got: %v", eventTypes(events))
	}
	if got, _ := resolved["request_id"].(string); got != requestID {
		t.Fatalf("expected resolved request_id %q, got: %v", requestID, resolved)
	}
	if findEvent(events, "session.updated") == nil {
		t.Fatalf("expected session.updated after prompt resolve, got: %v", eventTypes(events))
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "request.start",
		"params": map[string]any{
			"sessionID": sessionID,
			"call": map[string]any{
				"id":   "c_1",
				"tool": "edit",
				"parameters": map[string]any{
					"mode":    "single",
					"file":    targetFile,
					"find":    "hello",
					"replace": "hi",
				},
			},
		},
	})
	resp, events = readResponseAndEvents(7, 5*time.Second)
	validateEvents(events)
	types := eventTypes(events)
	if findEvent(events, "request.started") == nil || findEvent(events, "request.completed") == nil {
		t.Fatalf("expected request.started/request.completed events, got: %v", types)
	}
	if findEvent(events, "tool.call") == nil || findEvent(events, "tool.result") == nil {
		t.Fatalf("expected tool.call/tool.result events, got: %v", types)
	}
	if resp["result"] == nil {
		t.Fatalf("expected result, got: %v", resp)
	}
	resObj, _ := resp["result"].(map[string]any)
	status, _ := resObj["status"].(string)
	if status != "success" {
		t.Fatalf("expected success status, got: %v", resp)
	}
	if body, err := os.ReadFile(targetFile); err != nil {
		t.Fatalf("read updated file: %v", err)
	} else if !strings.Contains(string(body), "hi") {
		t.Fatalf("expected file content to be updated, got %q", string(body))
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      71,
		"method":  "session.get",
		"params":  map[string]any{"sessionID": sessionID},
	})
	resp = readResponse(71, 2*time.Second)
	result, ok = resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected session.get result object after request, got: %v", resp)
	}
	sessionObj, ok = result["session"].(map[string]any)
	if !ok {
		t.Fatalf("expected session object after request, got: %v", resp)
	}
	if got, _ := sessionObj["status"].(string); got != "idle" {
		t.Fatalf("expected idle session status after request, got: %v", sessionObj)
	}
	if got, _ := sessionObj["preview"].(string); !strings.Contains(got, "已编辑") {
		t.Fatalf("expected session preview to include edit summary, got: %v", sessionObj)
	}

	f := filepath.Join(workspace, "x.txt")
	if _, err := os.Stat(f); err != nil && !os.IsNotExist(err) {
		t.Fatalf("unexpected stat error: %v", err)
	}

	_ = inW.Close()
	_ = outW.Close()
	cancel()
	<-done
}

func TestStdioFlow_RequestStartReturnsApprovalDigestAndLifecycleEvents(t *testing.T) {
	workspace := t.TempDir()
	targetFile := filepath.Join(workspace, "request-start.txt")
	if err := os.WriteFile(targetFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	srv, err := NewServer(Options{
		Transport:             "stdio",
		DefaultWorkspacePath:  workspace,
		DefaultAllowedTools:   []string{"edit"},
		DefaultSandboxMode:    "full_access",
		RequireApprovalDigest: true,
	}, inR, outW, io.Discard, toolapiimpl.NewServices())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- srv.Run(ctx)
	}()

	rd := bufio.NewReader(outR)
	write := func(obj any) {
		b, _ := json.Marshal(obj)
		_, _ = inW.Write(append(b, '\n'))
	}

	readLine := func(timeout time.Duration) map[string]any {
		type res struct {
			line string
			err  error
		}
		ch := make(chan res, 1)
		go func() {
			l, e := rd.ReadString('\n')
			ch <- res{line: l, err: e}
		}()
		select {
		case r := <-ch:
			if r.err != nil {
				t.Fatalf("read: %v", r.err)
			}
			m := map[string]any{}
			if err := json.Unmarshal([]byte(strings.TrimSpace(r.line)), &m); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			return m
		case <-time.After(timeout):
			t.Fatalf("timeout reading output")
			return nil
		}
	}

	readResponseAndEvents := func(id float64, timeout time.Duration) (map[string]any, []map[string]any) {
		deadline := time.Now().Add(timeout)
		events := make([]map[string]any, 0, 4)
		for {
			remain := time.Until(deadline)
			if remain <= 0 {
				t.Fatalf("timeout waiting response id=%v", id)
			}
			m := readLine(remain)
			if m["method"] == "event" {
				events = append(events, m)
				continue
			}
			mid, ok := m["id"].(float64)
			if !ok || mid != id {
				continue
			}
			return m, events
		}
	}

	findEvent := func(events []map[string]any, target string) map[string]any {
		for _, ev := range events {
			params, ok := ev["params"].(map[string]any)
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

	validateEvents := func(events []map[string]any) {
		for _, ev := range events {
			params, ok := ev["params"]
			if !ok {
				t.Fatalf("event missing params: %v", ev)
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

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{"client": map[string]any{"name": "test", "version": "0.0.1"}, "protocolVersion": "1.0"},
	})
	if resp, _ := readResponseAndEvents(1, 2*time.Second); resp["result"] == nil {
		t.Fatalf("expected initialize result, got: %v", resp)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "session.create",
		"params": map[string]any{
			"workspacePath": workspace,
			"options": map[string]any{
				"allowedTools":          []any{"edit"},
				"requireApprovalDigest": true,
				"sandboxMode":           "full_access",
			},
		},
	})
	resp, _ := readResponseAndEvents(2, 2*time.Second)
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected session.create result, got: %v", resp)
	}
	sessionID, _ := result["sessionID"].(string)
	if sessionID == "" {
		t.Fatalf("expected sessionID, got: %v", resp)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "request.start",
		"params": map[string]any{
			"sessionID": sessionID,
			"call": map[string]any{
				"id":   "req_1",
				"tool": "edit",
				"parameters": map[string]any{
					"mode":    "single",
					"file":    targetFile,
					"find":    "hello",
					"replace": "hi",
				},
			},
		},
	})
	resp, events := readResponseAndEvents(3, 5*time.Second)
	validateEvents(events)
	if findEvent(events, "request.started") == nil {
		t.Fatalf("expected request.started event, got: %v", events)
	}
	if findEvent(events, "approval.required") == nil {
		t.Fatalf("expected approval.required event, got: %v", events)
	}
	if findEvent(events, "session.updated") == nil {
		t.Fatalf("expected session.updated event, got: %v", events)
	}

	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected ConfirmationRequired error, got: %v", resp)
	}
	if msg, _ := errObj["message"].(string); msg != "ConfirmationRequired" {
		t.Fatalf("expected ConfirmationRequired, got: %v", resp)
	}
	errData, ok := errObj["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected error data, got: %v", resp)
	}
	requestID, _ := errData["requestID"].(string)
	digest, _ := errData["approvalDigest"].(string)
	if requestID == "" || digest == "" {
		t.Fatalf("expected requestID and approvalDigest in error data, got: %v", errObj)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "approval.resolve",
		"params": map[string]any{
			"sessionID":      sessionID,
			"approvalID":     requestID,
			"decision":       "allow_once",
			"approvalDigest": digest,
		},
	})
	if resp, events = readResponseAndEvents(4, 2*time.Second); resp["result"] == nil || findEvent(events, "approval.resolved") == nil {
		t.Fatalf("expected approval resolve flow, got resp=%v events=%v", resp, events)
	}
	validateEvents(events)

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "request.start",
		"params": map[string]any{
			"sessionID": sessionID,
			"call": map[string]any{
				"id":   "req_1",
				"tool": "edit",
				"parameters": map[string]any{
					"mode":    "single",
					"file":    targetFile,
					"find":    "hello",
					"replace": "hi",
				},
			},
		},
	})
	resp, events = readResponseAndEvents(5, 5*time.Second)
	validateEvents(events)
	if findEvent(events, "request.completed") == nil {
		t.Fatalf("expected request.completed event, got: %v", events)
	}
	if findEvent(events, "tool.result") == nil {
		t.Fatalf("expected tool.result event, got: %v", events)
	}
	result, ok = resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected success result, got: %v", resp)
	}
	if got, _ := result["status"].(string); got != "success" {
		t.Fatalf("expected success status, got: %v", resp)
	}
	if body, err := os.ReadFile(targetFile); err != nil {
		t.Fatalf("read updated file: %v", err)
	} else if !strings.Contains(string(body), "hi") {
		t.Fatalf("expected file content to be updated, got %q", string(body))
	}

	_ = inW.Close()
	_ = outW.Close()
	cancel()
	<-done
}

func TestStdioFlow_RequestCancelEmitsRequestFailed(t *testing.T) {
	workspace := t.TempDir()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	srv, err := NewServer(Options{
		Transport:            "stdio",
		DefaultWorkspacePath: workspace,
		DefaultAllowedTools:  []string{"bash"},
		DefaultSandboxMode:   "full_access",
	}, inR, outW, io.Discard, toolapiimpl.NewServices())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- srv.Run(ctx)
	}()

	rd := bufio.NewReader(outR)
	write := func(obj any) {
		b, _ := json.Marshal(obj)
		_, _ = inW.Write(append(b, '\n'))
	}
	writeAsync := func(obj any) {
		go write(obj)
	}

	readLine := func(timeout time.Duration) map[string]any {
		type res struct {
			line string
			err  error
		}
		ch := make(chan res, 1)
		go func() {
			l, e := rd.ReadString('\n')
			ch <- res{line: l, err: e}
		}()
		select {
		case r := <-ch:
			if r.err != nil {
				t.Fatalf("read: %v", r.err)
			}
			m := map[string]any{}
			if err := json.Unmarshal([]byte(strings.TrimSpace(r.line)), &m); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			return m
		case <-time.After(timeout):
			t.Fatalf("timeout reading output")
			return nil
		}
	}

	readResponseAndEvents := func(id float64, timeout time.Duration) (map[string]any, []map[string]any) {
		deadline := time.Now().Add(timeout)
		events := make([]map[string]any, 0, 4)
		for {
			remain := time.Until(deadline)
			if remain <= 0 {
				t.Fatalf("timeout waiting response id=%v", id)
			}
			m := readLine(remain)
			if m["method"] == "event" {
				events = append(events, m)
				continue
			}
			mid, ok := m["id"].(float64)
			if !ok || mid != id {
				continue
			}
			return m, events
		}
	}

	readEventsUntil := func(timeout time.Duration, wantType string) []map[string]any {
		deadline := time.Now().Add(timeout)
		events := make([]map[string]any, 0, 4)
		for {
			remain := time.Until(deadline)
			if remain <= 0 {
				t.Fatalf("timeout waiting event %q", wantType)
			}
			m := readLine(remain)
			if m["method"] != "event" {
				continue
			}
			events = append(events, m)
			params, ok := m["params"].(map[string]any)
			if !ok {
				continue
			}
			if eventType, _ := params["event_type"].(string); eventType == wantType {
				return events
			}
		}
	}

	findEvent := func(events []map[string]any, target string) map[string]any {
		for _, ev := range events {
			params, ok := ev["params"].(map[string]any)
			if !ok {
				continue
			}
			if eventType, _ := params["event_type"].(string); eventType == target {
				return params
			}
		}
		return nil
	}

	validateEvents := func(events []map[string]any) {
		for _, ev := range events {
			params, ok := ev["params"]
			if !ok {
				t.Fatalf("event missing params: %v", ev)
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

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{"client": map[string]any{"name": "test", "version": "0.0.1"}, "protocolVersion": "1.0"},
	})
	if resp, _ := readResponseAndEvents(1, 2*time.Second); resp["result"] == nil {
		t.Fatalf("expected initialize result, got: %v", resp)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "session.create",
		"params": map[string]any{
			"workspacePath": workspace,
			"options": map[string]any{
				"allowedTools":  []any{"bash"},
				"executionMode": "auto",
				"sandboxMode":   "full_access",
			},
		},
	})
	resp, events := readResponseAndEvents(2, 2*time.Second)
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected session.create result, got: %v", resp)
	}
	sessionID, _ := result["sessionID"].(string)
	if sessionID == "" {
		t.Fatalf("expected sessionID, got: %v", resp)
	}
	validateEvents(events)

	writeAsync(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "request.start",
		"params": map[string]any{
			"sessionID": sessionID,
			"call": map[string]any{
				"id":   "req_cancel_1",
				"tool": "bash",
				"parameters": map[string]any{
					"command": "Start-Sleep -Seconds 5",
				},
			},
		},
	})
	startEvents := readEventsUntil(5*time.Second, "request.started")
	startEvents = append(startEvents, readEventsUntil(5*time.Second, "session.updated")...)
	validateEvents(startEvents)
	if findEvent(startEvents, "request.started") == nil || findEvent(startEvents, "session.updated") == nil {
		t.Fatalf("expected request.started + session.updated events, got: %v", startEvents)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "request.cancel",
		"params": map[string]any{
			"sessionID": sessionID,
			"requestID": "req_cancel_1",
		},
	})
	resp, cancelEvents := readResponseAndEvents(4, 2*time.Second)
	validateEvents(cancelEvents)
	result, ok = resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected request.cancel result, got: %v", resp)
	}
	if got, _ := result["ok"].(bool); !got {
		t.Fatalf("expected request.cancel ok=true, got: %v", resp)
	}

	resp, finishEvents := readResponseAndEvents(3, 6*time.Second)
	validateEvents(finishEvents)
	if failed := findEvent(finishEvents, "request.failed"); failed != nil {
		if findEvent(finishEvents, "request.completed") != nil {
			t.Fatalf("did not expect request.completed after cancel, got: %v", finishEvents)
		}
		if findEvent(finishEvents, "tool.result") == nil {
			t.Fatalf("expected tool.result event after cancel, got: %v", finishEvents)
		}
		result, ok = resp["result"].(map[string]any)
		if !ok {
			t.Fatalf("expected request.start response result, got: %v", resp)
		}
		if got, _ := result["status"].(string); got != "error" {
			t.Fatalf("expected canceled tool result status=error, got: %v", resp)
		}
	} else {
		if findEvent(finishEvents, "approval.required") == nil {
			t.Fatalf("expected request.failed or approval.required event, got: %v", finishEvents)
		}
		if errObj, ok := resp["error"].(map[string]any); !ok || errObj == nil {
			t.Fatalf("expected confirmation error response, got: %v", resp)
		}
	}

	_ = inW.Close()
	_ = outW.Close()
	cancel()
	<-done
}

func TestStdioFlow_InquiryResolveAliasCompletesRequest(t *testing.T) {
	workspace := t.TempDir()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	srv, err := NewServer(Options{
		Transport:            "stdio",
		DefaultWorkspacePath: workspace,
		DefaultAllowedTools:  []string{"ask_user_question"},
	}, inR, outW, io.Discard, toolapiimpl.NewServices())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- srv.Run(ctx)
	}()

	rd := bufio.NewReader(outR)
	write := func(obj any) {
		b, _ := json.Marshal(obj)
		_, _ = inW.Write(append(b, '\n'))
	}

	readLine := func(timeout time.Duration) map[string]any {
		type res struct {
			line string
			err  error
		}
		ch := make(chan res, 1)
		go func() {
			l, e := rd.ReadString('\n')
			ch <- res{line: l, err: e}
		}()
		select {
		case r := <-ch:
			if r.err != nil {
				t.Fatalf("read: %v", r.err)
			}
			m := map[string]any{}
			if err := json.Unmarshal([]byte(strings.TrimSpace(r.line)), &m); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			return m
		case <-time.After(timeout):
			t.Fatalf("timeout reading output")
			return nil
		}
	}

	readResponseAndEvents := func(id float64, timeout time.Duration) (map[string]any, []map[string]any) {
		deadline := time.Now().Add(timeout)
		events := make([]map[string]any, 0, 4)
		for {
			remain := time.Until(deadline)
			if remain <= 0 {
				t.Fatalf("timeout waiting response id=%v", id)
			}
			m := readLine(remain)
			if m["method"] == "event" {
				events = append(events, m)
				continue
			}
			mid, ok := m["id"].(float64)
			if !ok || mid != id {
				continue
			}
			return m, events
		}
	}

	findEvent := func(events []map[string]any, target string) map[string]any {
		for _, ev := range events {
			params, ok := ev["params"].(map[string]any)
			if !ok {
				continue
			}
			if eventType, _ := params["event_type"].(string); eventType == target {
				return params
			}
		}
		return nil
	}

	validateEvents := func(events []map[string]any) {
		for _, ev := range events {
			params, ok := ev["params"]
			if !ok {
				t.Fatalf("event missing params: %v", ev)
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

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{"client": map[string]any{"name": "test", "version": "0.0.1"}, "protocolVersion": "1.0"},
	})
	if resp, _ := readResponseAndEvents(1, 2*time.Second); resp["result"] == nil {
		t.Fatalf("expected initialize result, got: %v", resp)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "session.create",
		"params": map[string]any{
			"workspacePath": workspace,
			"options": map[string]any{
				"allowedTools": []any{"ask_user_question"},
			},
		},
	})
	resp, events := readResponseAndEvents(2, 2*time.Second)
	validateEvents(events)
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected session.create result, got: %v", resp)
	}
	sessionID, _ := result["sessionID"].(string)
	if sessionID == "" {
		t.Fatalf("expected sessionID, got: %v", resp)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "request.start",
		"params": map[string]any{
			"sessionID": sessionID,
			"call": map[string]any{
				"id":   "inq_call_1",
				"tool": "ask_user_question",
				"parameters": map[string]any{
					"question": "选择模式",
					"options":  []any{"auto", "plan"},
				},
			},
		},
	})
	resp, events = readResponseAndEvents(3, 5*time.Second)
	validateEvents(events)
	if findEvent(events, "request.started") == nil || findEvent(events, "inquiry.required") == nil {
		t.Fatalf("expected request.started + inquiry.required, got: %v", events)
	}
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected InquiryRequired error, got: %v", resp)
	}
	if msg, _ := errObj["message"].(string); msg != "InquiryRequired" {
		t.Fatalf("expected InquiryRequired, got: %v", resp)
	}
	errData, ok := errObj["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected error data, got: %v", resp)
	}
	requestID, _ := errData["requestID"].(string)
	if requestID == "" {
		t.Fatalf("expected inquiry requestID, got: %v", errObj)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "inquiry.resolve",
		"params": map[string]any{
			"sessionID": sessionID,
			"inquiryID": requestID,
			"option":    "plan",
			"text":      "先给出方案",
		},
	})
	resp, events = readResponseAndEvents(4, 2*time.Second)
	validateEvents(events)
	if resp["result"] == nil || findEvent(events, "inquiry.resolved") == nil {
		t.Fatalf("expected inquiry resolve flow, got resp=%v events=%v", resp, events)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "request.start",
		"params": map[string]any{
			"sessionID": sessionID,
			"call": map[string]any{
				"id":   "inq_call_1",
				"tool": "ask_user_question",
				"parameters": map[string]any{
					"question": "选择模式",
					"options":  []any{"auto", "plan"},
				},
			},
		},
	})
	resp, events = readResponseAndEvents(5, 5*time.Second)
	validateEvents(events)
	if findEvent(events, "request.completed") == nil || findEvent(events, "tool.result") == nil {
		t.Fatalf("expected completed request with tool result, got: %v", events)
	}
	result, ok = resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected success result, got: %v", resp)
	}
	data, ok := result["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected result data, got: %v", resp)
	}
	if got, _ := data["option"].(string); got != "plan" {
		t.Fatalf("expected resolved option plan, got: %v", resp)
	}

	_ = inW.Close()
	_ = outW.Close()
	cancel()
	<-done
}

func TestStdioFlow_SessionResumeAndDeleteAliases(t *testing.T) {
	workspace := t.TempDir()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	srv, err := NewServer(Options{
		Transport:            "stdio",
		DefaultWorkspacePath: workspace,
	}, inR, outW, io.Discard, toolapiimpl.NewServices())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- srv.Run(ctx)
	}()

	rd := bufio.NewReader(outR)
	write := func(obj any) {
		b, _ := json.Marshal(obj)
		_, _ = inW.Write(append(b, '\n'))
	}

	readLine := func(timeout time.Duration) map[string]any {
		type res struct {
			line string
			err  error
		}
		ch := make(chan res, 1)
		go func() {
			l, e := rd.ReadString('\n')
			ch <- res{line: l, err: e}
		}()
		select {
		case r := <-ch:
			if r.err != nil {
				t.Fatalf("read: %v", r.err)
			}
			m := map[string]any{}
			if err := json.Unmarshal([]byte(strings.TrimSpace(r.line)), &m); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			return m
		case <-time.After(timeout):
			t.Fatalf("timeout reading output")
			return nil
		}
	}

	readResponseAndEvents := func(id float64, timeout time.Duration) (map[string]any, []map[string]any) {
		deadline := time.Now().Add(timeout)
		events := make([]map[string]any, 0, 2)
		for {
			remain := time.Until(deadline)
			if remain <= 0 {
				t.Fatalf("timeout waiting response id=%v", id)
			}
			m := readLine(remain)
			if m["method"] == "event" {
				events = append(events, m)
				continue
			}
			mid, ok := m["id"].(float64)
			if !ok || mid != id {
				continue
			}
			return m, events
		}
	}

	validateEvents := func(events []map[string]any) {
		for _, ev := range events {
			params, ok := ev["params"]
			if !ok {
				t.Fatalf("event missing params: %v", ev)
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

	write(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"client": map[string]any{"name": "test", "version": "0.0.1"}, "protocolVersion": "1.0"}})
	if resp, _ := readResponseAndEvents(1, 2*time.Second); resp["result"] == nil {
		t.Fatalf("expected initialize result, got: %v", resp)
	}

	write(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "session.create", "params": map[string]any{"workspacePath": workspace}})
	resp, events := readResponseAndEvents(2, 2*time.Second)
	validateEvents(events)
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected session.create result, got: %v", resp)
	}
	sessionID, _ := result["sessionID"].(string)
	if sessionID == "" {
		t.Fatalf("expected sessionID, got: %v", resp)
	}

	write(map[string]any{"jsonrpc": "2.0", "id": 3, "method": "session.resume", "params": map[string]any{"sessionID": sessionID}})
	resp, events = readResponseAndEvents(3, 2*time.Second)
	validateEvents(events)
	result, ok = resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected session.resume result, got: %v", resp)
	}
	sessionObj, ok := result["session"].(map[string]any)
	if !ok {
		t.Fatalf("expected session object, got: %v", resp)
	}
	if got, _ := sessionObj["session_id"].(string); got != sessionID {
		t.Fatalf("expected session_id %q, got: %v", sessionID, sessionObj)
	}

	write(map[string]any{"jsonrpc": "2.0", "id": 4, "method": "session.delete", "params": map[string]any{"sessionID": sessionID}})
	resp, events = readResponseAndEvents(4, 2*time.Second)
	validateEvents(events)
	result, ok = resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected session.delete result, got: %v", resp)
	}
	if got, _ := result["ok"].(bool); !got {
		t.Fatalf("expected session.delete ok=true, got: %v", resp)
	}

	write(map[string]any{"jsonrpc": "2.0", "id": 5, "method": "session.get", "params": map[string]any{"sessionID": sessionID}})
	resp, _ = readResponseAndEvents(5, 2*time.Second)
	if resp["error"] == nil {
		t.Fatalf("expected SessionNotFound after delete, got: %v", resp)
	}

	_ = inW.Close()
	_ = outW.Close()
	cancel()
	<-done
}
