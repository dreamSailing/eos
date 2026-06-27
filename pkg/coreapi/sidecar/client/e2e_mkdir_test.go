package client_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dreamSailing/eos/pkg/coreapi"
	sidecarclient "github.com/dreamSailing/eos/pkg/coreapi/sidecar/client"
	"github.com/dreamSailing/eos/pkg/protocol/jsonrpc"
)

// localCoreBinary points at a locally-built eos-core sidecar so this test can
// exercise sandbox/approval changes without a signed release binary.
const localCoreBinary = `C:/home/eos/eos-core-rs/target/debug/eos-core.exe`

// TestE2EFileMkdirRelativePathApproval verifies that a relative write path
// (e.g. a model-supplied "test") is resolved against the request workspace root
// and allowed by the workspace-write sandbox after approval, rather than being
// denied as "outside all writable roots". This reproduces the user scenario
// where creating a directory via FileMkdir stalled after approval.
func TestE2EFileMkdirRelativePathApproval(t *testing.T) {
	if _, err := os.Stat(localCoreBinary); err != nil {
		t.Skipf("locally-built sidecar not found at %s: %v", localCoreBinary, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tmpDir := t.TempDir()

	client, err := sidecarclient.Start(ctx, sidecarclient.Options{
		BinaryPath: localCoreBinary,
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
		Title:         "e2e mkdir",
	})
	if err != nil {
		t.Fatalf("Sessions().Create() error = %v", err)
	}

	// Direct tool/execute with a RELATIVE path "test". file_mkdir is medium
	// risk under the default approval policy, so the first call must return
	// approval_required and an approval_id (pending inserted, sandbox not yet
	// checked). workspace_root is supplied so the later approval path can
	// resolve the relative write path.
	var toolResult coreapi.ToolResult
	if err := client.Process().Call(ctx, jsonrpc.MethodToolExecute, map[string]any{
		"name":           "file_mkdir",
		"args":           map[string]any{"path": "test"},
		"workspace_root": tmpDir,
		"session_id":     sess.ID,
		"request_id":     "req_mkdir_1",
	}, &toolResult); err != nil {
		t.Fatalf("tool/execute error = %v", err)
	}

	if toolResult.Status != "approval_required" {
		t.Fatalf("tool/execute status = %q, want approval_required (display=%q output=%s)",
			toolResult.Status, toolResult.Display, string(toolResult.Output))
	}
	var pending struct {
		ApprovalID string `json:"approval_id"`
	}
	if err := json.Unmarshal(toolResult.Output, &pending); err != nil || pending.ApprovalID == "" {
		t.Fatalf("missing approval_id in output: %s (err=%v)", string(toolResult.Output), err)
	}

	// Approve. The sandbox check now runs inside execute_approved; with the
	// relative-path resolution it must allow the write and actually mkdir.
	var approveResult coreapi.ToolResult
	if err := client.Process().Call(ctx, jsonrpc.MethodApprovalRespond, map[string]any{
		"approval_id": pending.ApprovalID,
		"decision":    "accept",
	}, &approveResult); err != nil {
		t.Fatalf("approval/respond error = %v", err)
	}
	if approveResult.Status != "ok" {
		t.Fatalf("approval/respond result status = %q, want ok (display=%q output=%s)",
			approveResult.Status, approveResult.Display, string(approveResult.Output))
	}

	created := filepath.Join(tmpDir, "test")
	info, err := os.Stat(created)
	if err != nil {
		t.Fatalf("expected directory %s to be created: %v", created, err)
	}
	if !info.IsDir() {
		t.Fatalf("%s exists but is not a directory", created)
	}
}
