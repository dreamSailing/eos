# 新增 create_skill 工具计划

## Summary

为 EOS 新增一个专门的 `create_skill` 工具，让 agent 可以根据用户的自然语言需求直接生成完整 skill 产物，而不是手动拼接 `.eos/skills/.../SKILL.md`。创建前，agent 需要先分析该 skill 更适合“工作区级”还是“全局级”，若用户未明确指定位置，则先询问用户创建到哪里，再调用工具落盘。工具本身负责在已确认 scope 后生成 `SKILL.md`、按需创建子目录，并在创建后刷新 skill loader，方便后续立即通过 `skill` / `skills_list` 使用。

## Current State Analysis

- Slash 命令 `/plan` 仅用于查看 todo 和切换 `auto|plan` 执行模式，相关实现位于 `internal/ui/features/slash/commands.go` 与 `internal/ui/slash_runtime.go`，和“创建 skill 工具”无直接关系。
- 当前 skill 体系已经具备“扫描、列出、加载、启用”的完整链路：
  - `internal/skills/loader.go` 负责扫描 skill 目录并解析 `SKILL.md` frontmatter。
  - `internal/skills/dirs.go` 约定默认扫描目录，包括工作区 `.eos/skills/`、用户目录 `~/.eos/skills/`，以及 `.claude/.trae` 的兼容只读目录。
  - `internal/tools/manager_skill.go` 与 `internal/tools/skill_tool.go` 负责 `skill` 工具的启用和注入。
  - `internal/tools/mcp_status_tool.go` 中已有 `skills_list` 的 structured handler。
- 当前运行时 prompt 已明确写出“当用户要求生成/创建 skills 时，默认写入当前工作区 `.eos/skills/`”，位置在：
  - `internal/runtime/prompt.go`
  - `internal/runtime/prompt_system.go`
  但这只是行为约定，代码里还没有专门的“创建 skill”工具，也没有“创建前先分析 scope 并询问用户”的标准流程。
- 工具注册和执行链路清晰：
  - `internal/tools/definitions.go` 定义工具常量、参数和说明。
  - `internal/tools/manager_types.go` 注册 structured handler。
  - `internal/tools/manager_execute.go` 统一做权限、风险等级、缓存和执行调度。
- 现有文件系统写入与路径保护能力已经完备，可复用：
  - `internal/tools/fs_tools_fs.go` / `edit.go` 通过 `WorkspaceRootFromContext(ctx)` + `utils.ResolvePathUnder(...)` 限制写入范围。
  - 这意味着新工具可以直接在当前工作区下安全创建 `.eos/skills/<skill-name>/...`。

## Proposed Changes

### 1. 新增工具定义与注册

文件：
- `internal/tools/definitions.go`
- `internal/tools/manager_types.go`

修改内容：
- 新增工具常量 `ToolCreateSkill = "create_skill"`。
- 在 `GetAllToolDefinitions()` 中增加 `create_skill` 的定义，风险等级设为 `RiskLevelMedium`。
- 在 `NewManager()` 的 `structured` 映射中注册 `m.createSkillStructured`。

参数设计：
- `name`：可选。skill 名称；为空时由工具根据需求推导。
- `request`：必填。用户的自然语言需求，用于生成 skill 内容。
- `scope`：建议必填，`workspace|user`。若缺失则工具直接报错，引导 agent 先完成分析与询问，再重新调用。
- `description`：可选。覆盖或补充自动生成描述。
- `allowed_tools`：可选。覆盖生成结果中的 `allowed-tools`。
- `model`：可选。写入 skill frontmatter 的 `model` 字段。
- `argument_hint`：可选。写入 frontmatter。
- `user_invocable`：可选。写入 frontmatter。
- `context`：可选。写入 frontmatter，例如 `fork`。
- `agent`：可选。写入 frontmatter。
- `keywords`：可选。写入 frontmatter。
- `include_scripts` / `include_references` / `include_assets`：可选布尔值，用于决定是否创建对应目录。
- `overwrite`：可选，默认 `false`，避免静默覆盖已有 skill。
- `activate`：可选，默认 `true`，创建后自动 reload skill 列表并尽可能保留激活状态。

设计理由：
- 既支持“纯一句话生成完整 skill”，也允许调用方在需要时锁定关键 frontmatter。
- 参数与 `internal/skills/loader.go` 已支持的 frontmatter 字段对齐，避免引入新格式。
- 将 `scope` 设为显式输入，避免工具在“项目专属 skill”与“跨项目通用 skill”之间擅自做不可逆判断。

### 2. 实现 create_skill handler

文件：
- 新增 `internal/tools/create_skill_tool.go`

修改内容：
- 新增 `func (m *Manager) createSkillStructured(ctx context.Context, params map[string]interface{}) ToolResult`。
- 处理流程：
  1. 解析参数，校验 `request`。
  2. 校验 `scope`，仅接受 `workspace|user`；为空时返回错误，并提示“先分析适合的 scope，再询问用户并带上 scope 重试”。
  3. 决定目标根目录：
     - `scope=workspace`：`<workspaceRoot>/.eos/skills/`
     - `scope=user`：`~/.eos/skills/`
     - 仅兼容读取 `.claude/.trae`，不写入这些目录。
  4. 归一化并清洗 skill 目录名，避免非法字符和空名。
  5. 检查目标目录或 `SKILL.md` 是否已存在；若存在且 `overwrite=false`，返回错误。
  6. 生成 skill 内容：
     - 优先使用当前配置的模型，根据 `request` 生成完整 `SKILL.md` 文本。
     - 若模型不可用、响应为空或解析失败，则回退为最小可用模板。
  7. 创建目录与文件：
     - `<skillDir>/SKILL.md`
     - 按需创建 `scripts/`、`references/`、`assets/`
  8. 若 `activate=true` 且 `m.skillManager != nil`，调用 `ReloadPreserveActive()`。
  9. 返回结构化结果，包括：
     - `name`
     - `scope`
     - `path`
     - `skill_md_path`
     - `created_files`
     - `reloaded`
     - `used_ai_generation`

实现细节：
- 路径校验复用 `utils.ResolvePathUnder(...)` 风格，保证 workspace 模式下不会越界写入。
- `user` scope 需要单独以用户目录为根做路径解析，避免直接字符串拼接。
- 为减少重复代码，工具内部可添加少量私有辅助函数，例如：
  - `resolveSkillTargetRoot(...)`
  - `sanitizeSkillName(...)`
  - `buildFallbackSkillDocument(...)`
  - `generateSkillDocumentWithModel(...)`
- 不做额外复杂抽象；优先保持单文件、最小实现。

### 3. 复用现有模型配置进行 AI 生成

文件：
- 新增 `internal/tools/create_skill_tool.go`
- 参考实现：`internal/tools/planning_tools.go`

修改内容：
- 参考 `planStepsStructured()` 的做法，使用 `ai.ResolveAPISettings()` 拉取当前配置的 API key / base / model。
- 向 `/v1/chat/completions` 发起一次受超时保护的请求，让模型输出完整 skill 文档。
- 对模型输出做约束：
  - 目标是生成完整 `SKILL.md`
  - frontmatter 字段必须与 loader 兼容
  - 正文必须描述何时使用、怎么使用、输入输出、边界和注意事项
- 若模型返回纯正文、前后包裹多余 markdown fence、或未带 frontmatter，工具侧进行最小修复。

设计理由：
- 这符合“AI 完整产物”的需求。
- 复用仓库里已有的轻量 HTTP 调用模式，避免引入新的模型 SDK 层。

### 4. 更新运行时提示，先分析 scope 并询问用户，再使用 create_skill

文件：
- `internal/runtime/prompt.go`
- `internal/runtime/prompt_system.go`
- `internal/runtime/prompt_test.go`

修改内容：
- 在现有“Skills 目录约定”附近新增明确指引：
  - 当用户要求创建 skill 时，先分析该 skill 更适合工作区级还是全局级。
  - 如果用户没有明确指定创建位置，先使用 `ask_user_question` 询问用户要创建在工作区还是全局。
  - 只有在 scope 已明确后，才调用 `create_skill` 工具，而不是手工写文件。
- 保留现有目录约定不变：
  - 工作区级写入工作区 `.eos/skills/`
  - 全局级写入 `~/.eos/skills/`
  - `.claude/.trae` 仅兼容读取
- 如 prompt 测试覆盖到这些片段，则补充断言，确保后续不会回退。

设计理由：
- 只把工具注册进去还不够；需要把“分析 scope + 向用户确认位置”变成标准行为，避免把本应跨项目复用的 skill 写进单个仓库，或把项目专属 skill 错写成全局。

### 5. 补充权限与可发现性约束

文件：
- `internal/runtime/classifier_patterns.go`
- 可选检查：`internal/runtime/tools_node_builder.go`（通常无需改逻辑，只需确认新工具会自动进入工具描述）

修改内容：
- 如果当前工具风险分类依赖显式 allow pattern，则为 `create_skill` 增加合适分类说明。
- 由于这是写入类工具，默认不放进只读 allow 列表；保持 `RiskLevelMedium` 即可走现有审批策略。
- 确认 tool description 自动出现在运行时工具清单中，无需手工拼接动态说明。

设计理由：
- 明确其是“可写入但非高危”的中风险工具。
- 避免错误地把它当成只读 meta 工具。

### 6. 添加针对性测试

文件：
- 新增 `internal/tools/create_skill_tool_test.go`
- 可能补充 `internal/runtime/prompt_test.go`

测试覆盖：
- `workspace` scope 下创建最小 skill 成功，文件落在 `<workspace>/.eos/skills/<name>/SKILL.md`。
- `scope=user` 时会写入用户目录 `~/.eos/skills/`；测试中通过临时 HOME 隔离。
- `scope` 缺失或非法时报错，错误信息明确提示需要先确认创建位置。
- `overwrite=false` 时，已有 skill 不会被覆盖。
- `overwrite=true` 时可覆盖已有 `SKILL.md`。
- `include_scripts/include_references/include_assets` 时会创建对应目录。
- `activate=true` 时在有 skill manager 的情况下会触发 reload，并可被 `skills_list` 看到。
- `request` 为空时报错。
- AI 生成不可用时会回退为合法的最小 `SKILL.md`，且 loader 可正常解析。
- 如更新 prompt 文案，则验证 prompt 中包含“先分析 scope、再询问用户、最后调用 `create_skill`”的说明。

测试策略：
- 以“能否生成可被现有 loader 成功加载的 skill”为核心验收标准，避免写过多低价值测试。

## Assumptions & Decisions

- 工具名称定为 `create_skill`，比在 `skill` 工具里混入“创建”语义更清晰。
- 不允许在 `scope` 未明确时静默落盘；必须先确认是 `workspace` 还是 `user`。
- agent 侧的标准流程是：
  - 先基于需求内容判断推荐 scope
  - 若用户未明确指定，则调用 `ask_user_question` 让用户确认位置
  - 再调用 `create_skill`
- “推荐 scope”的经验规则写入 prompt，而不是在工具内部偷偷替用户决定：
  - 与当前仓库、当前代码结构、仓库约定、项目私有流程强相关的 skill，推荐 `workspace`
  - 跨项目复用的通用流程、通用审查/生成/格式化类 skill，推荐 `user`
- 工具负责生成 skill 产物，不负责自动把 skill 注入当前对话；“创建后可用”与“立即调用”是两个动作。
- 创建后默认 reload skills，以便后续 `skills_list` 和 `skill` 能马上看到新 skill。
- AI 生成不是强依赖；模型不可用时仍应返回一个可工作的 skill 模板，避免工具不可用。
- 不新增新的 slash 命令；本次只补“agent 方便创建 skills 的工具能力”。

## Verification Steps

1. 运行针对新工具的单元测试：
   - `go test ./internal/tools -run CreateSkill`
2. 运行与 prompt 相关的测试：
   - `go test ./internal/runtime -run Prompt`
3. 如有 skills loader 相关改动影响，补跑：
   - `go test ./internal/skills`
4. 手动校验一轮典型流程：
   - 让 agent 遇到“创建 skill”请求但未说明位置，确认其先分析推荐 scope，再调用 `ask_user_question`
   - 用户确认位置后，再通过 `create_skill` 在对应 scope 下生成 skill
   - 调用 `skills_list` 确认新 skill 已出现
   - 用 `skill` 工具启用该 skill，确认 loader 可解析且 prompt 可注入
5. 若 AI 生成功能带有回退逻辑，分别验证：
   - 有模型配置时成功生成完整文档
   - 无模型配置时生成 fallback 文档且仍可被 loader 读取
