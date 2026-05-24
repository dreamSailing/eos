package sandbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceWriteAllowsOnlyWorkspacePaths(t *testing.T) {
	root := t.TempDir()
	policy := DefaultPolicy(root)

	ok, err := policy.AllowsWrite(filepath.Join(root, "notes.txt"))
	if err != nil {
		t.Fatalf("AllowsWrite() error = %v", err)
	}
	if !ok {
		t.Fatal("workspace file should be writable")
	}

	ok, err = policy.AllowsWrite(filepath.Join(filepath.Dir(root), "outside.txt"))
	if err != nil {
		t.Fatalf("AllowsWrite() outside error = %v", err)
	}
	if ok {
		t.Fatal("outside file should not be writable")
	}
}

func TestReadOnlyRejectsWrites(t *testing.T) {
	policy := DefaultPolicy(t.TempDir())
	policy.Mode = ModeReadOnly
	ok, err := policy.AllowsWrite(filepath.Join(policy.WorkspaceRoot, "notes.txt"))
	if err != nil {
		t.Fatalf("AllowsWrite() error = %v", err)
	}
	if ok {
		t.Fatal("read-only policy allowed a write")
	}
}

func TestDangerFullAccessAllowsWrites(t *testing.T) {
	policy := Policy{Mode: ModeDangerFullAccess}
	ok, err := policy.AllowsWrite(filepath.Join(t.TempDir(), "anywhere.txt"))
	if err != nil {
		t.Fatalf("AllowsWrite() error = %v", err)
	}
	if !ok {
		t.Fatal("danger-full-access should allow writes")
	}
}

func TestAllowedCommandPrefixes(t *testing.T) {
	policy := DefaultPolicy(t.TempDir())
	policy.AllowedCommandPrefixes = []string{"go test", "rg "}

	if !policy.AllowsCommand([]string{"go", "test", "./..."}) {
		t.Fatal("expected go test command to be allowed")
	}
	if policy.AllowsCommand([]string{"go", "testify", "./..."}) {
		t.Fatal("command prefix should require a token boundary")
	}
	if !policy.AllowsCommand([]string{"RG", "pattern"}) {
		t.Fatal("command prefix matching should be case-insensitive")
	}
	if policy.AllowsCommand([]string{"powershell", "-Command", "Remove-Item"}) {
		t.Fatal("unexpected dangerous command allowed")
	}
}

func TestReadOnlyRejectsCommandsUnlessExplicitlyAllowed(t *testing.T) {
	policy := DefaultPolicy(t.TempDir())
	policy.Mode = ModeReadOnly

	if policy.AllowsCommand([]string{"bash", "-lc", "echo hi"}) {
		t.Fatal("read-only policy should block command execution by default")
	}
	policy.AllowedCommandPrefixes = []string{"bash -lc git status"}
	if !policy.AllowsCommand([]string{"bash", "-lc", "git status --short"}) {
		t.Fatal("read-only policy should allow explicit safe command prefixes")
	}
}

func TestGuardedRunnerBlocksDisallowedCommand(t *testing.T) {
	called := false
	runner := GuardedRunner{
		Backend: DetectBackendForOS("windows"),
		Exec: func(command []string, policy Policy) Result {
			called = true
			return Result{Stdout: "ran"}
		},
	}
	policy := DefaultPolicy(t.TempDir())
	policy.Mode = ModeReadOnly

	result := runner.Run([]string{"bash", "-lc", "echo hi"}, policy)
	if result.Err == nil {
		t.Fatal("Run() error = nil, want blocked command error")
	}
	if result.ExitCode != 126 {
		t.Fatalf("ExitCode=%d, want 126", result.ExitCode)
	}
	if !result.Backend.Degraded {
		t.Fatal("Run() should surface backend degraded status")
	}
	if called {
		t.Fatal("Exec should not be called for blocked command")
	}
}

func TestWorkspaceWriteCommandPolicyBlocksOutsideRedirection(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	policy := DefaultPolicy(root)

	reason := policy.CommandViolation([]string{"bash", "-lc", "echo hi > " + quotePath(outside)})
	if reason == "" {
		t.Fatal("CommandViolation() = empty, want outside redirection blocked")
	}
	if !strings.Contains(reason, "outside workspace") {
		t.Fatalf("reason=%q, want outside workspace", reason)
	}
}

func TestWorkspaceWriteCommandPolicyAllowsWorkspaceRedirection(t *testing.T) {
	policy := DefaultPolicy(t.TempDir())
	if reason := policy.CommandViolation([]string{"bash", "-lc", "echo hi > notes.txt"}); reason != "" {
		t.Fatalf("CommandViolation()=%q, want allowed workspace redirection", reason)
	}
}

func TestWorkspaceWriteCommandPolicyBlocksOutsideMutatingTarget(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	policy := DefaultPolicy(root)

	reason := policy.CommandViolation([]string{"bash", "-lc", "touch " + quotePath(outside)})
	if reason == "" {
		t.Fatal("CommandViolation(touch outside) = empty, want blocked")
	}
	reason = policy.CommandViolation([]string{"bash", "-lc", "cp local.txt " + quotePath(outside)})
	if reason == "" {
		t.Fatal("CommandViolation(cp outside) = empty, want blocked")
	}
	reason = policy.CommandViolation([]string{"bash", "-lc", "gofmt -w " + quotePath(outside)})
	if reason == "" {
		t.Fatal("CommandViolation(gofmt outside) = empty, want blocked")
	}
}

func TestWorkspaceWriteCommandPolicyBlocksPowershellOutsideMutatingTarget(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	policy := DefaultPolicy(root)

	reason := policy.CommandViolation([]string{"powershell", "-Command", "New-Item -Path " + quotePath(outside)})
	if reason == "" {
		t.Fatal("CommandViolation(New-Item outside) = empty, want blocked")
	}
	if !strings.Contains(reason, "outside workspace") {
		t.Fatalf("reason=%q, want outside workspace", reason)
	}
}

func TestWorkspaceWriteCommandPolicyAllowsOutsideRead(t *testing.T) {
	policy := DefaultPolicy(t.TempDir())
	if reason := policy.CommandViolation([]string{"bash", "-lc", "cat " + quotePath(filepath.Join(filepath.Dir(policy.WorkspaceRoot), "outside.txt"))}); reason != "" {
		t.Fatalf("CommandViolation(cat outside)=%q, want outside reads allowed by command broker", reason)
	}
}

func TestGuardedRunnerBlocksWorkspaceWriteOutsideTarget(t *testing.T) {
	called := false
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	runner := GuardedRunner{
		Backend: DetectBackendForOS("windows"),
		Exec: func(command []string, policy Policy) Result {
			called = true
			return Result{Stdout: "ran"}
		},
	}
	result := runner.Run([]string{"bash", "-lc", "echo hi > " + quotePath(outside)}, DefaultPolicy(root))
	if result.Err == nil {
		t.Fatal("Run() error = nil, want workspace-write boundary error")
	}
	if !strings.Contains(result.Stderr, "outside workspace") {
		t.Fatalf("Stderr=%q, want outside workspace", result.Stderr)
	}
	if called {
		t.Fatal("Exec should not be called for outside workspace target")
	}
}

func TestWorkspaceWriteBlocksGlobalSystemChangeUnlessDangerFullAccess(t *testing.T) {
	command := []string{"bash", "-lc", "npm install -g typescript"}
	policy := DefaultPolicy(t.TempDir())
	reason := policy.CommandViolation(command)
	if !strings.Contains(reason, "global system changes") {
		t.Fatalf("workspace-write should block global system change, reason=%q", reason)
	}

	policy.Mode = ModeDangerFullAccess
	if reason := policy.CommandViolation(command); reason != "" {
		t.Fatalf("danger-full-access should allow explicit policy override, got %q", reason)
	}
}

func TestGuardedRunnerPreflightSurfacesVisibleDegradedBackend(t *testing.T) {
	policy := DefaultPolicy(t.TempDir())
	policy.AllowedCommandPrefixes = []string{"bash -lc git status"}
	result := GuardedRunner{Backend: DetectBackendForOS("windows")}.Run(
		[]string{"bash", "-lc", "git status --short"},
		policy,
	)
	if result.Err != nil {
		t.Fatalf("Run() error = %v, want successful preflight", result.Err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode=%d, want 0", result.ExitCode)
	}
	if result.Backend.GOOS != "windows" {
		t.Fatalf("GOOS=%q, want windows", result.Backend.GOOS)
	}
	if result.Backend.Enforced {
		t.Fatal("backend should not pretend sandbox enforcement is wired")
	}
	if !result.Backend.Degraded {
		t.Fatal("backend should report degraded status until OS enforcement is wired")
	}
	if result.Backend.Reason == "" {
		t.Fatal("degraded backend should carry a reason")
	}
}

func TestWindowsBackendIsVisibleDegradedStatus(t *testing.T) {
	status := DetectBackendForOS("windows")
	if status.GOOS != "windows" {
		t.Fatalf("GOOS=%q, want windows", status.GOOS)
	}
	if !status.Degraded {
		t.Fatal("windows backend should report explicit degraded status until OS enforcement is wired")
	}
	if status.Reason == "" {
		t.Fatal("degraded status should carry a reason")
	}
}

func TestReadOnlyRejectsWriteViaCommandViolation(t *testing.T) {
	root := t.TempDir()
	policy := DefaultPolicy(root)
	policy.Mode = ModeReadOnly

	// read-only 模式下，任何写命令都应被 CommandViolation 拦截
	reason := policy.CommandViolation([]string{"bash", "-lc", "echo data > file.txt"})
	if reason == "" {
		t.Fatal("CommandViolation() = empty, want read-only blocks write command")
	}

	// touch 写 workspace 内文件也应被拒绝
	reason = policy.CommandViolation([]string{"bash", "-lc", "touch " + quotePath(filepath.Join(root, "new.txt"))})
	if reason == "" {
		t.Fatal("CommandViolation() = empty, want read-only blocks touch")
	}

	// cp 复制文件也应被拒绝
	reason = policy.CommandViolation([]string{"bash", "-lc", "cp src.txt " + quotePath(filepath.Join(root, "dst.txt"))})
	if reason == "" {
		t.Fatal("CommandViolation() = empty, want read-only blocks cp")
	}
}

func TestReadOnlyRejectsWriteViaGuardedRunner(t *testing.T) {
	called := false
	root := t.TempDir()
	runner := GuardedRunner{
		Backend: DetectBackendForOS("windows"),
		Exec: func(command []string, policy Policy) Result {
			called = true
			return Result{Stdout: "ran"}
		},
	}
	policy := DefaultPolicy(root)
	policy.Mode = ModeReadOnly

	// 尝试写入 workspace 内文件
	result := runner.Run([]string{"bash", "-lc", "echo hi > notes.txt"}, policy)
	if result.Err == nil {
		t.Fatal("Run() error = nil, want read-only write blocked")
	}
	if result.ExitCode != 126 {
		t.Fatalf("ExitCode=%d, want 126", result.ExitCode)
	}
	if called {
		t.Fatal("Exec should not be called for blocked write")
	}
}

func TestWorkspaceWriteOutsideFailsWithNestedPath(t *testing.T) {
	called := false
	root := t.TempDir()
	// 构造一个嵌套的越界路径
	outside := filepath.Join(filepath.Dir(root), "sub", "outside.txt")
	runner := GuardedRunner{
		Backend: DetectBackendForOS("windows"),
		Exec: func(command []string, policy Policy) Result {
			called = true
			return Result{Stdout: "ran"}
		},
	}
	result := runner.Run([]string{"bash", "-lc", "mkdir -p " + quotePath(filepath.Dir(outside)) + " && echo hi > " + quotePath(outside)}, DefaultPolicy(root))
	if result.Err == nil {
		t.Fatal("Run() error = nil, want nested outside write blocked")
	}
	if !strings.Contains(result.Stderr, "outside workspace") {
		t.Fatalf("Stderr=%q, want outside workspace", result.Stderr)
	}
	if called {
		t.Fatal("Exec should not be called for nested outside write")
	}
}

func TestWorkspaceWriteOutsideFailsWithCpOutside(t *testing.T) {
	called := false
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "stolen.txt")
	runner := GuardedRunner{
		Backend: DetectBackendForOS("windows"),
		Exec: func(command []string, policy Policy) Result {
			called = true
			return Result{Stdout: "ran"}
		},
	}
	result := runner.Run([]string{"bash", "-lc", "cp secrets.txt " + quotePath(outside)}, DefaultPolicy(root))
	if result.Err == nil {
		t.Fatal("Run() error = nil, want cp outside blocked")
	}
	if !strings.Contains(result.Stderr, "outside workspace") {
		t.Fatalf("Stderr=%q, want outside workspace", result.Stderr)
	}
	if called {
		t.Fatal("Exec should not be called for cp outside")
	}
}

func TestWorkspaceWriteOutsideFailsWithMvOutside(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "moved.txt")
	policy := DefaultPolicy(root)

	reason := policy.CommandViolation([]string{"bash", "-lc", "mv internal.txt " + quotePath(outside)})
	if reason == "" {
		t.Fatal("CommandViolation(mv outside) = empty, want blocked")
	}
	if !strings.Contains(reason, "outside workspace") {
		t.Fatalf("reason=%q, want outside workspace", reason)
	}
}

func TestWorkspaceWriteOutsideFailsWithSedInPlace(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "config.txt")
	policy := DefaultPolicy(root)

	reason := policy.CommandViolation([]string{"bash", "-lc", "sed -i 's/foo/bar/' " + quotePath(outside)})
	if reason == "" {
		t.Fatal("CommandViolation(sed -i outside) = empty, want blocked")
	}
}

func TestDangerFullAccessExplicitlyAllowsWriteAnyPath(t *testing.T) {
	policy := Policy{Mode: ModeDangerFullAccess}

	// 任意路径写入都应被允许
	paths := []string{
		"/etc/passwd",
		"C:\\Windows\\System32\\config.txt",
		"/tmp/outside.txt",
		filepath.Join(t.TempDir(), "anywhere.txt"),
	}
	for _, path := range paths {
		ok, err := policy.AllowsWrite(path)
		if err != nil {
			t.Fatalf("AllowsWrite(%q) error = %v", path, err)
		}
		if !ok {
			t.Fatalf("AllowsWrite(%q) = false, want true for danger-full-access", path)
		}
	}
}

func TestDangerFullAccessExplicitlyAllowsAllCommands(t *testing.T) {
	policy := Policy{Mode: ModeDangerFullAccess}

	dangerousCommands := [][]string{
		{"bash", "-lc", "rm -rf /"},
		{"bash", "-lc", "npm install -g typescript"},
		{"bash", "-lc", "sudo apt install vim"},
		{"bash", "-lc", "echo secret > /etc/shadow"},
		{"powershell", "-Command", "Remove-Item -Recurse -Force C:\\temp"},
	}
	for _, cmd := range dangerousCommands {
		if !policy.AllowsCommand(cmd) {
			t.Fatalf("AllowsCommand(%v) = false, want true for danger-full-access", cmd)
		}
		reason := policy.CommandViolation(cmd)
		if reason != "" {
			t.Fatalf("CommandViolation(%v) = %q, want empty for danger-full-access", cmd, reason)
		}
	}
}

func TestDangerFullAccessAllowsWriteViaGuardedRunner(t *testing.T) {
	called := false
	runner := GuardedRunner{
		Backend: DetectBackendForOS("windows"),
		Exec: func(command []string, policy Policy) Result {
			called = true
			return Result{Stdout: "executed"}
		},
	}
	policy := Policy{Mode: ModeDangerFullAccess}

	result := runner.Run([]string{"bash", "-lc", "echo hi > /tmp/test.txt"}, policy)
	if result.Err != nil {
		t.Fatalf("Run() error = %v, want nil for danger-full-access", result.Err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode=%d, want 0", result.ExitCode)
	}
	if !called {
		t.Fatal("Exec should be called for danger-full-access allowed command")
	}
}

func TestAllBackendStatusesReportDegradedWithReasonAndUnsupportedCapabilities(t *testing.T) {
	backends := []struct {
		goos            string
		wantBackend   string
		wantReason    string
		wantUnsupported []string
	}{
		{
			goos:            "linux",
			wantBackend:   "bubblewrap-or-landlock",
			wantReason:    "backend probing not wired yet",
			wantUnsupported: []string{"seccomp-filter", "namespace-isolation"},
		},
		{
			goos:            "darwin",
			wantBackend:   "seatbelt",
			wantReason:    "backend probing not wired yet",
			wantUnsupported: []string{"seatbelt-profile", "filesystem-tampering-detection"},
		},
		{
			goos:            "windows",
			wantBackend:   "path-broker",
			wantReason:    "restricted token/job object backend not wired yet",
			wantUnsupported: []string{"restricted-token", "job-object", "path-broker-enforcement"},
		},
		{
			goos:            "freebsd",
			wantBackend:   "none",
			wantReason:    "unsupported OS",
			wantUnsupported: []string{"all-sandbox-capabilities"},
		},
	}

	for _, tc := range backends {
		t.Run(tc.goos, func(t *testing.T) {
			status := DetectBackendForOS(tc.goos)
			if status.GOOS != tc.goos {
				t.Errorf("GOOS=%q, want %q", status.GOOS, tc.goos)
			}
			if status.Backend != tc.wantBackend {
				t.Errorf("Backend=%q, want %q", status.Backend, tc.wantBackend)
			}
			if status.Enforced {
				t.Error("Enforced should be false until OS-level sandbox is wired")
			}
			if !status.Degraded {
				t.Error("Degraded should be true until OS-level sandbox is wired")
			}
			if status.Reason != tc.wantReason {
				t.Errorf("Reason=%q, want %q", status.Reason, tc.wantReason)
			}
			if len(status.UnsupportedCapabilities) == 0 {
				t.Fatal("UnsupportedCapabilities should not be empty for degraded backend")
			}
			for _, want := range tc.wantUnsupported {
				found := false
				for _, got := range status.UnsupportedCapabilities {
					if got == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("UnsupportedCapabilities missing %q, got %v", want, status.UnsupportedCapabilities)
				}
			}
		})
	}
}

func TestWindowsDegradedStatusIsObservableViaJSONRPC(t *testing.T) {
	status := DetectBackendForOS("windows")

	// 验证 degraded 状态可通过 JSON-RPC sandbox/backend_status 暴露
	if !status.Degraded {
		t.Fatal("Windows backend must report degraded=true for observability")
	}
	if status.Reason == "" {
		t.Fatal("Windows degraded status must carry a reason for observability")
	}
	if len(status.UnsupportedCapabilities) == 0 {
		t.Fatal("Windows degraded status must list unsupported_capabilities for observability")
	}

	// 验证 BackendStatus 序列化为 JSON 后字段完整
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if _, ok := decoded["degraded"]; !ok {
		t.Error("JSON output missing 'degraded' field")
	}
	if _, ok := decoded["reason"]; !ok {
		t.Error("JSON output missing 'reason' field")
	}
	if _, ok := decoded["unsupported_capabilities"]; !ok {
		t.Error("JSON output missing 'unsupported_capabilities' field")
	}
	if _, ok := decoded["backend"]; !ok {
		t.Error("JSON output missing 'backend' field")
	}
	if _, ok := decoded["goos"]; !ok {
		t.Error("JSON output missing 'goos' field")
	}
}

func TestGuardedRunnerAlwaysAttachesBackendStatusToResult(t *testing.T) {
	runner := GuardedRunner{
		Backend: DetectBackendForOS("linux"),
		Exec: func(command []string, policy Policy) Result {
			return Result{Stdout: "ok", ExitCode: 0}
		},
	}
	policy := DefaultPolicy(t.TempDir())

	// 成功的命令
	result := runner.Run([]string{"bash", "-lc", "echo ok"}, policy)
	if result.Backend.GOOS != "linux" {
		t.Errorf("success result Backend.GOOS=%q, want linux", result.Backend.GOOS)
	}
	if !result.Backend.Degraded {
		t.Error("success result should carry degraded status")
	}

	// 失败的命令（command violation）
	policy.Mode = ModeReadOnly
	result = runner.Run([]string{"bash", "-lc", "echo hi > file.txt"}, policy)
	if result.Backend.GOOS != "linux" {
		t.Errorf("blocked result Backend.GOOS=%q, want linux", result.Backend.GOOS)
	}
	if !result.Backend.Degraded {
		t.Error("blocked result should carry degraded status")
	}
}

func quotePath(path string) string {
	if strings.ContainsAny(path, " \t") {
		return `"` + path + `"`
	}
	if os.PathSeparator == '\\' {
		return strings.ReplaceAll(path, `\`, `/`)
	}
	return path
}
