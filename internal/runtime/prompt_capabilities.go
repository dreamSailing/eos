package runtime

import (
	"fmt"
	"strings"
	"github.com/dreamSailing/vb-coding/internal/config"
	"github.com/dreamSailing/vb-coding/internal/tools"
)

func BuildSkillsPromptAdditions(tm *tools.Manager) string {
	if tm == nil {
		return ""
	}
	sm := tm.GetSkillManager()
	if sm == nil {
		return ""
	}

	var scanDirs []string
	if st := sm.GetStats(); st != nil {
		if v, ok := st["scan_dirs"].([]string); ok {
			scanDirs = v
		}
	}

	var sb strings.Builder
	sb.WriteString("**可用 Skills**：\n")
	if len(scanDirs) > 0 {
		sb.WriteString("- 扫描目录：\n")
		for _, d := range scanDirs {
			if strings.TrimSpace(d) == "" {
				continue
			}
			sb.WriteString("  - " + d + "\n")
		}
	}

	if s := sm.FormatSkillsForPrompt(); s != "" {
		sb.WriteString("\n")
		sb.WriteString(s)
	} else {
		sb.WriteString("- (无)\n")
	}

	return strings.TrimSpace(sb.String())
}

func BuildMCPConfigPromptAdditions() string {
	cfg, _ := config.Load()
	if len(cfg.MCP) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("**已配置的 MCP Servers（来自 ~/.vb.json）**：\n")
	for _, e := range cfg.MCP {
		line := fmt.Sprintf("- %s | type=%s | enabled=%v", strings.TrimSpace(e.Name), strings.TrimSpace(string(e.Type)), e.Enabled)
		if strings.TrimSpace(e.Command) != "" {
			line += " | command=" + strings.TrimSpace(e.Command)
		}
		if strings.TrimSpace(e.BaseURL) != "" {
			line += " | base_url=" + strings.TrimSpace(e.BaseURL)
		}
		sb.WriteString(line + "\n")
	}
	return strings.TrimSpace(sb.String())
}

