package settings

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
)

// Layer represents the origin of a settings layer
type Layer string

const (
	LayerManaged Layer = "managed" // Highest priority (admin/org policy)
	LayerProject Layer = "project" // .vb/settings.json
	LayerUser    Layer = "user"    // ~/.vb.json (lowest priority)
)

// LoadMerged loads and merges settings from all layers.
// Priority: managed > project > user
func LoadMerged(userPath, projectRoot string) *Settings {
	user := loadLayer(userPath)
	project := loadLayer(filepath.Join(projectRoot, ".vb", "settings.json"))
	managed := loadLayer(filepath.Join(projectRoot, ".vb", "settings.managed.json"))

	// Merge: start with lowest priority, overlay higher
	result := user
	mergeInto(&result, project)
	mergeInto(&result, managed)

	return &result
}

// loadLayer loads a single settings file, returning defaults if not found
func loadLayer(path string) Settings {
	var s Settings
	if path == "" {
		return s
	}
	bs, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if err := json.Unmarshal(bs, &s); err != nil {
		slog.Debug("settings.load_layer.error", "path", path, "error", err)
		return s
	}
	return s
}

// mergeInto overlays src onto dst. Non-zero values in src override dst.
// Slices are concatenated (union), not replaced.
func mergeInto(dst *Settings, src Settings) {
	if src.PlanPromptStyle != "" {
		dst.PlanPromptStyle = src.PlanPromptStyle
	}
	if src.PlanBubbleColor != "" {
		dst.PlanBubbleColor = src.PlanBubbleColor
	}
	if src.AutoContext {
		dst.AutoContext = src.AutoContext
	}
	if src.DesktopNotifications != nil {
		dst.DesktopNotifications = src.DesktopNotifications
	}
	if src.MaxInjectKB > 0 {
		dst.MaxInjectKB = src.MaxInjectKB
	}
	if src.WatchMode != "" {
		dst.WatchMode = src.WatchMode
	}
	if src.WatchDebounceMs > 0 {
		dst.WatchDebounceMs = src.WatchDebounceMs
	}
	if src.PollIntervalSec > 0 {
		dst.PollIntervalSec = src.PollIntervalSec
	}
	if src.Language != "" {
		dst.Language = src.Language
	}
	if src.ActiveWorkspace != "" {
		dst.ActiveWorkspace = src.ActiveWorkspace
	}
	if src.Theme != "" {
		dst.Theme = src.Theme
	}
	if src.Trusted {
		dst.Trusted = src.Trusted
	}
	if src.TrustedAt != "" {
		dst.TrustedAt = src.TrustedAt
	}
	if src.MaxTurnTokens > 0 {
		dst.MaxTurnTokens = src.MaxTurnTokens
	}
	if src.MaxSessionTokens > 0 {
		dst.MaxSessionTokens = src.MaxSessionTokens
	}

	// Merge workspaces (union)
	if len(src.Workspaces) > 0 {
		existing := make(map[string]bool)
		for _, w := range dst.Workspaces {
			existing[w] = true
		}
		for _, w := range src.Workspaces {
			if !existing[w] {
				dst.Workspaces = append(dst.Workspaces, w)
			}
		}
	}

	// Merge auto rules (union by pattern)
	if len(src.AutoRules) > 0 {
		existingPatterns := make(map[string]bool)
		for _, r := range dst.AutoRules {
			existingPatterns[r.Pattern] = true
		}
		for _, r := range src.AutoRules {
			if !existingPatterns[r.Pattern] {
				dst.AutoRules = append(dst.AutoRules, r)
			}
		}
	}

	// Merge permissions (override if present)
	if src.Permissions != nil {
		dst.Permissions = src.Permissions
	}
}
