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
)

func TestSessionOptions_PlanModeBlocksNonLowRisk(t *testing.T) {
	workspace := t.TempDir()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	srv, err := NewServer(Options{
		Transport:             "stdio",
		DefaultWorkspacePath:  workspace,
		DefaultAllowedTools:   []string{"bash"},
		DefaultSandboxMode:    "full_access",
		RequireApprovalDigest: false,
	}, inR, outW, io.Discard, toolapiimpl.NewServices())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()
	defer func() {
		cancel()
		_ = inW.Close()
		_ = outW.Close()
		<-done
	}()

	rd := bufio.NewReader(outR)
	lines := make(chan string, 64)
	go func() {
		for {
			l, err := rd.ReadString('\n')
			if err != nil {
				return
			}
			lines <- l
		}
	}()
	write := func(obj any) {
		b, _ := json.Marshal(obj)
		_, _ = inW.Write(append(b, '\n'))
	}

	readLine := func() map[string]any {
		deadline := time.Now().Add(5 * time.Second)
		for {
			remain := time.Until(deadline)
			if remain <= 0 {
				t.Fatalf("timeout reading output")
			}
			select {
			case l := <-lines:
				m := map[string]any{}
				if err := json.Unmarshal([]byte(strings.TrimSpace(l)), &m); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				return m
			case <-time.After(remain):
				t.Fatalf("timeout reading output")
				return nil
			}
		}
	}

	readResponse := func(id float64) map[string]any {
		for {
			m := readLine()
			if m["id"] == nil {
				continue
			}
			mid, ok := m["id"].(float64)
			if !ok || mid != id {
				continue
			}
			return m
		}
	}

	write(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"client": map[string]any{"name": "test", "version": "0.0.1"}, "protocolVersion": "1.0"}})
	_ = readResponse(1)

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "session.create",
		"params": map[string]any{
			"workspacePath": workspace,
			"options": map[string]any{
				"executionMode":         "plan",
				"allowedTools":          []any{"bash"},
				"requireApprovalDigest": false,
			},
		},
	})
	resp := readResponse(2)
	resObj, _ := resp["result"].(map[string]any)
	sid, _ := resObj["sessionID"].(string)
	if strings.TrimSpace(sid) == "" {
		t.Fatalf("missing sessionID: %v", resp)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tool.execute",
		"params": map[string]any{
			"sessionID": sid,
			"call": map[string]any{
				"id":         "c_plan_1",
				"tool":       "bash",
				"parameters": map[string]any{"command": "Write-Output hi"},
			},
		},
	})
	execResp := readResponse(3)
	if execResp["error"] == nil {
		t.Fatalf("expected error, got: %v", execResp)
	}
	errObj, _ := execResp["error"].(map[string]any)
	if int(errObj["code"].(float64)) != -32003 {
		t.Fatalf("unexpected error: %v", execResp)
	}
}

func TestSessionInfoExposesAccessAndApprovalDerivedFromLegacyFields(t *testing.T) {
	workspace := t.TempDir()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	srv, err := NewServer(Options{
		Transport:             "stdio",
		DefaultWorkspacePath:  workspace,
		DefaultAllowedTools:   []string{"read"},
		DefaultSandboxMode:    "full_access",
		RequireApprovalDigest: false,
	}, inR, outW, io.Discard, toolapiimpl.NewServices())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()
	defer func() {
		cancel()
		_ = inW.Close()
		_ = outW.Close()
		<-done
	}()

	rd := bufio.NewReader(outR)
	lines := make(chan string, 64)
	go func() {
		for {
			l, err := rd.ReadString('\n')
			if err != nil {
				return
			}
			lines <- l
		}
	}()
	write := func(obj any) {
		b, _ := json.Marshal(obj)
		_, _ = inW.Write(append(b, '\n'))
	}
	readLine := func() map[string]any {
		deadline := time.Now().Add(5 * time.Second)
		for {
			remain := time.Until(deadline)
			if remain <= 0 {
				t.Fatalf("timeout reading output")
			}
			select {
			case l := <-lines:
				m := map[string]any{}
				if err := json.Unmarshal([]byte(strings.TrimSpace(l)), &m); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				return m
			case <-time.After(remain):
				t.Fatalf("timeout reading output")
				return nil
			}
		}
	}
	readResponse := func(id float64) map[string]any {
		for {
			m := readLine()
			if m["id"] == nil {
				continue
			}
			mid, ok := m["id"].(float64)
			if !ok || mid != id {
				continue
			}
			return m
		}
	}

	write(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"client": map[string]any{"name": "test", "version": "0.0.1"}, "protocolVersion": "1.0"}})
	_ = readResponse(1)

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "session.create",
		"params": map[string]any{
			"workspacePath": workspace,
			"options": map[string]any{
				"allowedTools": []any{"read"},
			},
		},
	})
	resp := readResponse(2)
	result, _ := resp["result"].(map[string]any)
	sessionID, _ := result["sessionID"].(string)
	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "session.get",
		"params":  map[string]any{"sessionID": sessionID},
	})
	sessionResp := readResponse(3)
	sessionResult, _ := sessionResp["result"].(map[string]any)
	sessionInfo, _ := sessionResult["session"].(map[string]any)
	metadata, _ := sessionInfo["metadata"].(map[string]any)
	accessMode, _ := metadata["access_mode"].(string)
	approvalMode, _ := metadata["approval_mode"].(string)
	sandboxMode, _ := metadata["sandbox_mode"].(string)
	if accessMode != "danger-full-access" {
		t.Fatalf("accessMode=%q, want danger-full-access", accessMode)
	}
	if approvalMode != "on-failure" {
		t.Fatalf("approvalMode=%q, want on-failure", approvalMode)
	}
	if sandboxMode != "full_access" {
		t.Fatalf("sandboxMode=%q, want full_access", sandboxMode)
	}
}

func TestToolExecute_MaxConcurrentAndCancel(t *testing.T) {
	workspace := t.TempDir()
	p := filepath.Join(workspace, "a.txt")
	if err := os.WriteFile(p, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	srv, err := NewServer(Options{
		Transport:             "stdio",
		DefaultWorkspacePath:  workspace,
		DefaultAllowedTools:   []string{"bash", "read"},
		DefaultSandboxMode:    "full_access",
		RequireApprovalDigest: false,
	}, inR, outW, io.Discard, toolapiimpl.NewServices())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()
	defer func() {
		cancel()
		_ = inW.Close()
		_ = outW.Close()
		<-done
	}()

	rd := bufio.NewReader(outR)
	lines := make(chan string, 64)
	go func() {
		for {
			l, err := rd.ReadString('\n')
			if err != nil {
				return
			}
			lines <- l
		}
	}()
	write := func(obj any) {
		b, _ := json.Marshal(obj)
		_, _ = inW.Write(append(b, '\n'))
	}

	readLine := func() map[string]any {
		deadline := time.Now().Add(10 * time.Second)
		for {
			remain := time.Until(deadline)
			if remain <= 0 {
				t.Fatalf("timeout reading output")
			}
			select {
			case l := <-lines:
				m := map[string]any{}
				if err := json.Unmarshal([]byte(strings.TrimSpace(l)), &m); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				return m
			case <-time.After(remain):
				t.Fatalf("timeout reading output")
				return nil
			}
		}
	}

	readResponse := func(id float64) map[string]any {
		for {
			m := readLine()
			if m["id"] == nil {
				continue
			}
			mid, ok := m["id"].(float64)
			if !ok || mid != id {
				continue
			}
			return m
		}
	}

	readNotification := func() map[string]any {
		for {
			m := readLine()
			if m["method"] != "event" {
				continue
			}
			return m
		}
	}

	write(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"client": map[string]any{"name": "test", "version": "0.0.1"}, "protocolVersion": "1.0"}})
	_ = readResponse(1)

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "session.create",
		"params": map[string]any{
			"workspacePath": workspace,
			"options": map[string]any{
				"allowedTools":           []any{"bash", "read"},
				"requireApprovalDigest":  false,
				"maxConcurrentToolCalls": 1,
				"executionMode":          "auto",
				"sandboxMode":            "full_access",
				"trustedWorkspace":       true,
				"confirmPolicyID":        "",
			},
		},
	})
	resp := readResponse(2)
	resObj, _ := resp["result"].(map[string]any)
	sid, _ := resObj["sessionID"].(string)
	if strings.TrimSpace(sid) == "" {
		t.Fatalf("missing sessionID: %v", resp)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tool.execute",
		"params": map[string]any{
			"sessionID": sid,
			"call": map[string]any{
				"id":         "c_sleep",
				"tool":       "bash",
				"parameters": map[string]any{"command": "Start-Sleep -Seconds 5"},
			},
		},
	})
	for i := 0; i < 3; i++ {
		_ = readNotification()
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tool.execute",
		"params": map[string]any{
			"sessionID": sid,
			"call": map[string]any{
				"id":         "c_read",
				"tool":       "read",
				"parameters": map[string]any{"path": p},
			},
		},
	})
	tooMany := readResponse(4)
	if tooMany["error"] == nil {
		t.Fatalf("expected error, got: %v", tooMany)
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tool.cancel",
		"params":  map[string]any{"sessionID": sid, "callID": "c_sleep"},
	})
	cancelResp := readResponse(5)
	cancelRes, _ := cancelResp["result"].(map[string]any)
	ok, _ := cancelRes["ok"].(bool)
	if !ok {
		t.Fatalf("expected ok=true, got: %v", cancelResp)
	}

	execResp := readResponse(3)
	if errObj, ok := execResp["error"].(map[string]any); ok && errObj != nil {
		if got, _ := errObj["message"].(string); got != "ConfirmationRequired" {
			t.Fatalf("expected ConfirmationRequired, got: %v", execResp)
		}
		return
	}
	res, _ := execResp["result"].(map[string]any)
	status, _ := res["status"].(string)
	if status == "" {
		t.Fatalf("missing status: %v", execResp)
	}
	if status != "error" {
		t.Fatalf("expected status=error, got: %v", execResp)
	}
}
