//go:build legacy

package parity

type ExpectedGap struct {
	Operation string
	Field     string
	Reason    string
}

func LoadExpectedGaps() []ExpectedGap {
	return []ExpectedGap{
		{
			Operation: "state/snapshot",
			Field:     "agents",
			Reason:    "Rust sidecar exposes its own agent registry; legacy snapshot reflects the Go runtime registry",
		},
		{
			Operation: "session/create",
			Field:     "metadata",
			Reason:    "Legacy engine populates metadata with extra fields (model, preview, rounds, summary, tokens); sidecar returns only user-provided metadata",
		},
		{
			Operation: "session/create",
			Field:     "workspace_root",
			Reason:    "Legacy engine normalizes path via filepath.Clean; sidecar may return path as-is",
		},
		{
			Operation: "session/list",
			Field:     "metadata",
			Reason:    "Legacy engine populates metadata with extra fields (model, preview, rounds, summary, tokens); sidecar returns only user-provided metadata",
		},
		{
			Operation: "session/list",
			Field:     "workspace_root",
			Reason:    "Legacy engine normalizes path via filepath.Clean; sidecar may return path as-is",
		},
		{
			Operation: "state/snapshot",
			Field:     "workspaces",
			Reason:    "Legacy engine includes default workspace path from HOME/.eos/workspace; sidecar only includes explicitly remembered workspaces",
		},
		{
			Operation: "state/snapshot",
			Field:     "trusted",
			Reason:    "Legacy engine workspace trust state may differ from sidecar's trust tracking",
		},
		{
			Operation: "state/snapshot",
			Field:     "current_session",
			Reason:    "Legacy state/snapshot includes current_session from foreground workspace; sidecar may not populate it identically",
		},
		{
			Operation: "state/snapshot",
			Field:     "messages",
			Reason:    "Legacy state/snapshot includes messages from current session; sidecar may return empty or different messages",
		},
		{
			Operation: "turn/start",
			Field:     "status",
			Reason:    "Turn status values may differ between Go Runtime internal states and Rust engine states",
		},
		{
			Operation: "approval/respond",
			Field:     "error_presence",
			Reason:    "Legacy engine accepts any approval ID via no-op; sidecar may validate against pending approvals",
		},
		{
			Operation: "turn/interrupt",
			Field:     "error_presence",
			Reason:    "Legacy engine no-op for unknown turn; sidecar may return error for non-existent turn",
		},
	}
}
