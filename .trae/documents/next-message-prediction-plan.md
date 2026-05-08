# 下一步预测功能实施计划

## Summary

- 目标：为 TUI 聊天输入框增加“下一步预测”能力。在 AI 完整回复结束后，基于最近的用户与 AI 对话，异步生成一条“用户下一句可能会发送的内容”，以 placeholder/ghost text 形式显示在空输入框中。
- 交互要求：
  - 仅在输入框为空时显示。
  - 用户按 `Right` 时，预测文本才会被写入输入框，变成真实可编辑、可发送内容。
  - 只要用户开始正常输入，预测文本立即消失，且不能抢占用户输入。
  - 不自动发送，不影响 Enter/Shift+Enter/历史记录/Slash 提示/@ 路径提示。
- 配置要求：提供“全局级”开关，保存到 `~/.eos.json`，而不是工作区 `.eos/settings.json`。

## Current State Analysis

### 输入与按键链路

- `internal/ui/components/input/textarea.go`
  - 当前输入组件是对 `bubble textarea` 的轻量封装。
  - 已有普通 `placeholder` 能力，但没有“可接受预测”状态，也没有右方向键接收逻辑。
  - `Update()` 直接委托给 `textarea.Update()`，没有在“真正输入前”拦截建议消失逻辑。
- `internal/ui/views/shell/model.go`
  - `Model` 持有 `input.Model`，并在 `HandleKey()` 中处理 `up/down/history`、`tab/enter` 接受 hints、`esc` 清空输入等。
  - 当前没有消费 `right` 键。
  - Shell 负责 hints 与输入框组合渲染，是接入“右键接受预测”的最佳位置。
- `internal/ui/app.go`
  - AI 完整回复在 `handleAIResponse(... final ...)` 收尾。
  - 用户发送消息在 `sendMessage()`，发送前后已经有完整的 UI 状态切换点，适合在这里清理/刷新预测状态。

### 模型调用能力

- `internal/bridge/runtime_invoke.go`
  - 已有 `Summarize()` 这类“主对话之外的单次模型调用”公共入口。
  - 说明 RuntimeCore 已支持单独派发额外模型请求，不必把预测逻辑塞进主 `GraphInvoke`。
- `internal/bridge/runtime_loop.go`
  - 已有 `summarizeReq` 分支，优先使用 `fastRT`，否则回退主 runtime。
  - 预测请求可以复用这套调度模式，避免影响前台请求的取消状态和主对话生命周期。
- `internal/runtime/model.go`
  - `EinoRuntime.Summarize()` 直接用 `rt.model.Chat()` 跑一个专用 prompt。
  - 这说明“新增一个专用预测 prompt + 一个专用 helper”符合现有 runtime 风格。
- `internal/runtime/prompt.go`
  - 已维护计划、总结等 prompt 常量。
  - 适合新增“下一步预测”专用 prompt，避免复用总结 prompt 导致口吻错误。

### 配置现状

- `internal/config/config.go`
  - 全局配置文件是 `~/.eos.json`，`Config` 已承载全局语言、模型、思考等设置。
  - 非工作区级功能开关最适合放这里。
- `internal/pkg/settings/types.go`
  - 这是工作区设置结构，对应 `.eos/settings.json`。
  - 由于本次开关要求是“全局级”，不应放在这里。
- `internal/ui/panels/settings.go`
  - 当前设置面板主要读写 `internal/pkg/settings.Settings`（工作区级）。
  - 如果要暴露全局开关，需要让面板额外加载/保存 `internal/config.Config` 的一项值，而不是继续塞进 workspace settings。

### 测试现状

- `internal/ui/components/input/textarea_test.go`：已有输入组件单测，可补“右键接受预测/用户输入清除预测”。
- `internal/ui/views/shell/model_test.go`：已有 Shell 层单测，可补“仅空输入框接受预测”。
- `internal/ui/app_model_test.go`：已有 App 级交互测试，可补“AI final 后生成预测并在发送/输入时清理”。
- `internal/ui/panels/settings_test.go`：已有设置面板测试，可补“全局预测开关行的显示与保存”。

## Assumptions & Decisions

- 预测来源：使用模型生成，不做纯规则实现。
- 触发时机：仅在 AI 回复 `final` 之后触发一次异步预测。
- 可见范围：默认仅空输入框显示。
- 接收方式：仅 `Right` 接受，不修改 `Tab` 现有语义（`Tab` 继续用于 thinking toggle / hints 接受）。
- 开关范围：全局级，保存在 `~/.eos.json`。
- 失败策略：预测请求失败、超时或返回空字符串时静默跳过，不向消息区追加错误提示，避免干扰主流程。
- 输出约束：
  - 只接受单条纯文本预测；
  - 去掉首尾空白、外层引号、多余换行；
  - 超长内容截断到适中的单行长度（实现时建议约 120 rune），避免 placeholder 撑坏布局。
- 生命周期约束：
  - 新预测生成后覆盖旧预测；
  - 用户发送消息、切换到 Bash、切换 away from shell、打开需要独占输入的确认/设置/帮助视图、开始新的 processing 时都清掉预测；
  - 用户一旦产生实际输入（字符、粘贴、退格等导致输入值变化），当前预测立即失效，不在用户再删回空串后自动恢复到旧预测。

## Proposed Changes

### 1. 新增专用预测模型调用

- 修改 `internal/runtime/prompt.go`
  - 新增 `PredictNextUserMessagePrompt` 常量。
  - prompt 明确要求：
    - 基于最近几轮“用户-助手”对话，预测“用户接下来最可能发送的一句话”；
    - 输出单行纯文本；
    - 不要解释、不要加前缀、不要引号、不要 markdown；
    - 若不确定则返回空字符串或极短保守结果。
- 修改 `internal/runtime/model.go`
  - 为 `EinoRuntime` 新增类似 `PredictNextUserMessage(ctx, transcript string) (string, error)` 的 helper。
  - 参考 `Summarize()` 的实现，直接构造 `system + user` 两条消息并调用 `rt.model.Chat()`。
  - 返回前做 `TrimSpace` 和基础清洗。

### 2. 在 RuntimeCore 中增加独立预测请求通道

- 修改 `internal/bridge/runtime_core.go`
  - 新增 `predictNextReq` / `predictNextRes` 请求结构。
  - 若当前有 pending 请求集合管理需求，增加对应的 pending 跟踪字段与 helper；如果执行时发现无需单独跟踪，可保持最小实现，但不能复用 `foreground` 状态。
- 修改 `internal/bridge/runtime_loop.go`
  - 参照 `summarizeReq` 增加 `predictNextReq` case。
  - 运行时选择策略：
    - 优先使用 `rc.fastRT`；
    - 无 fast runtime 时退回主 runtime；
    - 不触发主对话的 `StartRequest/EndRequest/FinalizeTask`，避免把预测计入正常会话轮次。
  - 构造输入文本时，从 `rc.cm.ExportState()` 读取最近对话，建议只取最近 2-3 轮 user/assistant 消息，避免把工具摘要和超长上下文直接塞进去。
  - 返回前统一清洗输出：单行化、裁剪长度、过滤空值。
- 修改 `internal/bridge/runtime_invoke.go`
  - 增加公共方法，例如 `PredictNextUserMessage(ctx context.Context) (string, error)`。
  - 这个方法只负责把请求派发到 loop，不操作前台取消器，不改会话历史。

### 3. 在 App 层引入预测消息与生命周期管理

- 修改 `internal/ui/msg.go`
  - 新增 UI 级消息，例如 `PredictionUpdateMsg`，用于承接异步预测结果。
  - 这里不新增 protocol event，保持该功能为 TUI 内部能力，降低跨层影响。
- 修改 `internal/ui/app.go`
  - 在 `AppModel` 中增加预测状态字段，至少包括：
    - 当前预测文本；
    - 预测序号/代次（用于忽略过期异步结果）；
    - 是否允许当前会话显示预测（受全局开关控制）。
  - 在 `NewAppModel()` 初始化时从 `internal/config.Load()` 读取全局开关。
  - 在 `handleAIResponse(... final ...)` 成功结束后：
    - 若当前模式为 AI；
    - 若全局开关开启；
    - 若不是 processing；
    - 发起后台 `tea.Cmd` 调用 `m.adapter.GetCore().PredictNextUserMessage(...)`；
    - 命令返回 `PredictionUpdateMsg`。
  - 在 `Update()` 中处理 `PredictionUpdateMsg`：
    - 若代次已过期则丢弃；
    - 否则更新 shell/input 的预测文本。
  - 在下列时机统一清理预测：
    - `sendMessage()` / `sendBashCommand()` 前后；
    - 进入 processing；
    - `Esc` 清空输入；
    - 切换模式；
    - 切换到 help/panel/setup/confirm 等非 shell 输入主视图；
    - 收到用户实际输入导致输入值变化时。

### 4. 给输入组件增加“预测 placeholder + 右键接受”能力

- 修改 `internal/ui/components/input/textarea.go`
  - 在现有 `placeholder` 概念之外，增加“预测文本”状态，建议字段：
    - `basePlaceholder string`：普通占位文案；
    - `prediction string`：当前预测内容；
  - 保持真正展示逻辑为：
    - 输入值为空且存在预测时，`textarea.Placeholder = prediction`；
    - 否则回退到 `basePlaceholder`。
  - 新增方法：
    - `SetBasePlaceholder(text string)` 或重构现有 `SetPlaceholder()` 使其保存基础 placeholder；
    - `SetPrediction(text string)`；
    - `ClearPrediction()`；
    - `HasPrediction()`；
    - `AcceptPrediction()`：把预测文本写入真实 value，并清掉 prediction。
  - 在 `Update()` 或相关输入前后对比逻辑中检测“用户发生实际编辑”：
    - 如果之前 value 为空且 prediction 存在；
    - 当前按键导致 value 变为非空；
    - 说明用户选择了自己输入，应清掉 prediction。
  - `Clear()` 也要清 prediction，避免旧建议残留。

### 5. 在 Shell 层接入右方向键与状态同步

- 修改 `internal/ui/views/shell/model.go`
  - 为 shell 增加预测转发方法，例如：
    - `SetPrediction(text string)`
    - `ClearPrediction()`
    - `HasPrediction()`
  - 在 `HandleKey()` 中新增 `right` 分支：
    - 仅当输入框为空且存在 prediction 时调用 `m.input.AcceptPrediction()`；
    - 返回 `handled=true`；
    - 其余情况不拦截右方向键，保持 textarea 原有行为。
  - 在 `SetMode()` 切到 Bash 时清空 prediction。
  - 基础 placeholder 仍保持现有文案：
    - AI: `Enter message... (Press ? for help)`
    - Bash: `Enter bash command...`
  - 可选但建议：在帮助面板快捷键中增加 `→` 的说明，提升可发现性。

### 6. 暴露全局配置开关

- 修改 `internal/config/config.go`
  - 在 `Config` 中新增全局字段，例如 `NextMessagePredictionEnabled *bool` 或等价命名。
  - 增加默认语义 helper，例如“nil 视为开启”或“nil 视为关闭”。
  - 建议默认开启，这样功能开箱即用；如果执行时发现产品更偏保守，可改为默认关闭，但需在实现时全链路一致。
- 修改 `internal/ui/panels/settings.go`
  - 设置面板在加载 workspace settings 的同时，也加载全局 `config.Config`。
  - 在表格中增加单独一行，例如 `NextMessagePrediction(Global)`。
  - 编辑该行时，不写入 `internal/pkg/settings.Settings`，而是更新内存中的全局配置副本。
  - 保存时通过扩展 `SettingsSaveMsg` 携带这项全局布尔值，交给 `AppModel` 写入 `~/.eos.json`。
- 修改 `internal/ui/app.go`
  - 扩展 `handleSettingsSave(...)`，在保留现有 workspace settings 保存逻辑的基础上，额外保存全局预测开关到 `config.Save(...)`。
  - 保存成功后立即刷新当前 `AppModel` / `shell` 的预测开关状态：
    - 如果关闭，立刻清掉当前预测。

### 7. 帮助文案与可发现性

- 修改 `internal/i18n/zh.go`
  - 为 `help.key.right` 新增中文文案，例如“接受下一步预测”。
- 修改 `internal/i18n/en.go`
  - 为 `help.key.right` 新增英文文案，例如“Accept next-message prediction”。
- 修改 `internal/ui/views/help/help.go`
  - 在快捷键列表中增加 `→` 行。

### 8. 测试补齐

- 修改 `internal/ui/components/input/textarea_test.go`
  - 增加：
    - 空输入框显示预测时，`AcceptPrediction()` 会把 prediction 写入 value；
    - 用户自己输入后 prediction 被清除；
    - `Clear()` 清理 prediction。
- 修改 `internal/ui/views/shell/model_test.go`
  - 增加：
    - `right` 仅在空输入框且存在 prediction 时接受；
    - 输入框已有真实内容时，`right` 不应把 prediction 覆盖进去。
- 修改 `internal/ui/app_model_test.go`
  - 增加：
    - AI final 后收到 `PredictionUpdateMsg`，shell 输入框进入预测态；
    - 用户开始输入后预测消失；
    - 发送消息、切换 Bash、关闭全局开关后预测会清空。
- 修改 `internal/ui/panels/settings_test.go`
  - 增加：
    - 设置面板显示 `NextMessagePrediction(Global)` 行；
    - 编辑后能正确生成保存消息/状态。
- 视具体实现补充 bridge/runtime 单测：
  - 如果新增 `PredictNextUserMessage()` 的清洗函数，给它补 focused unit test；
  - 若新增 `predictNextReq` loop 分支，给 `internal/bridge/runtime_loop_test.go` 补最小 happy path / empty path 测试。

## Verification

- 手工交互验证
  - 启动 TUI，完成一轮正常对话。
  - 在 AI 回复结束后，确认空输入框出现灰色预测文本。
  - 按 `Right`，确认预测文本进入真实输入框，但未自动发送。
  - 在有预测时直接键入任意字符，确认预测立即消失且字符正常输入。
  - 使用 `/`、`@`、`Tab`、`Enter`、`Esc`、上下历史导航，确认现有功能不回归。
  - 切到 Bash，再切回 AI，确认预测不会污染 Bash 输入。
  - 在设置面板关闭全局开关，确认现有预测立即清空，后续 AI 回复后也不再生成。
- 自动化验证
  - 运行与本次改动相关的 UI/bridge/runtime 单测，至少覆盖 input、shell、app、settings 面板。
  - 如实现过程中新增纯函数清洗逻辑，优先用单测验证边界输入（空串、换行、引号、超长文本）。
- 回归重点
  - 不能影响正常输入；
  - 不能影响 hints 接受；
  - 不能把预测记入历史或上下文；
  - 不能把预测请求误当成前台主请求处理；
  - 关闭开关后必须完全静默。
