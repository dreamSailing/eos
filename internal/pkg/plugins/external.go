package plugins

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dreamSailing/eos/internal/pkg/utils"
	"strings"
)

// ExternalToolPlugin 执行外部命令的插件
type ExternalToolPlugin struct {
	ToolName        string   `json:"name"`
	ToolDescription string   `json:"description"`
	Command         string   `json:"command"`
	Args            []string `json:"args"`
}

func (p *ExternalToolPlugin) Name() string {
	return p.ToolName
}

func (p *ExternalToolPlugin) Description() string {
	return p.ToolDescription
}

func (p *ExternalToolPlugin) PluginMetadata() Metadata {
	commandLine := strings.TrimSpace(p.Command)
	if len(p.Args) > 0 {
		commandLine = strings.TrimSpace(commandLine + " " + strings.Join(p.Args, " "))
	}
	return Metadata{
		Source:  "external",
		Kind:    "command",
		Command: commandLine,
	}
}

func (p *ExternalToolPlugin) Execute(ctx context.Context, params map[string]any) (any, error) {
	// 将参数转换为 JSON 传递给命令
	paramJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	// 构造命令，将参数作为最后一个参数传递
	args := append(p.Args, string(paramJSON))
	cmd := utils.CommandContext(ctx, p.Command, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("command execution failed: %w, output: %s", err, string(output))
	}

	// 尝试解析输出为 JSON
	var result any
	if err := json.Unmarshal(output, &result); err != nil {
		// 如果不是 JSON，则返回原始字符串
		return string(output), nil
	}

	return result, nil
}
