package settings

// Settings 表示用户配置
type Settings struct {
	PlanPromptStyle string   `json:"plan_prompt_style"`
	PlanBubbleColor string   `json:"plan_bubble"`
	AutoContext     bool     `json:"auto_context"`
	DesktopNotifications *bool `json:"desktop_notifications,omitempty"`
	MaxInjectKB     int      `json:"max_inject_kb"`
	WatchMode       string   `json:"watch_mode"`
	WatchDebounceMs int      `json:"watch_debounce_ms"`
	PollIntervalSec int      `json:"poll_interval_sec"`
	Language        string   `json:"language"`
	Workspaces      []string `json:"workspaces"`
	ActiveWorkspace string   `json:"active_workspace"`
	Theme           string   `json:"theme"`
	Trusted         bool     `json:"trusted"`
	TrustedAt       string   `json:"trusted_at,omitempty"`
}
