package plugins

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dreamSailing/eos/internal/config"
)

func MergeMCPEntries(cfg *config.Config, workspaceRoot string) []config.MCPEntry {
	base := append([]config.MCPEntry(nil), cfg.MCP...)
	extra := PluginMCPEntries(workspaceRoot, cfg)
	if len(extra) == 0 {
		return base
	}
	return append(base, extra...)
}

func PluginMCPEntries(workspaceRoot string, cfg *config.Config) []config.MCPEntry {
	items, err := Discover(workspaceRoot)
	if err != nil || len(items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	if cfg != nil {
		for _, entry := range cfg.MCP {
			name := strings.ToLower(strings.TrimSpace(entry.Name))
			if name != "" {
				seen[name] = struct{}{}
			}
		}
	}
	out := make([]config.MCPEntry, 0, len(items))
	for _, item := range items {
		if !item.HasMCP {
			continue
		}
		enabled := true
		if cfgEnabled, ok := config.PluginEnabled(cfg, item.Name); ok {
			enabled = cfgEnabled
		}
		if !enabled {
			continue
		}
		entries, err := loadPluginMCPEntries(item)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := strings.ToLower(strings.TrimSpace(entry.Name))
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, entry)
		}
	}
	return out
}

func loadPluginMCPEntries(item Manifest) ([]config.MCPEntry, error) {
	raw, err := os.ReadFile(item.MCPConfigPath())
	if err != nil {
		return nil, err
	}
	text := string(raw)
	replacer := strings.NewReplacer(
		"${CLAUDE_PLUGIN_ROOT}", filepath.ToSlash(item.RootDir),
		"${CLAUDE_PLUGIN_DATA}", filepath.ToSlash(PersistentDataDir(item.Name)),
	)
	text = replacer.Replace(text)
	return config.ParseLegacyMCPServersJSON([]byte(text))
}
