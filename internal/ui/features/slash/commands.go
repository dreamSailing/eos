package slash

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Command 斜杠命令
type Command struct {
	Name        string
	Description string
	Handler     func(args []string) tea.Msg
}

// Commands 所有斜杠命令
var Commands = []Command{
	{Name: "/help", Description: "Show help panel"},
	{Name: "/clear", Description: "Clear content area"},
	{Name: "/exit", Description: "Exit application"},
	{Name: "/history", Description: "Version history panel"},
	{Name: "/models", Description: "Model management panel"},
	{Name: "/mcp", Description: "MCP management panel"},
	{Name: "/ctx", Description: "Context panel"},
	{Name: "/cost", Description: "Cost statistics panel"},
	{Name: "/tasks", Description: "Background tasks panel"},
	{Name: "/lsp", Description: "LSP servers status"},
	{Name: "/rules", Description: "Rules editor panel"},
	{Name: "/workspace", Description: "Workspace commands"},
	{Name: "/git", Description: "Git operations"},
	{Name: "/lang", Description: "Switch language (zh/en)"},
	{Name: "/compact", Description: "Compact context"},
	{Name: "/sessions", Description: "Session management"},
	{Name: "/resume", Description: "Resume session"},
	{Name: "/settings", Description: "Settings panel"},
}

// ParseCommand 解析斜杠命令
func ParseCommand(input string) (cmd string, args []string, isCmd bool) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return "", nil, false
	}

	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", nil, false
	}

	cmd = parts[0]
	if len(parts) > 1 {
		args = parts[1:]
	}

	return cmd, args, true
}

// FindCommand 查找命令
func FindCommand(name string) *Command {
	for _, cmd := range Commands {
		if cmd.Name == name {
			return &cmd
		}
	}
	return nil
}

// GetSuggestions 获取命令建议
func GetSuggestions(prefix string) []Command {
	var suggestions []Command
	for _, cmd := range Commands {
		if strings.HasPrefix(cmd.Name, prefix) {
			suggestions = append(suggestions, cmd)
		}
	}
	return suggestions
}
