package bridge

type LSPServerInfo struct {
	Language string
	Command  string
	Found    bool
}

type LSPStatus struct {
	Enabled          bool
	AutoDetect       bool
	ConfigFile       string
	Workspace        string
	DetectedLanguage string
	ActiveLanguage   string
	ActiveServer     string
	ActiveRoot       string
	Servers          []LSPServerInfo
	Message          string
}

