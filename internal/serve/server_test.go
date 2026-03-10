package serve

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

	toolapiimpl "github.com/dreamSailing/vb-coding/internal/toolapi/impl"
)

func TestStdioFlow_HandshakeSessionListPreflightApproveExecute(t *testing.T) {
	workspace := t.TempDir()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	srv, err := NewServer(Options{
		Transport:             "stdio",
		DefaultWorkspacePath:  workspace,
		DefaultAllowedTools:   []string{"read", "bash"},
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

	readResponse := func(id float64, timeout time.Duration) map[string]any {
		deadline := time.Now().Add(timeout)
		for {
			remain := time.Until(deadline)
			if remain <= 0 {
				t.Fatalf("timeout waiting response id=%v", id)
			}
			m := readLine(remain)
			if m["id"] == nil {
				continue
			}
			mid, ok := m["id"].(float64)
			if !ok {
				continue
			}
			if mid == id {
				return m
			}
		}
	}

	readEvent := func(timeout time.Duration) map[string]any {
		deadline := time.Now().Add(timeout)
		for {
			remain := time.Until(deadline)
			if remain <= 0 {
				t.Fatalf("timeout waiting event")
			}
			m := readLine(remain)
			if m["method"] == "event" {
				return m
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
				"allowedTools":          []any{"read", "bash"},
				"requireApprovalDigest": true,
			},
		},
	})
	resp = readResponse(3, 2*time.Second)
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got: %v", resp)
	}
	sessionID, _ := result["sessionID"].(string)
	if sessionID == "" {
		t.Fatalf("expected sessionID, got: %v", resp)
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
				"tool": "bash",
				"parameters": map[string]any{
					"command": "echo hi",
				},
			},
		},
	})
	ev := readEvent(2 * time.Second)
	if ev["method"] != "event" {
		t.Fatalf("expected event, got: %v", ev)
	}
	resp = readResponse(5, 2*time.Second)
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
		"id":      6,
		"method":  "prompt.resolve",
		"params": map[string]any{
			"sessionID":      sessionID,
			"requestID":      requestID,
			"decision":       "allow_once",
			"approvalDigest": digest,
			"policyID":       "test",
		},
	})
	resp = readResponse(6, 2*time.Second)
	if resp["result"] == nil {
		t.Fatalf("expected result, got: %v", resp)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "tool.execute",
		"params": map[string]any{
			"sessionID": sessionID,
			"call": map[string]any{
				"id":   "c_1",
				"tool": "bash",
				"parameters": map[string]any{
					"command": "echo hi",
				},
			},
		},
	})
	ev = readEvent(5 * time.Second)
	ev2 := readEvent(5 * time.Second)
	resp = readResponse(7, 5*time.Second)
	et1 := ""
	if p, ok := ev["params"].(map[string]any); ok {
		et1, _ = p["type"].(string)
	}
	et2 := ""
	if p, ok := ev2["params"].(map[string]any); ok {
		et2, _ = p["type"].(string)
	}
	if !(et1 == "ToolCall" && et2 == "ToolResult") && !(et2 == "ToolCall" && et1 == "ToolResult") {
		t.Fatalf("expected ToolCall/ToolResult events, got: %v %v", et1, et2)
	}
	if resp["result"] == nil {
		t.Fatalf("expected result, got: %v", resp)
	}
	resObj, _ := resp["result"].(map[string]any)
	status, _ := resObj["status"].(string)
	if status != "success" {
		t.Fatalf("expected success status, got: %v", resp)
	}
	display, _ := resObj["display"].(string)
	if !strings.Contains(display, "hi") {
		t.Fatalf("expected output contains hi, got: %v", resp)
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
