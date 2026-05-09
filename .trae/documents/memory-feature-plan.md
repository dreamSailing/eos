# 记忆功能规划

## Summary

目标是在现有上下文/规则体系之上，补齐一套独立的记忆系统，让 agent 能在长期使用中持续积累并回用信息，具体包括：

- 新增独立的全局记忆与项目记忆文件体系，不再把“记忆”直接混放在 `EOS.md` 或 `Rules.md` 中。
- 全局记忆以“用户偏好/个人画像”为主，跨项目复用。
- 项目记忆以“项目约定 + 任务结论”为主，服务当前仓库。
- 运行时启动即自动注入全局记忆与项目记忆。
- agent 在会话中自动抽取有效记忆并自动写入。
- 交付范围为产品完整版：后端闭环 + UI 管理 + 索引/状态展示。

## Current State Analysis

### 已有能力

1. 现有长期规则文件会在启动和 prompt 组装时被注入：
   - `internal/ui/startup.go`
   - `internal/runtime/prompt_dynamic.go`
   - `internal/runtime/prompt_context.go`
2. 当前规则/长期文档的主要来源是：
   - 项目 `EOS.md`
   - 项目 `.eos/Rules.md`
   - 全局 `~/.eos/Rules.md`
3. 已存在“建议写记忆”的工具实现：
   - `internal/tools/memory_tool.go`
   - 目前支持 `suggest_memory` / `typed_memory`
4. 已存在会话记忆骨架：
   - `internal/session/session_memory.go`
   - `internal/bridge/runtime_core.go`
   - `internal/bridge/runtime_loop.go`
5. 已存在记忆索引/扫描骨架：
   - `internal/memory/memory_types.go`
   - `internal/memory/memory_index.go`
6. 已存在规则管理面板与“上下文/记忆”入口：
   - `internal/ui/panels/rules.go`
   - `internal/ui/panels/context.go`
   - `internal/ui/app.go`
   - `/memory` 当前实际打开的是上下文面板，而不是真正的记忆管理面板。

### 当前缺口

1. 独立 memory 文件体系不存在。
   - 当前“长期信息”仍绑定在 `EOS.md` / `Rules.md`。
2. 会话记忆提取未闭环。
   - `internal/session/session_memory.go` 中 `ExtractSessionMemory()` 仍是 placeholder，只记录日志，没有真正抽取/写回。
3. 自动记忆写入逻辑过于薄弱。
   - `internal/tools/memory_tool.go` 只是简单追加文本，不做去重、分类、路径分流、索引维护。
4. 全局与项目记忆未形成统一读写协议。
   - 读取靠 prompt 拼接与 pinned docs。
   - 写入靠不同位置各自处理。
5. 记忆体系没有产品化 UI。
   - `/memory` 仅展示上下文消息，不展示项目/全局记忆内容、来源、状态、最近更新。
6. `internal/memory` 包目前基本未接入主流程。
   - `MemoryIndex` / `ScanMemoryFiles` 尚未成为实际运行时依赖。

## Assumptions & Decisions

基于本次确认，锁定如下产品决策：

- 存储形态：采用独立 memory 文件体系。
- 写入方式：自动写入。
- 全局记忆语义：仅承载用户偏好/个人画像，不混入项目知识。
- 项目记忆语义：承载项目约定、结构知识、常用命令、任务结论、排障经验。
- 读取策略：启动即注入全局记忆 + 项目记忆；会话记忆继续作为补充层。
- 交付范围：本次直接做到“产品完整版”，不是只补后端。

同时保留现有规则体系不破坏：

- `EOS.md` 继续承担项目指导/初始化文档角色。
- `.eos/Rules.md` 与 `~/.eos/Rules.md` 继续承担规则编辑角色。
- 新 memory 体系与 rules 体系并存，但职责分离。

## Proposed Changes

### 1. 设计并落地独立 memory 文件结构

在 `internal/memory/` 包内补齐路径、模型和持久化能力，并让它成为统一入口。

涉及文件：

- 修改 `internal/memory/memory_types.go`
- 修改 `internal/memory/memory_index.go`
- 新增 `internal/memory/` 下的存储/路径辅助文件

实施内容：

1. 将现有 memory 类型重新收敛为两条主线：
   - `global` / `user_profile`：全局用户偏好
   - `project`：项目约定与项目经验
2. 定义独立文件位置：
   - 全局记忆：`~/.eos/memory/user.md`
   - 项目记忆：`<workspace>/.eos/memory/project.md`
   - 项目记忆索引：`<workspace>/.eos/memory/MEMORY.md`
3. 扩展 `MemoryEntry`，至少支持：
   - 类型
   - 内容
   - 来源会话/来源文件
   - 更新时间
   - 指纹/去重键
4. 将 `MemoryIndex` 从“扫描 `EOS.md`/`.eos/Rules.md`”改为扫描新的 memory 文件。
5. 为 memory 写入建立统一 API，负责：
   - 规范文件路径
   - 创建目录
   - 内容去重
   - 合并/追加
   - 索引刷新

原因：

- 现在 memory 逻辑散落在 `tools`、`runtime`、`session` 三层，缺少真正的统一存储层。
- 先把存储模型收口，后续 UI、启动注入、自动抽取都能复用。

### 2. 把自动写记忆从“追加文本”升级为统一写入管线

让 `internal/tools/memory_tool.go` 不再直接写文件，而是委托给 `internal/memory` 的统一 store。

涉及文件：

- 修改 `internal/tools/memory_tool.go`
- 修改 `internal/tools/definitions.go`

实施内容：

1. 调整 `suggest_memory` / `typed_memory` 的目标文件约束。
   - 不再只允许 `EOS.md` 和 `.eos/Rules.md`
   - 改为按 memory 类型自动路由到全局或项目 memory 文件
2. 对“自动写入”模式做明确语义：
   - headless/CI 与当前 TUI 均可直接落盘
   - 如后续需要，可保留开关以支持确认式写入
3. 增加记忆归一化逻辑：
   - trim
   - 去重
   - 相似重复检测的最小实现
   - 空内容/过短内容过滤
4. 优化工具描述，让模型明确知道：
   - 什么该写入全局
   - 什么该写入项目
   - 什么不该写入

原因：

- 当前工具实现仅是“把字符串 append 到指定文件”，无法支撑长期稳定使用。

### 3. 真正接通会话记忆抽取

把当前 placeholder 的会话记忆能力补成可用链路，并让它和新的长期 memory 体系协同。

涉及文件：

- 修改 `internal/session/session_memory.go`
- 修改 `internal/bridge/runtime_loop.go`
- 如有必要，修改 `internal/bridge/runtime_core.go`

实施内容：

1. 保留现有 `session.md` 作为短期会话摘要文件：
   - 路径仍为 `<workspace>/.eos/session-memory/session.md`
2. 实现 `ExtractSessionMemory()` 真正流程：
   - 初始化/读取 `session.md`
   - 构建会话提炼 prompt
   - 生成新的 session summary
3. 在抽取阶段增加“长期记忆候选提炼”：
   - 从近期会话中识别全局偏好
   - 从近期会话中识别项目约定/项目结论
4. 将有效候选自动写入新的 memory store。
5. 为抽取触发点增加基础保护：
   - 避免每轮都写
   - 避免重复写入同一结论
   - 避免把临时任务噪声写成长期记忆

原因：

- 当前 session memory 已经有触发时机和模板，但没有真正执行，属于最关键的断点之一。

### 4. 启动阶段自动加载全局/项目记忆

在启动和 prompt 动态拼装阶段，引入新的 memory 文件注入。

涉及文件：

- 修改 `internal/ui/startup.go`
- 修改 `internal/runtime/prompt_dynamic.go`
- 修改 `internal/runtime/prompt_context.go`

实施内容：

1. 在 TUI 启动时自动读取：
   - `~/.eos/memory/user.md`
   - `<workspace>/.eos/memory/project.md`
2. 使用 `SetPinnedDoc()` 将新的 memory 文档注入上下文。
3. 保持现有 `EOS.md` / `Rules.md` 注入不变，但调整顺序与职责说明：
   - 规则是规则
   - 记忆是记忆
4. 在 `prompt_dynamic.go` 中增加“记忆来源说明”与注入顺序：
   - 全局用户画像
   - 项目记忆
   - 会话记忆
5. 在 `prompt_context.go` 中避免把新 memory 与旧规则读取逻辑混为一谈，必要时拆出新的读取函数。

原因：

- 用户要求“越用越懂你”，核心前提就是每次启动都能回用已有长期记忆。

### 5. 升级 `/memory` 为真正的记忆管理面板

把当前 `/memory` 只打开上下文面板的行为，升级为面向“全局记忆 + 项目记忆 + 会话记忆 + 索引状态”的产品化入口。

涉及文件：

- 修改 `internal/ui/app.go`
- 修改 `internal/ui/panels/context.go`
- 新增或拆分 `internal/ui/panels/` 下的 memory 专用面板文件
- 可能调整 `internal/ui/features/slash/commands.go`

实施内容：

1. 保留上下文查看能力，但不再把它当作“记忆面板”的全部。
2. 新的 `/memory` 面板至少展示：
   - 全局记忆内容与路径
   - 项目记忆内容与路径
   - 会话记忆内容与路径
   - 最近更新时间 / 是否存在
   - 索引摘要
3. 提供基础操作：
   - 查看
   - 刷新
   - 编辑
   - 清理/重建索引
4. 在 `AppModel` 中新增 refresh / save / route 逻辑，复用现有 `RulesPanel` 的交互模式。

原因：

- 用户要求“产品完整版”，仅靠自动写和自动读不够，必须有可见、可管、可校验的入口。

### 6. 让会话持久化与记忆体系协同

检查并最小补强会话恢复后的记忆一致性。

涉及文件：

- 修改 `internal/bridge/runtime_sessions.go`
- 视情况修改 `internal/session/persist.go`

实施内容：

1. 保证恢复会话后，新的 memory pinned docs 仍会重新加载。
2. 保证 session 恢复不依赖旧的 pinned 快照中残留的 memory 内容。
3. 必要时在恢复/启动后统一做一次 memory rehydrate，避免陈旧缓存。

原因：

- 否则会出现“会话恢复后用的是旧记忆快照，不是磁盘最新记忆”的问题。

### 7. 增加测试与验收用例

围绕“写得进去、读得出来、不重复、能回用”补测试。

涉及文件：

- 修改/新增 `internal/memory/*_test.go`
- 修改/新增 `internal/session/*_test.go`
- 修改/新增 `internal/runtime/*_test.go`
- 修改/新增 `internal/ui/*_test.go`
- 必要时补 `internal/tools/*_test.go`

测试重点：

1. memory 路径解析正确：
   - workspace 级
   - home 级
2. 自动写入正确分流：
   - 全局偏好 -> `~/.eos/memory/user.md`
   - 项目经验 -> `<workspace>/.eos/memory/project.md`
3. 重复写入不产生明显重复内容。
4. 启动时注入顺序正确。
5. `/memory` 面板能展示新的三层记忆内容。
6. session memory 提取触发后，至少能真正更新 `session.md` 并落长期记忆。

## Verification

实施后按以下顺序验证：

1. 单元测试
   - 运行与 `internal/memory`、`internal/session`、`internal/runtime`、`internal/tools`、`internal/ui` 相关的测试。
2. 启动验证
   - 在空白工作区启动，确认自动创建/读取独立 memory 目录不会报错。
3. 自动写入验证
   - 构造一轮明确的用户偏好与项目结论，确认分别写入全局和项目 memory 文件。
4. 自动注入验证
   - 重启应用后，确认 agent 能读取到之前写入的全局偏好和项目结论。
5. UI 验证
   - `/memory` 能查看三层记忆并进行刷新/编辑。
6. 回归验证
   - 确认 `EOS.md`、`.eos/Rules.md`、`~/.eos/Rules.md` 现有行为不被破坏。

## Success Criteria

- 新增独立 memory 文件体系，且与现有 rules 体系职责分离。
- agent 可自动沉淀全局用户偏好与项目记忆。
- 每次启动会自动加载并回用全局/项目记忆。
- `/memory` 成为真正可用的记忆管理入口。
- 会话记忆不再是 placeholder，而是可运行链路。
- 现有 `EOS.md` / `Rules.md` 功能保持兼容。
