package client_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/eosaios/eos/pkg/coreapi"
	sidecarclient "github.com/eosaios/eos/pkg/coreapi/sidecar/client"
	"github.com/eosaios/eos/pkg/protocol/jsonrpc"
)

// packagedCoreBinary is the vendored sidecar that `go run .` actually loads.
const packagedCoreBinary = `C:/home/eos/eos-cli/pkg/coreapi/sidecar/binaries/x86_64-pc-windows-gnu/eos-core.exe`

// TestE2EVendoredFileMkdirRelativePathApproval drives the exact sidecar that
// `go run .` resolves, proving the Rust relative-path fix is packaged and that
// a relative file_mkdir under workspace-write succeeds after approval instead
// of stalling on "outside all writable roots".
func TestE2EVendoredFileMkdirRelativePathApproval(t *testing.T) {
	if _, err := os.Stat(packagedCoreBinary); err != nil {
		t.Skipf("packaged sidecar not found at %s: %v", packagedCoreBinary, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tmpDir := t.TempDir()

	client, err := sidecarclient.Start(ctx, sidecarclient.Options{
		BinaryPath: packagedCoreBinary,
		Env: map[string]string{
			"EOS_SANDBOX_MODE":           "workspace-write",
			"EOS_WORKSPACE_ROOT":         tmpDir,
			"EOS_SANDBOX_WORKSPACE_ROOT": tmpDir,
		},
	})
	if err != nil {
		t.Fatalf("sidecarclient.Start() error = %v", err)
	}
	defer client.Close()

	sess, err := client.Engine().Sessions().Create(ctx, coreapi.CreateSessionRequest{
		WorkspaceRoot: tmpDir,
		Title:         "e2e vendored mkdir",
	})
	if err != nil {
		t.Fatalf("Sessions().Create() error = %v", err)
	}

	var toolResult coreapi.ToolResult
	if err := client.Process().Call(ctx, jsonrpc.MethodToolExecute, map[string]any{
		"name":           "file_mkdir",
		"args":           map[string]any{"path": "test"},
		"workspace_root": tmpDir,
		"session_id":     sess.ID,
		"request_id":     "req_mkdir_v",
	}, &toolResult); err != nil {
		t.Fatalf("tool/execute error = %v", err)
	}
	if toolResult.Status != "approval_required" {
		t.Fatalf("tool/execute status = %q, want approval_required (display=%q)", toolResult.Status, toolResult.Display)
	}
	var pending struct {
		ApprovalID string `json:"approval_id"`
	}
	if err := json.Unmarshal(toolResult.Output, &pending); err != nil || pending.ApprovalID == "" {
		t.Fatalf("missing approval_id: %s", string(toolResult.Output))
	}

	var approveResult coreapi.ToolResult
	if err := client.Process().Call(ctx, jsonrpc.MethodApprovalRespond, map[string]any{
		"approval_id": pending.ApprovalID,
		"decision":    "accept",
	}, &approveResult); err != nil {
		t.Fatalf("approval/respond error = %v", err)
	}
	if approveResult.Status != "ok" {
		t.Fatalf("approval/respond status = %q, want ok (display=%q output=%s)",
			approveResult.Status, approveResult.Display, string(approveResult.Output))
	}

	info, err := os.Stat(filepath.Join(tmpDir, "test"))
	if err != nil {
		t.Fatalf("expected directory created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("test is not a directory")
	}
}
