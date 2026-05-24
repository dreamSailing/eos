package impl

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/lsp"
	pluginpkg "github.com/dreamSailing/eos/internal/pkg/plugins"
	"github.com/dreamSailing/eos/internal/runtime"
	"github.com/dreamSailing/eos/internal/skills"
	"github.com/dreamSailing/eos/internal/toolapi"
	"github.com/dreamSailing/eos/internal/tools"
)

type catalog struct{}

func newCatalog() toolapi.Catalog {
	return &catalog{}
}

func (c *catalog) List(ctx context.Context) ([]toolapi.ToolDefinition, error) {
	workspaceRoot := strings.TrimSpace(tools.WorkspaceRootFromContext(ctx))
	cfg, _ := config.Load()

	defs := make([]toolapi.ToolDefinition, 0, 64)
	defs = append(defs, builtinToolDefinitions()...)
	defs = append(defs, runtimeCapabilityDefinitions()...)
	defs = append(defs, agentCapabilityDefinitions()...)
	defs = append(defs, pluginCapabilityDefinitions(workspaceRoot, &cfg)...)
	defs = append(defs, skillCapabilityDefinitions(workspaceRoot, &cfg)...)
	defs = append(defs, mcpCapabilityDefinitions(workspaceRoot, &cfg)...)
	defs = append(defs, lspCapabilityDefinitions(workspaceRoot, &cfg)...)
	defs = dedupeDefinitions(defs)

	sort.Slice(defs, func(i, j int) bool {
		if defs[i].Source == defs[j].Source {
			return strings.ToLower(defs[i].Name) < strings.ToLower(defs[j].Name)
		}
		return strings.ToLower(string(defs[i].Source)) < strings.ToLower(string(defs[j].Source))
	})
	return defs, nil
}

func (c *catalog) RiskLevel(toolName string) toolapi.RiskLevel {
	defs, _ := c.List(context.Background())
	if def, ok := toolapi.FindToolDefinition(defs, strings.TrimSpace(toolName)); ok {
		return def.RiskLevel
	}
	if level := runtime.GetToolRiskLevel(strings.TrimSpace(toolName)); level >= runtime.ToolRiskLow {
		switch level {
		case runtime.ToolRiskLow:
			return toolapi.RiskLow
		case runtime.ToolRiskMedium:
			return toolapi.RiskMedium
		case runtime.ToolRiskHigh:
			return toolapi.RiskHigh
		}
	}
	return toRiskLevel(tools.GetToolRiskLevel(strings.TrimSpace(toolName)))
}

func builtinToolDefinitions() []toolapi.ToolDefinition {
	src := tools.GetAllToolDefinitions()
	out := make([]toolapi.ToolDefinition, 0, len(src))
	for _, d := range src {
		params := map[string]toolapi.ParameterInfo{}
		for k, v := range d.Params {
			if v == nil {
				continue
			}
			params[k] = toolapi.ParameterInfo{
				Type:     strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", v.Type))),
				Required: v.Required,
				Desc:     v.Desc,
			}
		}
		examples := make([]toolapi.ToolExample, 0, len(d.Examples))
		for _, ex := range d.Examples {
			examples = append(examples, toolapi.ToolExample{
				Description: ex.Description,
				Input:       ex.Input,
			})
		}
		level := toRiskLevel(d.RiskLevel)
		def := toolapi.ToolDefinition{
			Name:               d.Name,
			Description:        d.Description,
			RiskLevel:          level,
			Params:             params,
			Examples:           examples,
			Source:             toolapi.SourceBuiltin,
			Category:           inferCategory(d.Name),
			VisibleIn:          inferVisibleModes(level),
			ReadOnly:           level == toolapi.RiskLow,
			Invocable:          true,
			RequiresFullAccess: requiresFullAccessByName(d.Name),
			Tags:               inferTags(d.Name, level, true, level == toolapi.RiskLow),
			Metadata:           enrichBuiltinToolMetadata(d.Name),
		}
		out = append(out, def)
	}
	return out
}

func enrichBuiltinToolMetadata(name string) map[string]any {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case tools.ToolDocumentGenerate:
		return map[string]any{
			"output_type":      "document",
			"formats":          []string{"docx", "xlsx", "pdf"},
			"file_extensions":  []string{".docx", ".xlsx", ".pdf"},
			"sandbox_guarded":  true,
			"write_path_param": "path",
		}
	case tools.ToolDocumentConvert:
		return map[string]any{
			"output_type":      "document",
			"formats":          []string{"docx", "xlsx", "pdf"},
			"file_extensions":  []string{".docx", ".xlsx", ".pdf"},
			"sandbox_guarded":  true,
			"write_path_param": "destination_path",
		}
	case tools.ToolNotebookEdit:
		return map[string]any{
			"output_type":      "notebook",
			"formats":          []string{"ipynb"},
			"file_extensions":  []string{".ipynb"},
			"sandbox_guarded":  true,
			"write_path_param": "path",
		}
	case tools.ToolImageGenerate:
		return map[string]any{
			"output_type":      "image",
			"formats":          []string{"png", "jpg", "webp", "gif"},
			"file_extensions":  []string{".png", ".jpg", ".webp", ".gif"},
			"sandbox_guarded":  true,
			"write_path_param": "output_path",
		}
	case tools.ToolVideoGenerate:
		return map[string]any{
			"output_type":      "video",
			"formats":          []string{"mp4", "webm", "mov"},
			"file_extensions":  []string{".mp4", ".webm", ".mov"},
			"sandbox_guarded":  true,
			"write_path_param": "output_path",
		}
	case tools.ToolSpeechSynthesize:
		return map[string]any{
			"output_type":      "audio",
			"formats":          []string{"mp3", "wav", "flac", "aac", "ogg"},
			"file_extensions":  []string{".mp3", ".wav", ".flac", ".aac", ".ogg"},
			"sandbox_guarded":  true,
			"write_path_param": "output_path",
		}
	case tools.ToolBrowserScreenshot:
		return map[string]any{
			"output_type":      "image",
			"formats":          []string{"png"},
			"file_extensions":  []string{".png"},
			"sandbox_guarded":  true,
			"write_path_param": "path",
		}
	default:
		return nil
	}
}

func runtimeCapabilityDefinitions() []toolapi.ToolDefinition {
	return []toolapi.ToolDefinition{
		capabilityDefinition(toolapi.ToolDefinition{
			Name:               "duckduckgo_search",
			Description:        "通过 DuckDuckGo 搜索公开网页信息。",
			RiskLevel:          toolapi.RiskLow,
			Source:             toolapi.SourceRuntime,
			Category:           "web",
			ReadOnly:           true,
			RequiresFullAccess: true,
			VisibleIn:          allModes(),
			Params: map[string]toolapi.ParameterInfo{
				"query": {Type: "string", Required: true, Desc: "搜索关键词"},
			},
			Metadata: map[string]any{
				"provider": "duckduckgo",
				"scope":    "runtime_only",
			},
		}),
		capabilityDefinition(toolapi.ToolDefinition{
			Name:               "wikipedia_search",
			Description:        "通过 Wikipedia 查询百科信息。",
			RiskLevel:          toolapi.RiskLow,
			Source:             toolapi.SourceRuntime,
			Category:           "web",
			ReadOnly:           true,
			RequiresFullAccess: true,
			VisibleIn:          allModes(),
			Params: map[string]toolapi.ParameterInfo{
				"query": {Type: "string", Required: true, Desc: "查询关键词"},
			},
			Metadata: map[string]any{
				"provider": "wikipedia",
				"scope":    "runtime_only",
			},
		}),
		capabilityDefinition(toolapi.ToolDefinition{
			Name:               "http_get",
			Description:        "通过 HTTP GET 拉取 URL 的文本响应。",
			RiskLevel:          toolapi.RiskLow,
			Source:             toolapi.SourceRuntime,
			Category:           "web",
			ReadOnly:           true,
			RequiresFullAccess: true,
			VisibleIn:          allModes(),
			Params: map[string]toolapi.ParameterInfo{
				"url": {Type: "string", Required: true, Desc: "目标 URL"},
			},
			Metadata: map[string]any{"scope": "runtime_only"},
		}),
		capabilityDefinition(toolapi.ToolDefinition{
			Name:               "http_post",
			Description:        "通过 HTTP POST 发送请求并返回响应内容。",
			RiskLevel:          toolapi.RiskHigh,
			Source:             toolapi.SourceRuntime,
			Category:           "web",
			ReadOnly:           false,
			RequiresFullAccess: true,
			VisibleIn:          inferVisibleModes(toolapi.RiskHigh),
			Params: map[string]toolapi.ParameterInfo{
				"url":  {Type: "string", Required: true, Desc: "目标 URL"},
				"body": {Type: "string", Required: false, Desc: "请求体"},
			},
			Metadata: map[string]any{"scope": "runtime_only"},
		}),
		capabilityDefinition(toolapi.ToolDefinition{
			Name:               "http_put",
			Description:        "通过 HTTP PUT 发送请求并返回响应内容。",
			RiskLevel:          toolapi.RiskHigh,
			Source:             toolapi.SourceRuntime,
			Category:           "web",
			ReadOnly:           false,
			RequiresFullAccess: true,
			VisibleIn:          inferVisibleModes(toolapi.RiskHigh),
			Params: map[string]toolapi.ParameterInfo{
				"url":  {Type: "string", Required: true, Desc: "目标 URL"},
				"body": {Type: "string", Required: false, Desc: "请求体"},
			},
			Metadata: map[string]any{"scope": "runtime_only"},
		}),
		capabilityDefinition(toolapi.ToolDefinition{
			Name:               "http_delete",
			Description:        "通过 HTTP DELETE 发送请求并返回响应内容。",
			RiskLevel:          toolapi.RiskHigh,
			Source:             toolapi.SourceRuntime,
			Category:           "web",
			ReadOnly:           false,
			RequiresFullAccess: true,
			VisibleIn:          inferVisibleModes(toolapi.RiskHigh),
			Params: map[string]toolapi.ParameterInfo{
				"url": {Type: "string", Required: true, Desc: "目标 URL"},
			},
			Metadata: map[string]any{"scope": "runtime_only"},
		}),
		capabilityDefinition(toolapi.ToolDefinition{
			Name:        "vision_parse",
			Description: "使用当前模型解析一组图片并返回结构化理解结果。",
			RiskLevel:   toolapi.RiskLow,
			Source:      toolapi.SourceRuntime,
			Category:    "vision",
			ReadOnly:    true,
			VisibleIn:   allModes(),
			Params: map[string]toolapi.ParameterInfo{
				"images": {Type: "array", Required: true, Desc: "图片路径或引用列表"},
				"prompt": {Type: "string", Required: false, Desc: "补充提示"},
			},
			Metadata: map[string]any{"scope": "runtime_only"},
		}),
	}
}

func agentCapabilityDefinitions() []toolapi.ToolDefinition {
	info := runtime.GetDispatchToolsInfo()
	out := make([]toolapi.ToolDefinition, 0, len(info))
	for _, item := range info {
		name, _ := item["name"].(string)
		desc, _ := item["description"].(string)
		params := map[string]toolapi.ParameterInfo{}
		if raw, ok := item["parameters"].(map[string]interface{}); ok {
			required := map[string]bool{}
			if req, ok := raw["required"].([]string); ok {
				for _, field := range req {
					required[strings.TrimSpace(field)] = true
				}
			} else if req, ok := raw["required"].([]interface{}); ok {
				for _, field := range req {
					if text, ok := field.(string); ok {
						required[strings.TrimSpace(text)] = true
					}
				}
			}
			if props, ok := raw["properties"].(map[string]interface{}); ok {
				for field, value := range props {
					prop, _ := value.(map[string]interface{})
					typ, _ := prop["type"].(string)
					descText, _ := prop["description"].(string)
					params[field] = toolapi.ParameterInfo{
						Type:     strings.TrimSpace(strings.ToLower(typ)),
						Required: required[field],
						Desc:     descText,
					}
				}
			}
		}
		level := runtimeRiskLevel(name)
		out = append(out, capabilityDefinition(toolapi.ToolDefinition{
			Name:        name,
			Description: desc,
			RiskLevel:   level,
			Params:      params,
			Source:      toolapi.SourceAgent,
			Category:    "agent",
			VisibleIn:   inferVisibleModes(level),
			ReadOnly:    level == toolapi.RiskLow,
			Metadata: map[string]any{
				"scope": "runtime_only",
			},
		}))
	}
	return out
}

func pluginCapabilityDefinitions(workspaceRoot string, cfg *config.Config) []toolapi.ToolDefinition {
	plugins := pluginpkg.DefaultRegistry().List()
	sort.Slice(plugins, func(i, j int) bool {
		return strings.ToLower(plugins[i].Name()) < strings.ToLower(plugins[j].Name())
	})
	out := make([]toolapi.ToolDefinition, 0, len(plugins))
	seen := map[string]struct{}{}
	for _, plugin := range plugins {
		if plugin == nil {
			continue
		}
		name := strings.TrimSpace(plugin.Name())
		if name == "" {
			continue
		}
		seen[strings.ToLower(name)] = struct{}{}
		level := toolapi.RiskHigh
		meta := pluginpkg.MetadataOf(plugin)
		enabled := pluginpkg.DefaultRegistry().IsEnabled(name)
		if cfgEnabled, ok := config.PluginEnabled(cfg, name); ok {
			enabled = cfgEnabled
		}
		tags := inferTags(name, level, enabled, false)
		tags = append(tags, boolTag("enabled", enabled))
		if meta.Source != "" {
			tags = append(tags, "source:"+strings.ToLower(strings.TrimSpace(meta.Source)))
		}
		out = append(out, toolapi.ToolDefinition{
			Name:               name,
			Description:        strings.TrimSpace(plugin.Description()),
			RiskLevel:          level,
			Source:             toolapi.SourcePlugin,
			Category:           "extension",
			VisibleIn:          inferVisibleModes(level),
			ReadOnly:           false,
			Invocable:          enabled,
			RequiresFullAccess: true,
			Tags:               uniqueStrings(tags),
			Metadata: map[string]any{
				"origin":  "plugin_registry",
				"enabled": enabled,
				"source":  strings.TrimSpace(meta.Source),
				"command": strings.TrimSpace(meta.Command),
				"kind":    strings.TrimSpace(meta.Kind),
			},
		})
	}
	discovered, _ := pluginpkg.Discover(workspaceRoot)
	for _, plugin := range discovered {
		name := strings.TrimSpace(plugin.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[strings.ToLower(name)]; ok {
			continue
		}
		enabled := true
		if cfgEnabled, ok := config.PluginEnabled(cfg, name); ok {
			enabled = cfgEnabled
		}
		tags := []string{
			"plugin",
			"manifest",
			"location:" + strings.ToLower(strings.TrimSpace(plugin.Location)),
			boolTag("enabled", enabled),
		}
		for _, component := range plugin.Components() {
			tags = append(tags, "component:"+strings.ToLower(strings.TrimSpace(component)))
		}
		out = append(out, capabilityDefinition(toolapi.ToolDefinition{
			Name:               name,
			Description:        strings.TrimSpace(plugin.Description),
			RiskLevel:          toolapi.RiskLow,
			Source:             toolapi.SourcePlugin,
			Category:           "extension",
			VisibleIn:          allModes(),
			ReadOnly:           true,
			RequiresFullAccess: true,
			Tags:               uniqueStrings(tags),
			Metadata: map[string]any{
				"origin":        "plugin_manifest",
				"enabled":       enabled,
				"location":      strings.TrimSpace(plugin.Location),
				"root_dir":      strings.TrimSpace(plugin.RootDir),
				"manifest_path": strings.TrimSpace(plugin.ManifestPath),
				"components":    plugin.Components(),
			},
		}))
	}
	return out
}

func skillCapabilityDefinitions(workspaceRoot string, cfg *config.Config) []toolapi.ToolDefinition {
	loader := skills.NewLoader()
	dirs := skills.ResolveScanDirs(workspaceRoot, cfg)
	if len(dirs) == 0 {
		return nil
	}
	loader.SetSkillsDirs(dirs)
	if err := loader.Scan(); err != nil {
		return nil
	}
	skillList := loader.List()
	sort.Slice(skillList, func(i, j int) bool {
		return strings.ToLower(skillList[i].Name) < strings.ToLower(skillList[j].Name)
	})
	out := make([]toolapi.ToolDefinition, 0, len(skillList))
	for _, skill := range skillList {
		if skill == nil {
			continue
		}
		enabled := !config.IsSkillDisabled(cfg, strings.TrimSpace(skill.Name))
		kind := strings.TrimSpace(skill.Kind)
		if kind == "" {
			kind = "skill"
		}
		metadata := map[string]any{
			"skill_name":    skill.Name,
			"location":      skill.Location,
			"base_dir":      skill.BaseDir,
			"argument_hint": skill.ArgumentHint,
			"allowed_tools": skill.AllowedTools.Values(),
			"enabled":       enabled,
			"kind":          kind,
		}
		tags := []string{
			"skill",
			"location:" + strings.TrimSpace(strings.ToLower(skill.Location)),
			boolTag("enabled", enabled),
			"kind:" + strings.TrimSpace(strings.ToLower(kind)),
		}
		if strings.TrimSpace(skill.PluginName) != "" {
			metadata["plugin_name"] = strings.TrimSpace(skill.PluginName)
			metadata["plugin_root"] = strings.TrimSpace(skill.PluginRoot)
			tags = append(tags, "plugin:"+strings.ToLower(strings.TrimSpace(skill.PluginName)))
		}
		if skill.UserInvocable != nil {
			metadata["user_invocable"] = *skill.UserInvocable
		}
		out = append(out, capabilityDefinition(toolapi.ToolDefinition{
			Name:        "skill:" + strings.TrimSpace(skill.Name),
			Description: strings.TrimSpace(skill.Description),
			RiskLevel:   toolapi.RiskLow,
			Source:      toolapi.SourceSkill,
			Category:    "extension",
			VisibleIn:   allModes(),
			ReadOnly:    true,
			Tags:        append(tags, "activated_via:skill"),
			Metadata:    metadata,
		}))
	}
	return out
}

func mcpCapabilityDefinitions(workspaceRoot string, cfg *config.Config) []toolapi.ToolDefinition {
	if cfg == nil {
		return nil
	}
	entries := pluginpkg.MergeMCPEntries(cfg, workspaceRoot)
	if len(entries) == 0 {
		return nil
	}
	entries = append([]config.MCPEntry(nil), entries...)
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	out := make([]toolapi.ToolDefinition, 0, len(entries))
	for _, entry := range entries {
		tags := []string{"mcp", "server"}
		if entry.Enabled {
			tags = append(tags, "enabled")
		} else {
			tags = append(tags, "disabled")
		}
		if entry.Type != "" {
			tags = append(tags, "type:"+strings.ToLower(string(entry.Type)))
		}
		out = append(out, capabilityDefinition(toolapi.ToolDefinition{
			Name:               "mcp:" + strings.TrimSpace(entry.Name),
			Description:        describeMCPEntry(entry),
			RiskLevel:          toolapi.RiskHigh,
			Source:             toolapi.SourceMCP,
			Category:           "mcp",
			VisibleIn:          allModes(),
			ReadOnly:           true,
			RequiresFullAccess: true,
			Tags:               tags,
			Metadata: map[string]any{
				"server":                 entry.Name,
				"enabled":                entry.Enabled,
				"type":                   string(entry.Type),
				"command":                entry.Command,
				"args":                   append([]string(nil), entry.Args...),
				"base_url":               entry.BaseURL,
				"approval_mode":          strings.TrimSpace(entry.ApprovalMode),
				"tool_approval_mode":     strings.TrimSpace(entry.ToolApprovalOverride[strings.TrimSpace(entry.Name)]),
				"tool_approval_override": cloneStringMap(entry.ToolApprovalOverride),
			},
		}))
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func lspCapabilityDefinitions(workspaceRoot string, cfg *config.Config) []toolapi.ToolDefinition {
	if cfg == nil {
		cfg = &config.Config{}
	}
	detector := lsp.NewDetector()
	detectedLanguage := ""
	if strings.TrimSpace(workspaceRoot) != "" {
		detectedLanguage = string(detector.DetectLanguage(workspaceRoot))
	}
	servers := make([]map[string]any, 0, 4)
	for _, lang := range []lsp.LanguageType{lsp.LanguageGo, lsp.LanguagePython, lsp.LanguageTypeScript, lsp.LanguageJavaScript} {
		item := map[string]any{"language": string(lang)}
		info, err := detector.FindServer(lang)
		if err != nil || info == nil {
			item["found"] = false
			item["command"] = "not found"
		} else {
			item["found"] = true
			cmd := strings.TrimSpace(info.Command)
			if len(info.Args) > 0 {
				cmd = strings.TrimSpace(cmd + " " + strings.Join(info.Args, " "))
			}
			item["command"] = cmd
		}
		servers = append(servers, item)
	}
	return []toolapi.ToolDefinition{
		capabilityDefinition(toolapi.ToolDefinition{
			Name:        "lsp",
			Description: describeLSPCapability(workspaceRoot, cfg.LSP.EnabledValue(), cfg.LSP.AutoDetectValue(), detectedLanguage),
			RiskLevel:   toolapi.RiskLow,
			Source:      toolapi.SourceLSP,
			Category:    "lsp",
			VisibleIn:   allModes(),
			ReadOnly:    true,
			Tags: []string{
				"lsp",
				boolTag("enabled", cfg.LSP.EnabledValue()),
				boolTag("auto_detect", cfg.LSP.AutoDetectValue()),
			},
			Metadata: map[string]any{
				"workspace":         workspaceRoot,
				"enabled":           cfg.LSP.EnabledValue(),
				"auto_detect":       cfg.LSP.AutoDetectValue(),
				"detected_language": detectedLanguage,
				"servers":           servers,
			},
		}),
	}
}

func capabilityDefinition(def toolapi.ToolDefinition) toolapi.ToolDefinition {
	def.Invocable = false
	def.Tags = ensureCapabilityTags(def.Name, def.RiskLevel, def.ReadOnly, def.Tags)
	return def
}

func dedupeDefinitions(defs []toolapi.ToolDefinition) []toolapi.ToolDefinition {
	seen := map[string]struct{}{}
	out := make([]toolapi.ToolDefinition, 0, len(defs))
	for _, def := range defs {
		key := strings.ToLower(strings.TrimSpace(def.Name))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, def)
	}
	return out
}

func toRiskLevel(level tools.ToolRiskLevel) toolapi.RiskLevel {
	switch level {
	case tools.RiskLevelLow:
		return toolapi.RiskLow
	case tools.RiskLevelMedium:
		return toolapi.RiskMedium
	case tools.RiskLevelHigh:
		return toolapi.RiskHigh
	default:
		return toolapi.RiskLow
	}
}

func runtimeRiskLevel(name string) toolapi.RiskLevel {
	switch runtime.GetToolRiskLevel(strings.TrimSpace(name)) {
	case runtime.ToolRiskLow:
		return toolapi.RiskLow
	case runtime.ToolRiskMedium:
		return toolapi.RiskMedium
	case runtime.ToolRiskHigh:
		return toolapi.RiskHigh
	default:
		return toolapi.RiskLow
	}
}

func inferCategory(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch {
	case n == tools.ToolDocumentGenerate || n == tools.ToolDocumentConvert:
		return "document"
	case n == tools.ToolNotebookEdit:
		return "notebook"
	case n == tools.ToolImageGenerate || n == tools.ToolVideoGenerate || n == tools.ToolSpeechSynthesize:
		return "multimodal"
	case strings.Contains(n, "browser"):
		return "browser"
	case strings.Contains(n, "git"):
		return "git"
	case strings.Contains(n, "bash") || n == tools.ToolBGTask:
		return "shell"
	case n == tools.ToolRead || n == tools.ToolFS || n == tools.ToolEdit || n == tools.ToolHistory || n == tools.ToolProjectStructure:
		return "filesystem"
	case strings.HasPrefix(n, "http_") || strings.Contains(n, "duckduckgo") || strings.Contains(n, "wikipedia"):
		return "web"
	case n == "lsp" || strings.HasPrefix(n, "lsp:"):
		return "lsp"
	case strings.HasPrefix(n, "mcp:") || strings.Contains(n, "mcp"):
		return "mcp"
	case strings.HasPrefix(n, "skill:") || strings.Contains(n, "skill"):
		return "extension"
	case strings.HasPrefix(n, "invoke_") || strings.Contains(n, "agent"):
		return "agent"
	case strings.Contains(n, "search"):
		return "search"
	case strings.Contains(n, "todo") || strings.Contains(n, "plan"):
		return "planning"
	case strings.HasPrefix(n, "user_") || n == tools.ToolAskUserQuestion:
		return "interaction"
	case strings.Contains(n, "time"):
		return "system"
	case strings.Contains(n, "vision"):
		return "vision"
	default:
		return "general"
	}
}

func inferVisibleModes(level toolapi.RiskLevel) []string {
	switch level {
	case toolapi.RiskLow:
		return allModes()
	default:
		return []string{"auto"}
	}
}

func inferTags(name string, level toolapi.RiskLevel, invocable bool, readOnly bool) []string {
	category := inferCategory(name)
	tags := []string{category, string(level)}
	if readOnly {
		tags = append(tags, "read_only")
	} else {
		tags = append(tags, "mutating")
	}
	if invocable {
		tags = append(tags, "invocable")
	}
	if isFileGeneratingTool(name) {
		tags = append(tags, "file_generation")
	}
	return uniqueStrings(tags)
}

func isFileGeneratingTool(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case tools.ToolDocumentGenerate, tools.ToolDocumentConvert,
		tools.ToolNotebookEdit,
		tools.ToolImageGenerate, tools.ToolVideoGenerate, tools.ToolSpeechSynthesize,
		tools.ToolBrowserScreenshot:
		return true
	default:
		return false
	}
}

func requiresFullAccessByName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case strings.ToLower(tools.ToolBash),
		strings.ToLower(tools.ToolBashSession),
		strings.ToLower(tools.ToolBGTask),
		strings.ToLower(tools.ToolPowerShell):
		return true
	default:
		return false
	}
}

func ensureCapabilityTags(name string, level toolapi.RiskLevel, readOnly bool, tags []string) []string {
	base := inferTags(name, level, false, readOnly)
	base = append(base, "capability_only")
	base = append(base, tags...)
	return uniqueStrings(base)
}

func allModes() []string {
	return []string{"auto", "plan"}
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func describeMCPEntry(entry config.MCPEntry) string {
	parts := []string{"已配置 MCP server"}
	if strings.TrimSpace(entry.Name) != "" {
		parts = append(parts, entry.Name)
	}
	if entry.Type != "" {
		parts = append(parts, "type="+string(entry.Type))
	}
	if strings.TrimSpace(entry.Command) != "" {
		parts = append(parts, "command="+strings.TrimSpace(entry.Command))
	}
	if strings.TrimSpace(entry.BaseURL) != "" {
		parts = append(parts, "base_url="+strings.TrimSpace(entry.BaseURL))
	}
	if entry.Enabled {
		parts = append(parts, "enabled")
	} else {
		parts = append(parts, "disabled")
	}
	return strings.Join(parts, " | ")
}

func describeLSPCapability(workspaceRoot string, enabled bool, autoDetect bool, detectedLanguage string) string {
	parts := []string{"当前工作区 LSP 能力摘要"}
	if strings.TrimSpace(workspaceRoot) != "" {
		parts = append(parts, "workspace="+strings.TrimSpace(workspaceRoot))
	}
	parts = append(parts, boolTag("enabled", enabled))
	parts = append(parts, boolTag("auto_detect", autoDetect))
	if strings.TrimSpace(detectedLanguage) != "" {
		parts = append(parts, "detected="+strings.TrimSpace(detectedLanguage))
	}
	return strings.Join(parts, " | ")
}

func boolTag(prefix string, enabled bool) string {
	if enabled {
		return prefix + ":true"
	}
	return prefix + ":false"
}
