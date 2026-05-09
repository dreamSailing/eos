# 输入框实时预测增强计划

## Summary

- 目标：将现有“仅在空输入时展示的下一句预测”升级为“用户输入过程中也能在光标后方看到灰色续写提示，并根据当前输入实时刷新”的交互。
- 成功标准：
  - 在 `AI` 模式、Shell 视图、非处理中，用户输入后可看到基于当前输入前缀的续写提示。
  - 预测更新采用轻量防抖，避免每次击键都直接请求模型。
  - 接受预测时支持 `Right` 和 `Tab` 两种快捷键。
  - 现有空输入预测能力保持可用，不影响 `/`、`@` hints、历史记录、发送消息与取消行为。

## Current State Analysis

- 输入组件当前只维护 `prediction` 文本，并在输入为空时把预测写入 placeholder，没有“已输入内容 + 灰色后缀”的渲染能力，见 `internal/ui/components/input/textarea.go:17`、`internal/ui/components/input/textarea.go:91`、`internal/ui/components/input/textarea.go:257`。
- 输入组件在用户开始输入后会直接清空预测，因此现有预测无法跟随输入保留或局部接受，见 `internal/ui/components/input/textarea.go:243`。
- Shell 层只在“输入为空”时通过 `Right` 接受预测，已有输入时不会接受，也没有 `Tab` 接受预测逻辑，见 `internal/ui/views/shell/model.go:889`。
- App 层仅在 AI 回复结束后调用一次预测接口；收到预测结果时，如果输入框非空则直接丢弃，因此不会在输入过程中刷新，见 `internal/ui/app.go:158`、`internal/ui/app.go:704`、`internal/ui/app.go:2358`。
- Bridge 当前预测接口不接收当前输入前缀，只会根据最近对话构造 transcript 预测“下一整句用户消息”，见 `internal/bridge/runtime_invoke.go:157`、`internal/bridge/runtime_invoke.go:285`。
- Runtime 的预测 prompt 也只描述“预测下一句话”，没有约束“必须基于用户已输入前缀补全后续内容”，见 `internal/runtime/prompt.go:54`、`internal/runtime/model.go:481`。
- 现有测试只覆盖“空输入接受预测”“一旦输入就清掉预测”等老行为，需同步调整测试预期，见 `internal/ui/components/input/textarea_test.go:51`、`internal/ui/views/shell/model_test.go:35`、`internal/ui/app_model_test.go:198`。

## Proposed Changes

### 1. 扩展输入组件，支持“输入值 + 预测后缀”渲染

- 文件：`internal/ui/components/input/textarea.go`
- 修改内容：
  - 将现有 `prediction` 概念拆成“完整候选”与“当前可展示后缀”的计算逻辑，新增基于当前输入值提取 suffix 的能力。
  - 为输入组件增加一个只读渲染层：当用户已有输入且预测以当前输入为前缀时，在 `textarea` 视图后追加一段灰色后缀文本，而不是写入真实输入值。
  - 保留空输入时的 placeholder 展示逻辑，保证老的欢迎态/空框态不退化。
  - 新增“接受后缀预测”的方法：若存在 suffix，则把 suffix 追加到真实输入值；若输入为空，则仍接受完整预测。
  - 调整 `Update` 中“输入即清空预测”的逻辑：改为在输入变化后重新计算 suffix，仅当当前预测已不再匹配输入前缀时才清理。
- 目的：
  - 让用户在输入过程中看到真正的“尾随建议”。
  - 避免预测文本写入真实输入前破坏编辑体验。

### 2. 扩展 Shell 交互，支持 `Right` 和 `Tab` 接受实时续写

- 文件：`internal/ui/views/shell/model.go`
- 修改内容：
  - 将当前 `Right` 接受预测逻辑改为“只要输入组件存在可接受预测就接受”，不再限制为空输入。
  - 在 hints 未显示时，为 `Tab` 增加相同的接受预测分支；若无预测则保持现有 `Tab` 行为不变。
  - 继续保证 hints 打开时，`Tab` 优先用于接受 slash/path hints，不与预测冲突。
- 目的：
  - 满足本次确认的交互偏好。
  - 复用现有 Shell 按键分发，不引入新的全局快捷键冲突。

### 3. 在 App 层增加“输入驱动的预测请求 + 防抖 + 结果失效控制”

- 文件：`internal/ui/app.go`
- 修改内容：
  - 为 `AppModel` 增加预测请求所需的输入快照、轻量防抖状态和更细粒度的序列号/版本号管理。
  - 把 `requestPrediction()` 扩展为接收当前输入文本；当输入变化且满足条件时，走防抖后再请求预测。
  - 在按键处理完成、输入值变化后触发预测调度，而不是只在 AI 回复完成后请求一次。
  - 继续保留 AI 回复完成后的预取逻辑，但将其统一为“以当前输入值（可能为空）为参数发起预测”。
  - 更新 `PredictionUpdateMsg` 处理逻辑：不再简单要求“输入必须为空”，而是校验返回结果是否仍匹配最新输入快照；若过期则丢弃。
  - 在以下场景主动清空或失效预测：切到非 shell、切 Bash、进入 processing、发送消息、清空输入、打开特殊视图、切换设置关闭预测。
- 目的：
  - 真正实现“边输入边预测”。
  - 用防抖和序列号避免闪烁、竞态和过期结果覆盖。

### 4. 扩展 Bridge/Runtime 预测接口，让模型基于当前前缀补全

- 文件：`internal/bridge/runtime_invoke.go`
- 修改内容：
  - 扩展 `PredictNextUserMessage` 签名，使其接收当前输入前缀。
  - 在 `buildPredictionTranscript` 基础上，把“当前用户已输入内容”拼进 transcript，明确这是待续写前缀而非完整新句子。
  - 增强 `cleanPredictionText`：如果模型返回的是完整句子而不是后缀，保留清洗后全文；后续交给 UI 侧根据前缀计算 suffix。
- 文件：`internal/runtime/prompt.go`
- 修改内容：
  - 更新 `PredictNextUserMessagePrompt`，明确两种模式：
    - 当前输入为空：预测用户下一条完整消息。
    - 当前输入非空：基于已输入前缀续写，输出最自然的完整消息，不要改写前缀。
- 文件：`internal/runtime/model.go`
- 修改内容：
  - 保持 Runtime 调用结构不变，仅消费新的 transcript/prompt 语义。
- 目的：
  - 让模型输出更贴近“补全”而不是“重新猜一句完全不同的话”。

### 5. 调整并补充测试，覆盖实时预测新行为

- 文件：`internal/ui/components/input/textarea_test.go`
- 修改内容：
  - 保留空输入接受预测测试。
  - 新增“已有输入时只显示/接受后缀”的测试。
  - 新增“输入继续变化但仍命中前缀时，不立即清空预测”的测试。
  - 新增“不再匹配前缀时清空预测”的测试。
- 文件：`internal/ui/views/shell/model_test.go`
- 修改内容：
  - 调整 `Right` 键测试，使其覆盖“空输入”和“已有前缀”两种接受场景。
  - 新增 `Tab` 在无 hints 时接受预测的测试。
  - 保留 hints 可见时 `Tab` 优先接受 hint 的行为测试，如现有覆盖不足则补一条。
- 文件：`internal/ui/app_model_test.go`
- 修改内容：
  - 调整旧测试，不再要求“输入一个字就必须立刻清空预测”，而是验证输入后会根据前缀重新匹配/刷新。
  - 新增“输入变化后调度预测请求”的状态测试。
  - 新增“过期 prediction seq 不覆盖当前输入”的测试。
- 目的：
  - 把本次行为变更锁定为可回归验证的契约。

## Assumptions & Decisions

- 已确认交互偏好：
  - 接受预测使用 `Right` 和 `Tab`。
  - 更新策略使用轻量防抖，而不是每次击键立即请求。
- 防抖时长默认实现为约 `300ms`，若代码中已有更合适的 Tick/延时模式，则沿用现有风格。
- 实时预测仍只在 `AI` 模式下启用；`Bash` 模式不展示任何续写。
- 预测结果以“完整候选句”在状态中保存，UI 根据当前输入计算 suffix，这样可以兼容模型偶尔返回整句的情况。
- 若当前输入不是预测结果前缀，则不展示灰色后缀，避免误导性补全。
- 本次不新增设置项；继续复用已有全局开关 `NextMessagePrediction(Global)`。

## Verification

- 单元测试：
  - 运行输入组件、Shell、AppModel 相关测试，重点覆盖 `textarea`、`shell`、`app` 三层新增场景。
- 交互检查：
  - 在 Shell 的 `AI` 模式输入中文短句，确认输入后 300ms 左右出现灰色尾随提示。
  - 持续追加字符，确认提示随输入变化刷新，旧结果不会闪回。
  - 按 `Right` 或 `Tab`，确认仅把灰色后缀补入输入框，不影响已输入前缀。
  - 打开 slash/path hints 时按 `Tab`，确认仍优先接受 hint。
  - 切到 `Bash`、开始发送、按 `Esc` 清空、关闭预测设置，确认预测消失。
- 回归检查：
  - 空输入时仍可展示并接受完整预测。
  - `/` 和 `@` 提示、历史上下键、多行输入、发送消息流程保持原有行为。
