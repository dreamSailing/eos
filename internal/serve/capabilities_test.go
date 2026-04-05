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

	"github.com/dreamSailing/vb-coding/internal/config"
	pluginpkg "github.com/dreamSailing/vb-coding/internal/pkg/plugins"
	toolapiimpl "github.com/dreamSailing/vb-coding/internal/toolapi/impl"
)

type serveTestPlugin struct {
	name string
	desc string
}

func (p *serveTestPlugin) Name() string        { return p.name }
func (p *serveTestPlugin) Description() string { return p.desc }
func (p *serveTestPlugin) Execute(_ context.Context, _ map[string]any) (any, error) {
	return map[string]any{"ok": true}, nil
}

func TestCapabilityListIncludesUnifiedCapabilitiesWhileToolListStaysExecutable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	pluginpkg.DefaultRegistry().Reset()
	t.Cleanup(func() { pluginpkg.DefaultRegistry().Reset() })
	pluginpkg.DefaultRegistry().Register(&serveTestPlugin{name: "echo_plugin", desc: "echo plugin"})

	cfg := config.Config{
		MCP: []config.MCPEntry{
			{Name: "demo", Type: config.MCPTypeStdio, Command: "demo-mcp", Enabled: true},
		},
	}
	if err := config.Save(cfg, config.Path()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	skillDir := filepath.Join(home, ".vb", "skills", "review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	content := "---\nname: review\ndescription: code review helper\n---\n\nbody"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	workspace := t.TempDir()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	srv, err := NewServer(Options{
		Transport:             "stdio",
		DefaultWorkspacePath:  workspace,
		DefaultAllowedTools:   []string{"read", "skills_list", "mcp_status", "echo_plugin"},
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
		_ = <-done
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
			if m["method"] == "event" {
				continue
			}
			mid, _ := m["id"].(float64)
			if mid == id {
				return m
			}
		}
	}
	findByName := func(items []any, name string) map[string]any {
		for _, item := range items {
			entry, _ := item.(map[string]any)
			if got, _ := entry["name"].(string); got == name {
				return entry
			}
		}
		return nil
	}

	write(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"client": map[string]any{"name": "test", "version": "0.0.1"}, "protocolVersion": "1.0"}})
	_ = readResponse(1, 10*time.Second)

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "session.create",
		"params": map[string]any{
			"workspacePath": workspace,
			"options": map[string]any{
				"allowedTools": []any{"read", "skills_list", "mcp_status", "echo_plugin"},
			},
		},
	})
	resp := readResponse(2, 10*time.Second)
	result, _ := resp["result"].(map[string]any)
	sessionID, _ := result["sessionID"].(string)
	if sessionID == "" {
		t.Fatalf("missing sessionID: %v", resp)
	}

	write(map[string]any{"jsonrpc": "2.0", "id": 3, "method": "capability.list", "params": map[string]any{"sessionID": sessionID}})
	capResp := readResponse(3, 10*time.Second)
	capResult, _ := capResp["result"].(map[string]any)
	capabilities, _ := capResult["capabilities"].([]any)
	if got, _ := capResult["mode"].(string); got != "default" {
		t.Fatalf("mode=%v, want default", capResult["mode"])
	}
	modeProfile, _ := capResult["modeProfile"].(map[string]any)
	if got, _ := modeProfile["approvalBehavior"].(string); got != "prompt_mutations" {
		t.Fatalf("approvalBehavior=%v, want prompt_mutations", modeProfile["approvalBehavior"])
	}
	summary, _ := capResult["summary"].(map[string]any)
	if _, ok := summary["blockedByAllow"]; !ok {
		t.Fatalf("summary missing blockedByAllow: %v", summary)
	}
	catalog, _ := capResult["catalog"].([]any)
	if len(catalog) < len(capabilities) {
		t.Fatalf("catalog should include full capability decisions, got %d entries vs %d visible", len(catalog), len(capabilities))
	}
	for _, name := range []string{"read", "duckduckgo_search", "spawn_agent", "skill:review", "echo_plugin", "mcp:demo", "lsp"} {
		if findByName(capabilities, name) == nil {
			t.Fatalf("missing capability %q in %v", name, capabilities)
		}
	}
	if entry := findByName(capabilities, "spawn_agent"); entry == nil || entry["invocable"] != false {
		t.Fatalf("spawn_agent should be capability-only: %v", entry)
	} else {
		access, _ := entry["access"].(map[string]any)
		if got, _ := access["reason"].(string); got != "non_invocable" {
			t.Fatalf("spawn_agent access.reason=%q, want non_invocable", got)
		}
	}

	write(map[string]any{"jsonrpc": "2.0", "id": 4, "method": "tool.list", "params": map[string]any{"sessionID": sessionID}})
	toolResp := readResponse(4, 10*time.Second)
	toolResult, _ := toolResp["result"].(map[string]any)
	toolsAny, _ := toolResult["tools"].([]any)
	toolCatalog, _ := toolResult["catalog"].([]any)
	if findByName(toolsAny, "read") == nil || findByName(toolsAny, "echo_plugin") == nil {
		t.Fatalf("expected executable tools in %v", toolsAny)
	}
	for _, name := range []string{"duckduckgo_search", "spawn_agent", "skill:review", "mcp:demo", "lsp"} {
		if findByName(toolsAny, name) != nil {
			t.Fatalf("%q should not appear in tool.list: %v", name, toolsAny)
		}
	}
	if entry := findByName(toolCatalog, "bash"); entry != nil {
		access, _ := entry["access"].(map[string]any)
		if got, _ := access["reason"].(string); got != "allowed_tools" {
			t.Fatalf("bash access.reason=%q, want allowed_tools", got)
		}
	}
}
