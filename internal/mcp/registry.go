package mcp

import (
	"sort"
	"sync"

	"github.com/dreamSailing/vb-coding/internal/config"
)

// RegistryEntry describes a known MCP server from the built-in registry
type RegistryEntry struct {
	Name        string            `json:"name"`
	Type        config.MCPClientType `json:"type"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	BaseURL     string            `json:"base_url,omitempty"`
	Description string            `json:"description"`
	Category    string            `json:"category,omitempty"`
}

var (
	registryOnce sync.Once
	registryList []RegistryEntry
)

// GetRegistry returns the built-in MCP server registry
func GetRegistry() []RegistryEntry {
	registryOnce.Do(func() {
		registryList = []RegistryEntry{
			{
				Name:        "filesystem",
				Type:        config.MCPTypeStdio,
				Command:     "npx",
				Args:        []string{"-y", "@modelcontextprotocol/server-filesystem"},
				Description: "File system access MCP server",
				Category:    "filesystem",
			},
			{
				Name:        "github",
				Type:        config.MCPTypeStdio,
				Command:     "npx",
				Args:        []string{"-y", "@modelcontextprotocol/server-github"},
				Description: "GitHub API integration",
				Category:    "vcs",
			},
			{
				Name:        "git",
				Type:        config.MCPTypeStdio,
				Command:     "npx",
				Args:        []string{"-y", "@modelcontextprotocol/server-git"},
				Description: "Git operations MCP server",
				Category:    "vcs",
			},
			{
				Name:        "postgres",
				Type:        config.MCPTypeStdio,
				Command:     "npx",
				Args:        []string{"-y", "@modelcontextprotocol/server-postgres"},
				Description: "PostgreSQL database access",
				Category:    "database",
			},
			{
				Name:        "sqlite",
				Type:        config.MCPTypeStdio,
				Command:     "npx",
				Args:        []string{"-y", "@modelcontextprotocol/server-sqlite"},
				Description: "SQLite database access",
				Category:    "database",
			},
			{
				Name:        "brave-search",
				Type:        config.MCPTypeStdio,
				Command:     "npx",
				Args:        []string{"-y", "@modelcontextprotocol/server-brave-search"},
				Description: "Brave web search integration",
				Category:    "search",
			},
			{
				Name:        "memory",
				Type:        config.MCPTypeStdio,
				Command:     "npx",
				Args:        []string{"-y", "@modelcontextprotocol/server-memory"},
				Description: "Persistent memory/knowledge graph",
				Category:    "memory",
			},
			{
				Name:        "fetch",
				Type:        config.MCPTypeStdio,
				Command:     "npx",
				Args:        []string{"-y", "@modelcontextprotocol/server-fetch"},
				Description: "Web fetcher MCP server",
				Category:    "web",
			},
			{
				Name:        "sequential-thinking",
				Type:        config.MCPTypeStdio,
				Command:     "npx",
				Args:        []string{"-y", "@modelcontextprotocol/server-sequential-thinking"},
				Description: "Sequential thinking for complex reasoning",
				Category:    "reasoning",
			},
			{
				Name:        "puppeteer",
				Type:        config.MCPTypeStdio,
				Command:     "npx",
				Args:        []string{"-y", "@modelcontextprotocol/server-puppeteer"},
				Description: "Browser automation via Puppeteer",
				Category:    "automation",
			},
		}
	})
	return registryList
}

// SearchRegistry searches the registry by name, category, or description
func SearchRegistry(query string) []RegistryEntry {
	entries := GetRegistry()
	if query == "" {
		return entries
	}

	var results []RegistryEntry
	for _, e := range entries {
		if matchRegistryEntry(e, query) {
			results = append(results, e)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	return results
}

// FindRegistryEntry finds a specific registry entry by exact name
func FindRegistryEntry(name string) *RegistryEntry {
	for _, e := range GetRegistry() {
		if e.Name == name {
			return &e
		}
	}
	return nil
}

func matchRegistryEntry(e RegistryEntry, query string) bool {
	q := query
	if contains(e.Name, q) || contains(e.Description, q) || contains(e.Category, q) {
		return true
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
