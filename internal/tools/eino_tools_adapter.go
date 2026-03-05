package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"github.com/dreamSailing/vb-coding/internal/ai"
	"github.com/dreamSailing/vb-coding/internal/pkg/utils"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// invokableAdapter implements eino InvokableTool by delegating to Manager.ExecuteStructured
type invokableAdapter struct {
	mgr      *Manager
	name     string
	desc     string
	params   *schema.ParamsOneOf
	examples []ToolExample // 工具使用示例
}

func (t *invokableAdapter) Info(ctx context.Context) (*schema.ToolInfo, error) {
	desc := t.desc
	// 将示例添加到描述中，提升模型理解复杂参数的准确率
	if len(t.examples) > 0 {
		desc += "\n\nExamples:\n"
		for _, ex := range t.examples {
			desc += fmt.Sprintf("- %s: %s\n", ex.Description, formatExampleInput(ex.Input))
		}
	}
	return &schema.ToolInfo{Name: t.name, Desc: desc, ParamsOneOf: t.params}, nil
}

// formatExampleInput 格式化示例输入为简洁的字符串表示
func formatExampleInput(input map[string]any) string {
	if len(input) == 0 {
		return "{}"
	}
	var parts []string
	for k, v := range input {
		switch val := v.(type) {
		case string:
			parts = append(parts, fmt.Sprintf("%s=%q", k, val))
		case []any:
			items := make([]string, len(val))
			for i, item := range val {
				if s, ok := item.(string); ok {
					items[i] = fmt.Sprintf("%q", s)
				} else {
					items[i] = fmt.Sprintf("%v", item)
				}
			}
			parts = append(parts, fmt.Sprintf("%s=[%s]", k, strings.Join(items, ", ")))
		default:
			parts = append(parts, fmt.Sprintf("%s=%v", k, val))
		}
	}
	return fmt.Sprintf("{%s}", strings.Join(parts, ", "))
}

func (t *invokableAdapter) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args map[string]any
	if argumentsInJSON != "" {
		if err := utils.UnmarshalWithEscapeFix(argumentsInJSON, &args); err != nil {
			return "", fmt.Errorf("invalid args: %v", err)
		}
	} else {
		args = map[string]any{}
	}
	if t.name == "vision_parse" {
		apiKey, base, model := ai.ResolveAPISettings()
		if apiKey == "" || base == "" || model == "" {
			return "VISION_UNAVAILABLE", nil
		}
		var images [][]byte
		var mimes []string
		var prompt string
		if v, ok := args["prompt"].(string); ok {
			prompt = v
		}
		wd, _ := os.Getwd()
		if arr, ok := args["images"].([]any); ok {
			for _, item := range arr {
				if m, okm := item.(map[string]any); okm {
					p, _ := m["path"].(string)
					mime, _ := m["mime"].(string)
					ap := p
					if !filepath.IsAbs(ap) {
						ap = filepath.Join(wd, filepath.FromSlash(p))
					}
					if rel, e := filepath.Rel(wd, ap); e != nil || strings.HasPrefix(rel, "..") {
						continue
					}
					if bs, err := os.ReadFile(ap); err == nil {
						images = append(images, bs)
						mimes = append(mimes, mime)
					}
				}
			}
		}
		if len(images) == 0 {
			return "剪贴板或附件为空", nil
		}
		out, code, err := ai.VisionParseWithHTTP(ctx, base, apiKey, model, images, mimes, prompt)
		if err != nil {
			return code, nil
		}
		return out, nil
	}

	// 1. 统一安全检查
	call := ToolCall{Tool: t.name, Parameters: args}
	category, _, summary, dangerous := ClassifyToolDanger(call)

	if dangerous && SafetyGatePrompt != nil {
		if SafetyGateSessionAllowed == nil || !SafetyGateSessionAllowed(category) {
			// 特殊处理 fs write/diff 模式以生成变更预览
			switch t.name {
			case ToolFS:
				mode, _ := args["mode"].(string)
				if mode == "write" {
					p, _ := args["path"].(string)
					c, _ := args["content"].(string)
					// 生成 diff 并设置到 pendingDiff
					diffResults := t.mgr.ExecuteStructured(context.Background(), []ToolCall{{Tool: ToolFS, Parameters: map[string]any{"mode": "diff", "path": p, "content": c}}})
					if len(diffResults) > 0 {
						if SetPendingDiff != nil {
							if diffText, ok := diffResults[0].Data["text"].(string); ok {
								SetPendingDiff(diffText)
							}
						}
						// 保存行数统计
						if added, ok := diffResults[0].Data["added_lines"].(int); ok {
							lastDiffAddedLines = added
						} else if added, ok := diffResults[0].Data["added_lines"].(float64); ok {
							lastDiffAddedLines = int(added)
						}
						if removed, ok := diffResults[0].Data["removed_lines"].(int); ok {
							lastDiffRemovedLines = removed
						} else if removed, ok := diffResults[0].Data["removed_lines"].(float64); ok {
							lastDiffRemovedLines = int(removed)
						}
					}
				}
			case ToolEdit:
				// edit 工具如果是 batch 模式，先运行 previewOnly 以便用户查看
				mode, _ := args["mode"].(string)
				if mode == "batch" {
					argsPrev := make(map[string]any)
					for k, v := range args {
						argsPrev[k] = v
					}
					argsPrev["previewOnly"] = true
					_ = t.mgr.ExecuteStructured(context.Background(), []ToolCall{{Tool: ToolEdit, Parameters: argsPrev}})
				}
			}

			// 调用安全门提示用户
			dec := SafetyGatePrompt(ctx, category, summary)
			if dec == "deny" {
				lastDiffAddedLines = 0
				lastDiffRemovedLines = 0
				return "", fmt.Errorf("operation denied by user")
			}
			if dec == "session" && SafetyGateAllowSession != nil {
				SafetyGateAllowSession(category)
			}
		}
	}

	var out string
	var err error

	tc := ToolCall{Tool: t.name, Parameters: args}
	results := t.mgr.ExecuteStructured(ctx, []ToolCall{tc})
	if len(results) > 0 {
		if results[0].Status == "error" {
			err = fmt.Errorf("%s", results[0].Error)
		} else {
			if results[0].Display != "" {
				out = results[0].Display
			} else {
				bs, _ := json.Marshal(results[0].Data)
				out = string(bs)
			}
		}
	}

	if err != nil {
		return "", err
	}
	return out, nil
}

// formatToolDescription formats the description with examples
func formatToolDescription(desc string, examples []ToolExample) string {
	if len(examples) == 0 {
		return desc
	}
	sb := strings.Builder{}
	sb.WriteString(desc)
	sb.WriteString("\n\nExamples:\n")
	for _, ex := range examples {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", ex.Description, formatExampleInput(ex.Input)))
	}
	return sb.String()
}

// BuildEinoTools constructs eino tools from Manager
func BuildEinoTools(mgr *Manager) []tool.BaseTool {
	mkParams := func(m map[string]*schema.ParameterInfo) *schema.ParamsOneOf {
		if m == nil {
			return nil
		}
		return schema.NewParamsOneOfByParams(m)
	}
	tools := make([]tool.BaseTool, 0)
	// 使用集中定义的工具
	for _, def := range GetAllToolDefinitions() {
		// 预先将示例注入到描述中，确保模型能看到
		fullDesc := formatToolDescription(def.Description, def.Examples)
		tools = append(tools, &invokableAdapter{
			mgr:      mgr,
			name:     def.Name,
			desc:     fullDesc,
			params:   mkParams(def.Params),
			examples: def.Examples,
		})
	}
	// 添加特殊工具（不在集中定义中的）
	tools = append(tools, &invokableAdapter{mgr: mgr, name: "vision_parse", desc: "Parse images with current model", params: mkParams(map[string]*schema.ParameterInfo{"images": {Type: schema.Array, Required: true}, "prompt": {Type: schema.String, Required: false}})})
	return tools
}

func BuildEinoReadOnlyTools(mgr *Manager) []tool.BaseTool {
	mkParams := func(m map[string]*schema.ParameterInfo) *schema.ParamsOneOf {
		if m == nil {
			return nil
		}
		return schema.NewParamsOneOfByParams(m)
	}
	tools := make([]tool.BaseTool, 0)
	// 使用集中定义的工具，只添加低风险工具（只读工具）
	for _, def := range GetAllToolDefinitions() {
		if def.RiskLevel != RiskLevelLow {
			continue
		}
		// 预先将示例注入到描述中
		fullDesc := formatToolDescription(def.Description, def.Examples)
		tools = append(tools, &invokableAdapter{
			mgr:      mgr,
			name:     def.Name,
			desc:     fullDesc,
			params:   mkParams(def.Params),
			examples: def.Examples,
		})
	}
	// 添加特殊工具（不在集中定义中的）
	tools = append(tools, &invokableAdapter{mgr: mgr, name: "vision_parse", desc: "Parse images with current model", params: mkParams(map[string]*schema.ParameterInfo{"images": {Type: schema.Array, Required: true}, "prompt": {Type: schema.String, Required: false}})})
	return tools
}

// Safety gate callbacks provided by runtime/UI
var (
	SafetyGatePrompt         func(ctx context.Context, category, summary string) string
	SafetyGateSessionAllowed func(category string) bool
	SafetyGateAllowSession   func(category string)
	SafetyGateClassify       func(call ToolCall) (category string, level string, summary string, dangerous bool)
	SetPendingDiff           func(diff string)
	ClearReviewText          func() // 新增：清除审阅变更文本的回调
	ObservationConsumer      func(res ToolResult)
)

// 存储最近的 diff 行数统计
var (
	lastDiffAddedLines   int
	lastDiffRemovedLines int
)
