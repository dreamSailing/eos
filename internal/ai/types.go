package ai

// Message 表示一条对话消息
type Message struct {
	Role       string   `json:"role"`
	Content    string   `json:"content"`
	ImagePaths []string `json:"image_paths,omitempty"`
	IsMeta     bool     `json:"isMeta,omitempty"` // IsMeta 标识该消息是否为隐藏消息（用于 Skills 系统）
}
