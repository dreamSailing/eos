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

	// Token budget configuration
	MaxTurnTokens    int `json:"max_turn_tokens,omitempty"`
	MaxSessionTokens int `json:"max_session_tokens,omitempty"`

	// Auto mode classifier custom rules
	AutoRules []AutoRule `json:"auto_rules,omitempty"`
}

// AutoRule defines a custom classification rule for auto mode
type AutoRule struct {
	Pattern     string `json:"pattern"`      // regex or glob pattern
	Action      string `json:"action"`       // "allow", "deny", "ask"
	Category    string `json:"category"`     // tool/command category
	Description string `json:"description"`  // human-readable description
}
