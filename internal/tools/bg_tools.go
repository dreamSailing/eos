package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"github.com/dreamSailing/eos/internal/pkg/utils"
	"github.com/dreamSailing/eos/internal/tools/bg"
	"log/slog"
	"strconv"
	"strings"
)

func (m *Manager) bgTaskStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	action, _ := params["action"].(string)
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: "action parameter is required", Display: "错误：action 参数为必填项"}
	}

	switch action {
	case "start":
		command, _ := params["command"].(string)
		if err := validateBashCommand(command); err != nil {
			return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: err.Error(), Display: "错误：" + err.Error()}
		}
		wd, _ := params["working_dir"].(string)
		var env []string
		if raw, ok := params["env"].([]interface{}); ok {
			for _, v := range raw {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					env = append(env, s)
				}
			}
		}
		logCap := 2000
		if v, ok := params["log_cap"].(float64); ok && int(v) > 0 {
			logCap = int(v)
		}
		var info bg.TaskInfo
		_, err := m.runSandboxedCommand(ctx, []string{"bash", "-lc", command}, func() (string, error) {
			started, err := bg.Default().Start(command, &bg.StartOptions{WorkingDir: wd, Env: env, LogCap: logCap})
			if err != nil {
				return "", err
			}
			info = started
			return started.ID, nil
		})
		if err != nil {
			slog.Error("bg_task.start.error", "component", utils.ComponentTool, "cmd", command, "err", err.Error())
			return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: err.Error(), Display: "错误：" + err.Error()}
		}
		return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "success", Data: map[string]any{"task": info}, Display: "已启动任务：" + info.ID}
	case "list":
		items := bg.Default().List()
		return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "success", Data: map[string]any{"tasks": items}}
	case "cleanup":
		n := bg.Default().CleanupFinished()
		return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "success", Data: map[string]any{"cleaned": n}, Display: fmt.Sprintf("已清理 %d 个已完成的任务", n)}
	case "info":
		id, _ := params["id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: "id required", Display: "错误：id 为必填项"}
		}
		info, err := bg.Default().Info(id)
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: err.Error(), Display: "错误：" + err.Error()}
		}
		return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "success", Data: map[string]any{"task": info}}
	case "tail":
		id, _ := params["id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: "id required", Display: "错误：id 为必填项"}
		}
		fromSeq := int64(0)
		if v, ok := params["from_seq"].(float64); ok {
			fromSeq = int64(v)
		} else if v, ok := params["from_seq"].(string); ok && strings.TrimSpace(v) != "" {
			if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
				fromSeq = n
			}
		}
		limit := 200
		if v, ok := params["limit"].(float64); ok && int(v) > 0 {
			limit = int(v)
		}
		res, err := bg.Default().Tail(id, &bg.TailOptions{FromSeq: fromSeq, Limit: limit})
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: err.Error(), Display: "错误：" + err.Error()}
		}
		return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "success", Data: map[string]any{"tail": res}}
	case "kill":
		id, _ := params["id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: "id required", Display: "错误：id 为必填项"}
		}
		info, err := bg.Default().Kill(id)
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: err.Error(), Display: "错误：" + err.Error()}
		}
		return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "success", Data: map[string]any{"task": info}, Display: "已终止任务：" + id}
	default:
		return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: fmt.Sprintf("unknown action: %s", action), Display: "错误：未知操作 " + action}
	}
}
