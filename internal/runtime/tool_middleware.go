package runtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
	"github.com/dreamSailing/vb-coding/internal/tools"

	"github.com/cloudwego/eino/compose"
)

func buildToolCallMiddleware(onMeta func(string), toolTimeout time.Duration) compose.ToolMiddleware {
	emitter := NewEventEmitter(onMeta)

	return compose.ToolMiddleware{
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
				if input != nil {
					tools.NotifyToolCall(ctx, input.Name)
				}
				// 发送工具调用事件（JSON格式）
				if onMeta != nil && input != nil {
					var params map[string]any
					if input.Arguments != "" {
						// 尝试解析参数为JSON
						_ = json.Unmarshal([]byte(input.Arguments), &params)
					}
					emitter.EmitToolCallJSON(input.CallID, input.Name, params)
					slog.Debug("runtime.tool_middleware.tool_call", "component", utils.ComponentTool,
						"tool", input.Name,
						"call_id", input.CallID,
					)
				}

				var output *compose.ToolOutput
				var err error
				if input != nil && strings.EqualFold(strings.TrimSpace(input.Name), tools.ToolUserConfirm) {
					output, err = next(ctx, input)
				} else {
					c2, cancel := context.WithTimeout(ctx, toolTimeout)
					output, err = next(c2, input)
					cancel()
				}

				// 发送工具结果事件（JSON格式）
				if onMeta != nil && output != nil {
					result := tools.ToolResult{
						ID:     input.CallID,
						Tool:   input.Name,
						Status: "success",
						Ts:     time.Now().Unix(),
					}
					if err != nil {
						result.Status = "error"
						result.Error = err.Error()
					}
					if output.Result != "" {
						result.Display = output.Result
						if len(result.Display) > 200 {
							result.Display = result.Display[:200] + "..."
						}
					}
					if result.Status == "success" && output.Result != "" {
						low := strings.ToLower(strings.TrimSpace(output.Result))
						if strings.HasPrefix(low, "error:") || strings.HasPrefix(low, "err:") || strings.Contains(low, "not found") || strings.Contains(low, "文件未找到") {
							result.Status = "error"
							result.Error = output.Result
						}
					}
					tools.NotifyToolResult(ctx, input.Name, result.Status == "success")

					emitter.EmitToolResultJSON(result)
					slog.Debug("runtime.tool_middleware.tool_result", "component", utils.ComponentTool,
						"tool", input.Name,
						"call_id", input.CallID,
						"status", result.Status,
					)
				}

				return output, err
			}
		},
	}
}
