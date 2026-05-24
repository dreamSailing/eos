package sandbox

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDetectBackendReturnsCurrentOS(t *testing.T) {
	status := DetectBackend()
	if status.GOOS == "" {
		t.Fatal("DetectBackend() returned empty GOOS")
	}
	known := map[string]bool{"linux": true, "darwin": true, "windows": true}
	if !known[status.GOOS] {
		t.Logf("DetectBackend() GOOS=%q (unrecognized platform, expected degraded)", status.GOOS)
	}
	if !status.Degraded {
		t.Fatal("DetectBackend() should report degraded until OS enforcement is wired")
	}
}

func TestBackendStatusZeroValueIsNotDegraded(t *testing.T) {
	var status BackendStatus
	if status.Degraded {
		t.Fatal("zero-value BackendStatus should have Degraded=false")
	}
	if status.Enforced {
		t.Fatal("zero-value BackendStatus should have Enforced=false")
	}
	if status.GOOS != "" {
		t.Fatalf("zero-value BackendStatus GOOS=%q, want empty", status.GOOS)
	}
	if status.Backend != "" {
		t.Fatalf("zero-value BackendStatus Backend=%q, want empty", status.Backend)
	}
}

func TestBackendStatusAllPlatformsHaveConsistentStructure(t *testing.T) {
	platforms := []string{"linux", "darwin", "windows", "freebsd"}
	for _, goos := range platforms {
		t.Run(goos, func(t *testing.T) {
			status := DetectBackendForOS(goos)
			if status.GOOS != goos {
				t.Errorf("GOOS=%q, want %q", status.GOOS, goos)
			}
			if status.Backend == "" {
				t.Error("Backend should not be empty")
			}
			if status.Enforced {
				t.Error("Enforced must be false until OS-level sandbox is wired")
			}
			if !status.Degraded {
				t.Error("Degraded must be true until OS-level sandbox is wired")
			}
			if status.Reason == "" {
				t.Error("degraded backend must carry a non-empty Reason")
			}
			if len(status.UnsupportedCapabilities) == 0 {
				t.Error("degraded backend must list at least one UnsupportedCapabilities entry")
			}
			for _, cap := range status.UnsupportedCapabilities {
				if cap == "" {
					t.Error("UnsupportedCapabilities must not contain empty strings")
				}
			}
		})
	}
}

func TestBackendStatusJSONRoundTripAllPlatforms(t *testing.T) {
	platforms := []string{"linux", "darwin", "windows", "freebsd"}
	for _, goos := range platforms {
		t.Run(goos, func(t *testing.T) {
			original := DetectBackendForOS(goos)
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			var restored BackendStatus
			if err := json.Unmarshal(data, &restored); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if !reflect.DeepEqual(restored, original) {
				t.Errorf("round-trip mismatch:\n  got:  %+v\n  want: %+v", restored, original)
			}
		})
	}
}

func TestResultJSONSerializationIncludesBackend(t *testing.T) {
	result := Result{
		Stdout:   "hello",
		Stderr:   "warn",
		ExitCode: 0,
		Backend:  DetectBackendForOS("linux"),
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	backend, ok := decoded["Backend"].(map[string]interface{})
	if !ok {
		t.Fatal("Result JSON missing or invalid 'Backend' field")
	}
	if backend["goos"] != "linux" {
		t.Errorf("Backend.goos=%v, want linux", backend["goos"])
	}
	if backend["backend"] != "bubblewrap-or-landlock" {
		t.Errorf("Backend.backend=%v, want bubblewrap-or-landlock", backend["backend"])
	}
}

func TestResultJSONRoundTrip(t *testing.T) {
	original := Result{
		Stdout:   "out",
		Stderr:   "err",
		ExitCode: 42,
		Backend:  DetectBackendForOS("darwin"),
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var restored Result
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if restored.Stdout != original.Stdout {
		t.Errorf("Stdout=%q, want %q", restored.Stdout, original.Stdout)
	}
	if restored.Stderr != original.Stderr {
		t.Errorf("Stderr=%q, want %q", restored.Stderr, original.Stderr)
	}
	if restored.ExitCode != original.ExitCode {
		t.Errorf("ExitCode=%d, want %d", restored.ExitCode, original.ExitCode)
	}
	if !reflect.DeepEqual(restored.Backend, original.Backend) {
		t.Errorf("Backend mismatch: got %+v, want %+v", restored.Backend, original.Backend)
	}
}

func TestPolicyJSONSerialization(t *testing.T) {
	policy := Policy{
		Mode:                   ModeWorkspaceWrite,
		WorkspaceRoot:          "/home/user/project",
		WritableRoots:          []string{"/tmp"},
		Network:                NetworkDeny,
		AllowedCommandPrefixes: []string{"go test", "rg "},
	}
	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded["mode"] != "workspace-write" {
		t.Errorf("mode=%v, want workspace-write", decoded["mode"])
	}
	if decoded["network"] != "deny" {
		t.Errorf("network=%v, want deny", decoded["network"])
	}
	if _, ok := decoded["workspace_root"]; !ok {
		t.Error("JSON output missing 'workspace_root' field")
	}
	if _, ok := decoded["writable_roots"]; !ok {
		t.Error("JSON output missing 'writable_roots' field")
	}
	if _, ok := decoded["allowed_command_prefixes"]; !ok {
		t.Error("JSON output missing 'allowed_command_prefixes' field")
	}
}

func TestPolicyJSONOmitEmptyBehavior(t *testing.T) {
	policy := Policy{Mode: ModeReadOnly, Network: NetworkDeny}
	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if _, ok := decoded["workspace_root"]; ok {
		t.Error("workspace_root should be omitted when empty")
	}
	if _, ok := decoded["writable_roots"]; ok {
		t.Error("writable_roots should be omitted when empty")
	}
	if _, ok := decoded["allowed_command_prefixes"]; ok {
		t.Error("allowed_command_prefixes should be omitted when empty")
	}
}

func TestBackendStatusJSONFieldNamesMatchAPIContract(t *testing.T) {
	status := DetectBackendForOS("windows")
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	expectedFields := []string{"goos", "backend", "enforced", "degraded", "reason", "unsupported_capabilities"}
	for _, field := range expectedFields {
		if _, ok := decoded[field]; !ok {
			t.Errorf("JSON output missing expected field %q", field)
		}
	}
	if _, ok := decoded["goos"].(string); !ok {
		t.Errorf("goos field type = %T, want string", decoded["goos"])
	}
	if _, ok := decoded["backend"].(string); !ok {
		t.Errorf("backend field type = %T, want string", decoded["backend"])
	}
	if _, ok := decoded["enforced"].(bool); !ok {
		t.Errorf("enforced field type = %T, want bool", decoded["enforced"])
	}
	if _, ok := decoded["degraded"].(bool); !ok {
		t.Errorf("degraded field type = %T, want bool", decoded["degraded"])
	}
	if _, ok := decoded["reason"].(string); !ok {
		t.Errorf("reason field type = %T, want string", decoded["reason"])
	}
	if caps, ok := decoded["unsupported_capabilities"].([]interface{}); !ok {
		t.Errorf("unsupported_capabilities field type = %T, want array", decoded["unsupported_capabilities"])
	} else if len(caps) == 0 {
		t.Error("unsupported_capabilities should not be empty for degraded backend")
	}
}

func TestGuardedRunnerEmptyCommandReturnsError(t *testing.T) {
	runner := GuardedRunner{
		Backend: DetectBackendForOS("linux"),
		Exec: func(command []string, policy Policy) Result {
			return Result{Stdout: "should not reach"}
		},
	}
	policy := DefaultPolicy(t.TempDir())

	result := runner.Run(nil, policy)
	if result.Err == nil {
		t.Fatal("Run(nil) error = nil, want error for empty command")
	}
	if result.ExitCode != 2 {
		t.Fatalf("ExitCode=%d, want 2", result.ExitCode)
	}

	result = runner.Run([]string{}, policy)
	if result.Err == nil {
		t.Fatal("Run([]) error = nil, want error for empty command")
	}
	if result.ExitCode != 2 {
		t.Fatalf("ExitCode=%d, want 2", result.ExitCode)
	}

	result = runner.Run([]string{"  "}, policy)
	if result.Err == nil {
		t.Fatal("Run([' ']) error = nil, want error for blank command")
	}
	if result.ExitCode != 2 {
		t.Fatalf("ExitCode=%d, want 2", result.ExitCode)
	}

	if result.Backend.GOOS != "linux" {
		t.Errorf("Backend.GOOS=%q, want linux", result.Backend.GOOS)
	}
}

func TestGuardedRunnerNilExecReturnsSuccessWithBackend(t *testing.T) {
	runner := GuardedRunner{
		Backend: DetectBackendForOS("darwin"),
		Exec:    nil,
	}
	policy := DefaultPolicy(t.TempDir())

	result := runner.Run([]string{"bash", "-lc", "echo ok"}, policy)
	if result.Err != nil {
		t.Fatalf("Run() error = %v, want nil for nil exec", result.Err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode=%d, want 0", result.ExitCode)
	}
	if result.Backend.GOOS != "darwin" {
		t.Errorf("Backend.GOOS=%q, want darwin", result.Backend.GOOS)
	}
	if !result.Backend.Degraded {
		t.Error("nil exec result should still carry degraded backend status")
	}
}

func TestGuardedRunnerBackendFallbackWhenGOOSEmpty(t *testing.T) {
	runner := GuardedRunner{
		Backend: BackendStatus{},
		Exec: func(command []string, policy Policy) Result {
			return Result{Stdout: "ok"}
		},
	}
	policy := DefaultPolicy(t.TempDir())

	result := runner.Run([]string{"bash", "-lc", "echo ok"}, policy)
	if result.Backend.GOOS == "" {
		t.Fatal("fallback should populate Backend.GOOS from DetectBackend()")
	}
	if !result.Backend.Degraded {
		t.Error("fallback backend should report degraded status")
	}
}

func TestNewGuardedRunnerInitializesBackend(t *testing.T) {
	runner := NewGuardedRunner(nil)
	if runner.Backend.GOOS == "" {
		t.Fatal("NewGuardedRunner should initialize Backend from DetectBackend()")
	}
	if !runner.Backend.Degraded {
		t.Error("NewGuardedRunner Backend should be degraded")
	}
	if runner.Exec != nil {
		t.Error("NewGuardedRunner(nil) should set Exec to nil")
	}
}

func TestGuardedRunnerResultPreservesExecOutput(t *testing.T) {
	runner := GuardedRunner{
		Backend: DetectBackendForOS("linux"),
		Exec: func(command []string, policy Policy) Result {
			return Result{Stdout: "hello world", Stderr: "some warn", ExitCode: 7}
		},
	}
	policy := DefaultPolicy(t.TempDir())

	result := runner.Run([]string{"bash", "-lc", "echo hello world"}, policy)
	if result.Stdout != "hello world" {
		t.Errorf("Stdout=%q, want hello world", result.Stdout)
	}
	if result.Stderr != "some warn" {
		t.Errorf("Stderr=%q, want some warn", result.Stderr)
	}
	if result.ExitCode != 7 {
		t.Errorf("ExitCode=%d, want 7", result.ExitCode)
	}
	if result.Err != nil {
		t.Errorf("Err=%v, want nil", result.Err)
	}
	if result.Backend.GOOS != "linux" {
		t.Errorf("Backend.GOOS=%q, want linux", result.Backend.GOOS)
	}
}

func TestGuardedRunnerResultBackendStatusOnBlockedCommand(t *testing.T) {
	platforms := []string{"linux", "darwin", "windows"}
	for _, goos := range platforms {
		t.Run(goos, func(t *testing.T) {
			runner := GuardedRunner{
				Backend: DetectBackendForOS(goos),
				Exec: func(command []string, policy Policy) Result {
					return Result{Stdout: "should not reach"}
				},
			}
			policy := DefaultPolicy(t.TempDir())
			policy.Mode = ModeReadOnly

			result := runner.Run([]string{"bash", "-lc", "echo hi"}, policy)
			if result.Err == nil {
				t.Fatal("blocked command should return error")
			}
			if result.ExitCode != 126 {
				t.Fatalf("ExitCode=%d, want 126", result.ExitCode)
			}
			if result.Backend.GOOS != goos {
				t.Errorf("Backend.GOOS=%q, want %q", result.Backend.GOOS, goos)
			}
			if !result.Backend.Degraded {
				t.Error("blocked result should carry degraded backend status")
			}
			if result.Backend.Backend == "" {
				t.Error("blocked result Backend.Backend should not be empty")
			}
			if len(result.Backend.UnsupportedCapabilities) == 0 {
				t.Error("blocked result should list UnsupportedCapabilities")
			}
		})
	}
}

func TestCrossPlatformPermissionBehaviorMatrix(t *testing.T) {
	workspace := t.TempDir()
	insidePath := filepath.Join(workspace, "notes.txt")
	outsidePath := filepath.Join(filepath.Dir(workspace), "outside.txt")

	platforms := []string{"linux", "darwin", "windows"}

	type writeExpectation struct {
		allowed bool
		hasErr  bool
	}
	type commandExpectation struct {
		allowed        bool
		violationEmpty bool
	}

	matrix := []struct {
		name           string
		mode           Mode
		writeInside    writeExpectation
		writeOutside   writeExpectation
		commandInside  commandExpectation
		commandOutside commandExpectation
	}{
		{
			name:           "read-only",
			mode:           ModeReadOnly,
			writeInside:    writeExpectation{allowed: false, hasErr: false},
			writeOutside:   writeExpectation{allowed: false, hasErr: false},
			commandInside:  commandExpectation{allowed: false, violationEmpty: false},
			commandOutside: commandExpectation{allowed: false, violationEmpty: false},
		},
		{
			name:           "workspace-write",
			mode:           ModeWorkspaceWrite,
			writeInside:    writeExpectation{allowed: true, hasErr: false},
			writeOutside:   writeExpectation{allowed: false, hasErr: false},
			commandInside:  commandExpectation{allowed: true, violationEmpty: true},
			commandOutside: commandExpectation{allowed: true, violationEmpty: false},
		},
		{
			name:           "danger-full-access",
			mode:           ModeDangerFullAccess,
			writeInside:    writeExpectation{allowed: true, hasErr: false},
			writeOutside:   writeExpectation{allowed: true, hasErr: false},
			commandInside:  commandExpectation{allowed: true, violationEmpty: true},
			commandOutside: commandExpectation{allowed: true, violationEmpty: true},
		},
	}

	for _, platform := range platforms {
		t.Run(platform, func(t *testing.T) {
			status := DetectBackendForOS(platform)
			t.Logf("platform=%s backend=%s degraded=%v reason=%q", platform, status.Backend, status.Degraded, status.Reason)
			if !status.Degraded {
				t.Error("all platforms should report degraded until OS enforcement is wired")
			}

			for _, entry := range matrix {
				t.Run(entry.name, func(t *testing.T) {
					policy := Policy{
						Mode:          entry.mode,
						WorkspaceRoot: workspace,
						Network:       NetworkDeny,
					}.Normalized()

					t.Run("write_inside", func(t *testing.T) {
						ok, err := policy.AllowsWrite(insidePath)
						if (err != nil) != entry.writeInside.hasErr {
							t.Errorf("AllowsWrite(inside) err=%v, want hasErr=%v", err, entry.writeInside.hasErr)
						}
						if ok != entry.writeInside.allowed {
							t.Errorf("AllowsWrite(inside)=%v, want %v", ok, entry.writeInside.allowed)
						}
					})

					t.Run("write_outside", func(t *testing.T) {
						ok, err := policy.AllowsWrite(outsidePath)
						if (err != nil) != entry.writeOutside.hasErr {
							t.Errorf("AllowsWrite(outside) err=%v, want hasErr=%v", err, entry.writeOutside.hasErr)
						}
						if ok != entry.writeOutside.allowed {
							t.Errorf("AllowsWrite(outside)=%v, want %v", ok, entry.writeOutside.allowed)
						}
					})

					t.Run("command_inside", func(t *testing.T) {
						cmd := []string{"bash", "-lc", "echo ok > " + quotePath(insidePath)}
						allowed := policy.AllowsCommand(cmd)
						if allowed != entry.commandInside.allowed {
							t.Errorf("AllowsCommand(inside)=%v, want %v", allowed, entry.commandInside.allowed)
						}
						reason := policy.CommandViolation(cmd)
						if (reason == "") != entry.commandInside.violationEmpty {
							t.Errorf("CommandViolation(inside)=%q, want empty=%v", reason, entry.commandInside.violationEmpty)
						}
					})

					t.Run("command_outside", func(t *testing.T) {
						cmd := []string{"bash", "-lc", "echo ok > " + quotePath(outsidePath)}
						allowed := policy.AllowsCommand(cmd)
						if allowed != entry.commandOutside.allowed {
							t.Errorf("AllowsCommand(outside)=%v, want %v", allowed, entry.commandOutside.allowed)
						}
						reason := policy.CommandViolation(cmd)
						if (reason == "") != entry.commandOutside.violationEmpty {
							t.Errorf("CommandViolation(outside)=%q, want empty=%v", reason, entry.commandOutside.violationEmpty)
						}
					})
				})
			}
		})
	}
}

func TestCrossPlatformGuardedRunnerBehaviorMatrix(t *testing.T) {
	workspace := t.TempDir()
	outsidePath := filepath.Join(filepath.Dir(workspace), "outside.txt")
	platforms := []string{"linux", "darwin", "windows"}

	modes := []struct {
		name          string
		mode          Mode
		expectBlocked bool
	}{
		{"read-only", ModeReadOnly, true},
		{"workspace-write-outside", ModeWorkspaceWrite, true},
		{"danger-full-access", ModeDangerFullAccess, false},
	}

	for _, platform := range platforms {
		t.Run(platform, func(t *testing.T) {
			for _, m := range modes {
				t.Run(m.name, func(t *testing.T) {
					called := false
					runner := GuardedRunner{
						Backend: DetectBackendForOS(platform),
						Exec: func(command []string, policy Policy) Result {
							called = true
							return Result{Stdout: "executed"}
						},
					}
					policy := Policy{
						Mode:          m.mode,
						WorkspaceRoot: workspace,
						Network:       NetworkDeny,
					}.Normalized()

					result := runner.Run(
						[]string{"bash", "-lc", "echo hi > " + quotePath(outsidePath)},
						policy,
					)

					if m.expectBlocked {
						if result.Err == nil {
							t.Fatal("Run() error = nil, want blocked")
						}
						if called {
							t.Fatal("Exec should not be called for blocked command")
						}
						if result.Backend.GOOS != platform {
							t.Errorf("Backend.GOOS=%q, want %q", result.Backend.GOOS, platform)
						}
						if !result.Backend.Degraded {
							t.Error("blocked result should carry degraded backend status")
						}
					} else {
						if result.Err != nil {
							t.Fatalf("Run() error = %v, want nil for danger-full-access", result.Err)
						}
						if !called {
							t.Fatal("Exec should be called for danger-full-access")
						}
						if result.Backend.GOOS != platform {
							t.Errorf("Backend.GOOS=%q, want %q", result.Backend.GOOS, platform)
						}
					}
				})
			}
		})
	}
}

func TestNormalizeModeCoversAllVariants(t *testing.T) {
	cases := []struct {
		input string
		want  Mode
	}{
		{"read-only", ModeReadOnly},
		{"readonly", ModeReadOnly},
		{"READ-ONLY", ModeReadOnly},
		{"read_only", ModeReadOnly},
		{"workspace-write", ModeWorkspaceWrite},
		{"workspacewrite", ModeWorkspaceWrite},
		{"workspace", ModeWorkspaceWrite},
		{"sandbox", ModeWorkspaceWrite},
		{"danger-full-access", ModeDangerFullAccess},
		{"dangerfullaccess", ModeDangerFullAccess},
		{"full-access", ModeDangerFullAccess},
		{"fullaccess", ModeDangerFullAccess},
		{"full-access-mode", ModeDangerFullAccess},
		{"", ModeWorkspaceWrite},
		{"unknown-mode", ModeWorkspaceWrite},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := NormalizeMode(tc.input)
			if got != tc.want {
				t.Errorf("NormalizeMode(%q)=%q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestWindowsBackendDoesNotPromiseSameIsolationAsLinuxOrMac(t *testing.T) {
	win := DetectBackendForOS("windows")
	linux := DetectBackendForOS("linux")
	darwin := DetectBackendForOS("darwin")

	if win.Backend == linux.Backend {
		t.Error("Windows and Linux should use different backend identifiers")
	}
	if win.Backend == darwin.Backend {
		t.Error("Windows and Darwin should use different backend identifiers")
	}
	if win.Backend == "none" {
		t.Error("Windows backend should not be 'none'")
	}
	if len(win.UnsupportedCapabilities) == 0 {
		t.Error("Windows should list its own unsupported capabilities")
	}

	backendSet := map[string]bool{win.Backend: true, linux.Backend: true, darwin.Backend: true}
	if len(backendSet) != 3 {
		t.Errorf("expected 3 distinct backend identifiers, got %d: %v", len(backendSet), backendSet)
	}
}
