# 计划模式下载计划

## Summary

- 目标：在 `plan` 执行模式下，AI 生成的计划消息除现有“复制”外，再提供一个可点击的“下载”动作。
- 交互：点击“下载”后优先弹出系统目录选择器，用户选择目录后，将当前计划内容保存为 Markdown 文件。
- 平台：首版同时支持 Windows、macOS、Linux。
- 降级：若系统目录选择器不可用，则回退到现有确认输入框，允许用户手动输入保存目录。
- 导出内容：保存原始 Markdown 内容，而不是终端纯文本渲染结果。

## Current State Analysis

### 现有计划/消息呈现

- `internal/ui/app.go`
  - AI 与子代理最终消息都走 `historyEntry` 渲染。
  - 鼠标点击仅支持 `copyHits`，入口是 `tryCopyBubbleAt()`。
  - `historyEntry` 当前没有记录“消息生成时的执行模式”，因此无法只给计划模式消息显示下载动作。
- `internal/ui/components/messages/renderer.go`
  - `RenderAIResponseAtWithCopy()` 和 `RenderAgentFinalAtWithCopy()` 只能向气泡传单个复制标签。
- `internal/ui/components/messages/messages.go`
  - `AIMessage` / `AgentBubbleMessage` 只支持一个 `CopyLabel`。
  - 复制按钮渲染逻辑写死为单个右对齐按钮。
- `internal/ui/styles/styles.go`
  - 已有 `MsgCopyButton` 样式，可以作为多动作按钮的基础复用点。

### 现有可复用交互

- `internal/ui/views/confirm/model.go`
  - `confirm.Request` 已支持 `AllowText` 和 `TextHint`，可直接复用为“手动输入保存目录”的降级方案。
- `internal/ui/app_model_test.go`
  - 已有针对 UI 交互与设置持久化的测试模式，可在这里补充计划下载相关行为测试。

### 当前缺失能力

- 仓库内没有现成的跨平台“系统目录选择器/保存对话框”抽象。
- 现有点击命中模型只支持一种动作，不能区分“复制”和“下载”。
- 当前没有用于计划导出的文件命名、目录校验、保存成功/失败提示文案。

## Proposed Changes

### 1. 扩展消息动作渲染

涉及文件：

- `internal/ui/components/messages/messages.go`
- `internal/ui/components/messages/renderer.go`
- `internal/ui/styles/styles.go`

改动：

- 将 `AIMessage` / `AgentBubbleMessage` 的单一 `CopyLabel` 扩展为动作列表，例如 `[]BubbleAction`，至少支持：
  - `copy`
  - `download`
- 保持普通模式下仍只渲染“复制”，避免影响现有消息布局。
- 计划模式消息在底部右侧并排渲染两个按钮，顺序固定为：
  - `复制`
  - `下载`
- 在 `styles.go` 中补一个与 `MsgCopyButton` 一致或轻微区分的通用动作按钮样式，避免把样式语义继续绑死在 copy 上。

实现决策：

- 不新增新的消息类型，继续复用现有 AI/Agent 气泡。
- 不改变 Markdown 渲染流程，下载内容从原始消息文本取值，不从终端渲染结果反推。

### 2. 将点击命中从“复制”泛化为“消息动作”

涉及文件：

- `internal/ui/app.go`

改动：

- 将 `copyHit` 泛化为类似 `bubbleActionHit` 的结构，至少记录：
  - 命中坐标范围
  - 对应历史消息索引
  - 动作类型 `copy|download`
  - 原始消息文本
- 将 `copyHits` 改为通用动作命中集合。
- 将 `copyMarks()` 扩展为按动作返回标签集合，例如：
  - 复制标签：`复制 / 已复制 / Copy / Copied`
  - 下载标签：新增中英文标签
- 将 `trackCopyHitAt()` 改为对单条消息中的多个动作逐个建立命中区域。
- 将 `tryCopyBubbleAt()` 改成统一动作分发，例如 `tryHandleBubbleActionAt()`：
  - `copy` 走现有剪贴板逻辑
  - `download` 走新保存逻辑

实现决策：

- 继续沿用“通过渲染后的按钮文本反推点击区域”的做法，保持与现有复制按钮一致的实现方式。
- 不改 shell 内容区域的鼠标事件模型，只替换命中数据结构与处理分发。

### 3. 只在计划模式消息上显示下载动作

涉及文件：

- `internal/ui/app.go`

改动：

- 为 `historyEntry` 新增与下载相关的元数据，至少包括：
  - `executionMode string`
  - `rawMarkdown string`
- 在追加 AI 最终消息、子代理最终消息时，把当前 `m.state.ExecutionMode` 写入历史项。
- `rawMarkdown` 直接保存原始 assistant/agent 最终内容，作为导出源。
- 渲染时根据 `historyEntry.executionMode == "plan"` 决定是否追加“下载”动作。

实现决策：

- 仅对最终消息显示下载动作，不对流式中间态、工具消息、系统消息显示。
- “计划模式消息”的判定以消息生成当时的执行模式为准，而不是当前 UI 是否仍处在 plan 模式，避免用户切回 `auto` 后旧计划消息丢失下载入口。

### 4. 新增跨平台目录选择与保存抽象

涉及文件：

- 新增 `internal/pkg/filedialog/filedialog.go`
- 新增 `internal/pkg/filedialog/filedialog_windows.go`
- 新增 `internal/pkg/filedialog/filedialog_darwin.go`
- 新增 `internal/pkg/filedialog/filedialog_linux.go`

改动：

- 提供统一接口，例如：
  - `ChooseDirectory(title string) (string, error)`
- 各平台实现策略：
  - Windows：调用 PowerShell / .NET `FolderBrowserDialog`
  - macOS：调用 `osascript` 的 `choose folder`
  - Linux：按顺序尝试 `zenity`、`kdialog`、`qarma` 等图形目录选择器
- 目录选择器实现内部统一做：
  - 去除尾部换行
  - 标准化绝对路径
  - 返回“不可用/取消/执行失败”的可区分错误
- 如有必要，复用 `internal/pkg/utils/command_windows.go` 的平台命令习惯，避免 Windows 弹出多余控制台窗口。

实现决策：

- 首版选择“目录选择器 + 程序内生成文件名”的方案，不走系统 SaveFile 对话框。
- 这样更贴合你的需求“先选目录”，同时减少三端实现差异。
- Linux 允许依赖系统已有图形工具；若不存在，则进入手动路径降级。

### 5. 新增计划保存流程与降级交互

涉及文件：

- `internal/ui/app.go`
- `internal/ui/views/confirm/model.go`（原则上复用，无需结构调整；只有在现有交互文案不足时再补）

改动：

- 在 `app.go` 中新增计划下载处理流程，例如：
  1. 校验消息内容非空
  2. 调用 `filedialog.ChooseDirectory(...)`
  3. 若成功，生成目标文件路径并写入 Markdown
  4. 若选择器不可用，打开确认输入框要求手动输入保存目录
  5. 校验目录存在且为目录
  6. 将 Markdown 保存到目标目录
  7. 在历史中追加成功/失败系统消息
- 复用 `confirm.Request{AllowText:true}` 做降级交互，新增一个新的 `Kind`，例如：
  - `plan_download_path`
- 在 `confirm.ResultMsg` 分支里处理该保存动作。

文件命名决策：

- 默认文件名采用稳定且可读的 Markdown 名称，例如：
  - `plan-<sessionID>-<YYYYMMDD-HHMMSS>.md`
- 若会话 ID 为空，则退化为：
  - `plan-<YYYYMMDD-HHMMSS>.md`

目录与写入决策：

- 只允许选择或输入目录，不直接输入完整文件路径。
- 程序自动拼接文件名，避免平台路径差异与误输入。
- 如同名文件已存在，优先在同目录生成递增后缀，例如：
  - `plan-xxx.md`
  - `plan-xxx-2.md`

### 6. 补充国际化文案

涉及文件：

- `internal/i18n/zh.go`
- `internal/i18n/en.go`

新增文案：

- 动作标签：
  - `op.download`
  - `op.downloaded`（如果需要短暂状态反馈）
- 下载结果提示：
  - 选择器不可用
  - 请输入保存目录
  - 路径不是目录
  - 计划已保存到某路径
  - 计划保存失败
  - 当前消息不是可下载计划

实现决策：

- 文案风格保持与现有 `op.copy`、`clipboard.copied` 一致。
- 成功提示使用系统消息，不额外做复杂 toast 组件。

### 7. 增加测试覆盖

涉及文件：

- `internal/ui/app_model_test.go`
- 新增 `internal/pkg/filedialog/filedialog_test.go` 或按平台拆分测试（如果核心逻辑足够可抽离）

测试点：

- 计划模式最终消息会带“下载”动作，普通模式不会带。
- 多动作按钮命中计算不会破坏原有复制能力。
- 下载动作在目录选择器返回成功路径时，会生成预期 Markdown 文件。
- 目录选择器不可用时，会进入 `confirm` 文本输入降级流程。
- 手动输入非法路径时给出错误提示。
- 同名文件存在时，文件名去重逻辑正确。

实现决策：

- 目录选择器本身不做真实 GUI 调起测试；在 `app.go` 中通过可替换函数变量或接口注入 fake chooser / fake writer 来覆盖主流程。
- 优先覆盖下载主流程与回退逻辑，避免脆弱的像素/布局测试。

## Assumptions & Decisions

- “AI 生成的计划”定义为：消息生成时处于 `plan` 执行模式的 assistant 最终消息和子代理最终消息。
- 下载内容保存为原始 Markdown，不保存 ANSI、气泡边框或已渲染的终端文本。
- 点击“下载”选择的是目录，不是文件名；文件名由程序自动生成。
- Linux 图形目录选择器允许依赖常见桌面命令；缺失时走手动路径输入，不阻塞功能落地。
- 首版不做“下载后自动打开目录”或“自定义文件名编辑”，保持范围聚焦。
- 若后续发现计划模式消息里还需要对 system/tool 消息下载，可在相同动作框架上继续扩展，但本次不纳入范围。

## Verification Steps

- 单测：
  - `go test ./internal/ui/...`
  - `go test ./internal/pkg/...`
- 手动验证：
  - 启动应用，切到 `plan` 模式，生成一条计划消息，确认底部显示“复制 + 下载”。
  - 点击“复制”，确认现有复制行为不回归。
  - 点击“下载”，在 Windows/macOS/Linux 上分别验证目录选择器能返回目录并保存 `.md` 文件。
  - 在 Linux 缺少图形目录选择器的环境中验证会回退到手动输入目录。
  - 在普通 `auto` 模式生成消息，确认不显示“下载”。
  - 对重复下载同一条计划消息，确认文件名不会被静默覆盖。

