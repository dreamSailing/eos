# 计划默认落到用户空间的实现方案

## Summary

- 目标：让计划内容在默认工作区副本之外，再自动持久化到用户空间 `~/.eos/plans`，避免异常退出后计划丢失，并且便于在目录中检索、交给其他 agent 执行。
- 约束：保持现有工作区内 `.trae/documents` 的行为不回退；用户空间采用“按日期归档 + 最新文件 + 历史快照”；不额外新增 `/plan` 或通知 UI 的路径展示。
- 落地原则：优先复用现有 `lastPlan` / `plan.ready` / `~/.eos/*` 约定，在运行时侧补一个轻量的计划持久化层，而不是引入新的复杂配置中心。

## Current State Analysis

### 现有入口与行为

- `internal/ui/features/slash/commands.go`
  - `/plan` 当前只是一个运行时命令入口，描述为“查看当前计划/待办，或切换计划执行模式”。
- `internal/ui/slash_runtime.go`
  - `handlePlanSlash()` 只读取 todo 列表和执行模式，没有计划文件路径、计划版本或本地持久化逻辑。
- `internal/session/context.go`
  - `ContextManager` 已经有 `lastPlan` 和 `SetOnPlanUpdate()` 回调，说明运行时内部已经有“计划内容更新”的抽象。
- `internal/session/context_utils.go`
  - `SetLastPlan()` 会更新 `lastPlan`，并触发 `onPlanUpdate`；这是最自然的“计划内容变更”监听点。
- `internal/runtime/orchestration.go`
  - 当前只在 `SetOnPlanUpdate()` 回调里发出 `plan.ready` 事件，没有把计划内容落盘。
- `internal/bridge/runtime_sessions.go`
  - 会话持久化仍是工作区内 `.eos/sessions`；这说明当前仓库里已有成熟的“工作区级持久化”模式，但没有“计划的用户空间持久化”。

### 可复用的用户空间约定

- `internal/config/config.go`
  - 配置文件默认落在 `~/.eos.json`，说明产品已经接受“用户主目录下的 EOS 持久化”模式。
- `internal/memory/memory_types.go`
  - 全局记忆默认落在 `~/.eos/memory/user.md`，可作为 `~/.eos/plans` 的目录风格参考。
- `internal/tools/fileops/history.go`
  - 版本历史已经使用 `~/.eos/versions` + 工作区命名空间的思路，说明“用户空间全局目录 + 按工作区隔离”是现成模式。

### 当前缺口

- 仓库内没有发现 `.trae/documents` 的生成或同步逻辑；这意味着“计划文件本身”并不是由现有 Go 代码统一管理的。
- 当前能稳定拿到的计划数据源是 `ContextManager.lastPlan`，因此若要在仓库内补齐默认本地持久化，应该围绕 `SetLastPlan()` / `plan.ready` 做同步，而不是去依赖仓库里并不存在的 `.trae/documents` 生成代码。

## Assumptions & Decisions

- 默认用户空间目录定为 `~/.eos/plans`；若无法获取 home，则回退到当前工作目录下 `.eos/plans`。
- 默认保留工作区副本和用户空间副本，两边都保留；其中：
  - 工作区侧维持“当前版本可直接访问”的语义。
  - 用户空间侧负责“防丢失 + 历史归档 + 跨 agent 传递”。
- 用户空间目录按日期组织，建议格式为 `~/.eos/plans/YYYY/MM/DD/...`。
- 版本策略采用“最新文件 + 历史快照”：
  - 首次生成计划时立即写入一份最新文件并落一份快照。
  - 用户后续每次修改计划，再同步刷新最新文件，并追加一个历史快照。
- 不新增 `/plan` 的路径展示，也不在 `NotifyUser`/提醒里额外展示路径；需求聚焦在“默认本地可恢复、目录可检索”。
- 由于仓库内没有现成的 `.trae/documents` 计划生成器，本次实现以“运行时拿到的计划文本”为单一可信源；如果外层编排还会写 `.trae/documents`，则用户空间持久化相当于新增一个自动镜像层。

## Proposed Changes

### 1. 新增计划持久化能力

- 新增文件：`internal/bridge/runtime_plans.go`
- 内容：
  - 定义计划持久化所需的路径与写入逻辑，例如：
    - 计算用户空间根目录 `~/.eos/plans`
    - 计算日期目录 `YYYY/MM/DD`
    - 计算工作区命名空间（建议复用 `internal/tools/fileops/history.go` 的“基于工作区归一化路径做 hash”思路，但在计划模块内单独实现，避免扩大现有版本历史模块的改动范围）
    - 生成“最新文件路径”和“历史快照路径”
  - 最新文件建议命名为稳定文件名，例如 `current.md` 或 `plan-latest.md`
  - 历史快照建议使用时间戳文件名，例如 `plan-YYYYMMDD-HHMMSS-<slug>.md`
  - 计划标题 slug 可优先取 Markdown 首个 `# ` 标题；没有标题时回退到固定名 `plan`
- 为什么放在 `bridge`
  - `RuntimeCore` 已经负责 session、workspace、settings 等跨运行时持久化问题，计划存储属于同一层级职责。

### 2. 在运行时把计划更新落盘

- 修改文件：`internal/runtime/orchestration.go`
- 内容：
  - 保留现有 `plan.ready` 事件触发。
  - 在 `SetOnPlanUpdate()` 的链路中，把计划文本传给一个运行时外层可消费的持久化函数，而不是只发事件不存储。
- 修改文件：`internal/bridge/runtime_core.go`
  - 为 `RuntimeCore` 增加一个面向计划持久化的入口，负责根据当前活跃工作区决定写入位置，并调用 `runtime_plans.go` 的写入方法。
  - 这里应保证写入失败不会中断主对话流程，只记日志并继续保留内存态计划。
- 设计细节：
  - 每次计划更新都执行两类写入：
    - 工作区副本：刷新当前计划副本。
    - 用户空间副本：刷新 latest 并追加 snapshot。
  - 对同内容重复保存做简单去重，避免模型多次触发相同 `SetLastPlan()` 时刷出一堆重复快照。

### 3. 明确工作区副本的落点

- 优先修改文件：`internal/bridge/runtime_plans.go`（新文件）
- 如需要补充辅助方法，可同步修改：`internal/bridge/runtime_paths.go`
- 方案：
  - 工作区副本继续放在工作区可见目录，建议沿用现有 `.trae/documents` 语义：
    - `<workspace>/.trae/documents/current-plan.md`
  - 不尝试在本次改动里接管外层编排已经生成的“唯一标题计划文件”；仓库内能力只负责维护一个稳定的“当前计划镜像文件”。
- 这样做的原因：
  - 当前仓库里查不到 `.trae/documents/<unique>.md` 的实际生成代码，直接改“原始计划文件命名规则”不可落地。
  - 先补一个稳定镜像文件，可以确保默认本地始终有可恢复版本，同时不影响外层已有计划文件机制。

### 4. 让计划跨会话恢复时仍可继续同步

- 复用文件：`internal/session/persist.go`
  - 这里已经会持久化 `LastPlan`，无需变更字段结构。
- 修改文件：`internal/bridge/runtime_sessions.go`
  - 在恢复会话、自动保存会话相关流程里确认 `LastPlan` 恢复后，计划持久化回调仍然处于可用状态。
  - 如当前恢复链路不会重新触发 `SetLastPlan()`，则只保证后续计划再次更新时能继续写入；不在恢复时额外补写历史快照，避免无意义重复版本。

### 5. 测试覆盖

- 新增测试：`internal/bridge/runtime_plans_test.go`
  - 覆盖：
    - 用户空间目录解析与 home fallback
    - 日期目录生成
    - 工作区命名空间隔离
    - 首版保存时同时生成 latest 和 snapshot
    - 后续更新时 latest 被覆盖、snapshot 追加
    - 相同内容重复保存不追加 snapshot
- 修改/新增测试：`internal/runtime/orchestration_test.go`
  - 验证计划更新回调除了发 `plan.ready` 外，还会调用持久化入口。
- 视改动范围补充：`internal/bridge/runtime_sessions_test.go`
  - 验证恢复会话后不破坏计划更新链路。

## Verification Steps

### 单元验证

- 运行与计划持久化相关的桥接层、运行时测试：
  - `go test ./internal/bridge ./internal/runtime ./internal/session`
- 若新增了独立测试文件，重点断言：
  - `~/.eos/plans/YYYY/MM/DD/<workspace-namespace>/` 目录结构正确
  - latest 文件内容始终等于最新计划文本
  - 每次有效修订都会产生新的 snapshot
  - 重复文本不会重复生成 snapshot

### 手动验证

- 进入计划模式，生成一版计划后检查：
  - 工作区下是否存在稳定副本，例如 `.trae/documents/current-plan.md`
  - 用户空间下是否存在当天目录和 latest/snapshot 文件
- 修改计划内容至少一次，再检查：
  - latest 文件已更新
  - snapshot 数量增加
- 模拟异常中断后重新进入工作区：
  - 可以直接从 `~/.eos/plans` 找到当天计划文件
  - 其他 agent 只需要读取该 Markdown 文件即可继续执行

### 风险点

- `SetLastPlan()` 当前在仓库里只有定义、未搜到显式调用方；执行阶段需要先确认真实计划文本是通过哪条链路写入 `lastPlan` 的。
- 如果外层编排层已经在工作区生成原始计划文件，本次改动要避免与其命名冲突，因此工作区镜像文件名应使用稳定、单独的名字而不是复用外层文件名。
