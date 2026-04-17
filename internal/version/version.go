package version

var (
	// AppVersion 是应用程序的版本号
	AppVersion = "v0.1.0"

	// BuildCommit 是构建时的 git commit hash（通过 -ldflags 注入）
	BuildCommit = "unknown"

	// BuildDate 是构建时间（通过 -ldflags 注入，RFC3339 格式）
	BuildDate = ""
)

const (
	// AppName 是应用程序名称
	AppName = "EOS"
)
