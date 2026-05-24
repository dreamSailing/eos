package sandbox

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
)

type Mode string

const (
	ModeReadOnly         Mode = "read-only"
	ModeWorkspaceWrite   Mode = "workspace-write"
	ModeDangerFullAccess Mode = "danger-full-access"
)

type NetworkPolicy string

const (
	NetworkDeny  NetworkPolicy = "deny"
	NetworkAllow NetworkPolicy = "allow"
)

type Policy struct {
	Mode                   Mode          `json:"mode"`
	WorkspaceRoot          string        `json:"workspace_root,omitempty"`
	WritableRoots          []string      `json:"writable_roots,omitempty"`
	Network                NetworkPolicy `json:"network"`
	AllowedCommandPrefixes []string      `json:"allowed_command_prefixes,omitempty"`
}

type BackendStatus struct {
	GOOS                   string   `json:"goos"`
	Backend                string   `json:"backend"`
	Enforced               bool     `json:"enforced"`
	Degraded               bool     `json:"degraded"`
	Reason                 string   `json:"reason,omitempty"`
	UnsupportedCapabilities []string `json:"unsupported_capabilities,omitempty"`
}

type Runner interface {
	Run(command []string, policy Policy) Result
}

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
	Backend  BackendStatus
}

func NormalizeMode(mode string) Mode {
	key := strings.ToLower(strings.TrimSpace(mode))
	key = strings.ReplaceAll(key, "_", "-")
	switch key {
	case "read-only", "readonly":
		return ModeReadOnly
	case "danger-full-access", "dangerfullaccess", "full-access", "fullaccess", "full-access-mode":
		return ModeDangerFullAccess
	default:
		return ModeWorkspaceWrite
	}
}

func DefaultPolicy(workspaceRoot string) Policy {
	return Policy{
		Mode:          ModeWorkspaceWrite,
		WorkspaceRoot: strings.TrimSpace(workspaceRoot),
		Network:       NetworkDeny,
	}
}

func (p Policy) Normalized() Policy {
	p.Mode = NormalizeMode(string(p.Mode))
	if strings.TrimSpace(string(p.Network)) == "" {
		p.Network = NetworkDeny
	}
	p.WorkspaceRoot = strings.TrimSpace(p.WorkspaceRoot)
	p.WritableRoots = compactStrings(p.WritableRoots)
	p.AllowedCommandPrefixes = compactStrings(p.AllowedCommandPrefixes)
	return p
}

func (p Policy) AllowsWrite(path string) (bool, error) {
	p = p.Normalized()
	switch p.Mode {
	case ModeDangerFullAccess:
		return true, nil
	case ModeReadOnly:
		return false, nil
	}

	roots := append([]string(nil), p.WritableRoots...)
	if strings.TrimSpace(p.WorkspaceRoot) != "" {
		roots = append([]string{p.WorkspaceRoot}, roots...)
	}
	if len(roots) == 0 {
		return false, errors.New("workspace-write policy requires a workspace root or writable root")
	}
	for _, root := range roots {
		ok, err := pathWithin(root, path)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func (p Policy) AllowsCommand(argv []string) bool {
	p = p.Normalized()
	if p.Mode == ModeDangerFullAccess {
		return true
	}
	if len(p.AllowedCommandPrefixes) == 0 {
		return p.Mode != ModeReadOnly
	}
	joined := strings.Join(argv, " ")
	for _, prefix := range p.AllowedCommandPrefixes {
		if commandPrefixMatches(joined, prefix) {
			return true
		}
	}
	return false
}

func commandPrefixMatches(command string, prefix string) bool {
	command = strings.ToLower(strings.TrimSpace(command))
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if command == "" || prefix == "" {
		return false
	}
	return command == prefix || strings.HasPrefix(command, prefix+" ")
}

func compactStrings(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func DetectBackend() BackendStatus {
	return DetectBackendForOS(runtime.GOOS)
}

func DetectBackendForOS(goos string) BackendStatus {
	switch goos {
	case "linux":
		return BackendStatus{
			GOOS:     goos,
			Backend:  "bubblewrap-or-landlock",
			Enforced: false,
			Degraded: true,
			Reason:   "backend probing not wired yet",
			UnsupportedCapabilities: []string{"seccomp-filter", "namespace-isolation"},
		}
	case "darwin":
		return BackendStatus{
			GOOS:     goos,
			Backend:  "seatbelt",
			Enforced: false,
			Degraded: true,
			Reason:   "backend probing not wired yet",
			UnsupportedCapabilities: []string{"seatbelt-profile", "filesystem-tampering-detection"},
		}
	case "windows":
		return BackendStatus{
			GOOS:     goos,
			Backend:  "path-broker",
			Enforced: false,
			Degraded: true,
			Reason:   "restricted token/job object backend not wired yet",
			UnsupportedCapabilities: []string{"restricted-token", "job-object", "path-broker-enforcement"},
		}
	default:
		return BackendStatus{
			GOOS:     goos,
			Backend:  "none",
			Enforced: false,
			Degraded: true,
			Reason:   "unsupported OS",
			UnsupportedCapabilities: []string{"all-sandbox-capabilities"},
		}
	}
}

func pathWithin(root string, path string) (bool, error) {
	rootAbs, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return false, err
	}
	pathAbs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return false, err
	}
	rootClean := filepath.Clean(rootAbs)
	pathClean := filepath.Clean(pathAbs)
	rel, err := filepath.Rel(rootClean, pathClean)
	if err != nil {
		return false, err
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))), nil
}
