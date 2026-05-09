## Summary

为 EOS 增加“agent 可操作浏览器”的能力，优先采用成熟的外部浏览器 MCP server，而不是在仓库内自研浏览器驱动。基于当前仓库已经具备完整的 MCP 客户端加载、工具暴露、配置编辑和插件发现能力，最合适的方案是产品化接入官方 Playwright MCP，让用户和 agent 可以低成本启用浏览器自动化能力。

本次方案目标不是实现一个新的内置浏览器工具栈，而是把“浏览器 MCP server 的安装、配置、启用、发现、引导和验证”做成一等能力。这样可以最大化复用现有 `internal/mcp`、`internal/tools`、`internal/ui` 和插件发现机制，降低维护成本，并获得更成熟稳定的浏览器自动化能力。

## Current State Analysis

### 已有能力

1. 仓库已经具备完整的 MCP client 接入链路。
   - `internal/bridge/runtime_loop.go`
   - `internal/toolapi/impl/extensions.go`
   - `internal/mcp/mcp.go`
   - Runtime 启动时会加载配置中的 MCP server，并把其工具并入运行时工具集。

2. 仓库已经具备 MCP 配置管理 UI。
   - `internal/ui/app.go`
   - `internal/ui/panels/mcp.go`
   - `internal/ui/views/setup/mcp_editor.go`
   - 当前可以在 `/mcp` 面板里增删改查和启停 MCP server。

3. 仓库已经具备 MCP 工具目录和运行态展示能力。
   - `internal/tools/mcp_status_tool.go`
   - `internal/tools/mcp_resource_tool.go`
   - `internal/tools/mcp_prompt_tool.go`
   - `internal/toolapi/impl/catalog.go`
   - 说明 agent 理论上可以消费外部 MCP 暴露的浏览器工具，只要 server 被正确接入。

4. 仓库已经具备插件发现与插件自带 `.mcp.json` 合并能力。
   - `internal/pkg/plugins/discovery.go`
   - `internal/pkg/plugins/mcp.go`
   - 会扫描 `~/.eos/plugins`、`~/.claude/plugins`、`~/.trae/plugins`，以及工作区下对应目录。

5. 仓库已经支持标准 MCP server 文档和 CLI 能力。
   - `README.md`
   - `README.en.md`
   - `internal/docs/mcp/SERVER.md`
   - 但当前文档面向的是 EOS 自己作为 MCP server，对“如何接入浏览器 MCP”没有产品化说明。

### 当前缺口

1. 当前没有默认浏览器能力接入。
   - 仓库中没有浏览器专用 MCP 预设。
   - 没有 `browser` / `playwright` 的内置配置模板。
   - 没有浏览器专用 UI 入口或引导文案。

2. 当前没有“一键启用浏览器”的产品体验。
   - 用户虽然可以手工在 `/mcp` 面板里输入 JSON 配置，但门槛较高。
   - agent 也无法稳定假设“浏览器 MCP 一定存在且可用”。

3. 当前没有浏览器可用性检查和错误指引。
   - 缺少对 `playwright` MCP server 是否已配置、是否启用、是否可调用的专门检测。
   - 缺少缺少 Node、缺少 Playwright、浏览器未安装等失败场景的明确提示。

4. 当前提示词层没有明确告诉 agent 如何使用浏览器能力。
   - `internal/runtime/prompt.go`
   - `internal/runtime/prompt_dynamic.go`
   - 现在只有泛化的 MCP 说明，没有“遇到网页操作任务时优先使用浏览器 MCP”的清晰引导。

### 技术决策依据

1. 当前仓库最适合接入外部成熟浏览器 MCP，而不是内置实现浏览器控制。
   - 仓库已经有成熟的 MCP 客户端能力和 UI。
   - 自研浏览器驱动意味着要新增进程管理、浏览器生命周期、页面状态、动作语义、选择器鲁棒性和截图/快照协议，维护成本很高。

2. 浏览器 MCP 方案优先选择 Playwright MCP。
   - 它是成熟且主流的浏览器自动化方案，天然支持 MCP 暴露。
   - 它的能力模型与当前仓库现有 MCP 接入方式完全兼容。
   - 它比在 EOS 内部增加一套独立 browser tool 更符合当前架构。

## Proposed Changes

### 1. 增加浏览器 MCP 预设与标准配置生成

涉及文件：

- `internal/ui/app.go`
- `internal/ui/views/setup/mcp_editor.go`
- 可能新增 `internal/ui/mcp_presets.go`
- 可能新增 `internal/ui/mcp_presets_test.go`

变更目标：

- 提供 Playwright MCP 的官方推荐配置预设，降低手工编辑 JSON 的门槛。
- 让用户在 `/mcp` 面板或 MCP 编辑器中可以快速生成浏览器配置。

实现方式：

1. 新增 MCP 预设定义层。
   - 定义至少一个预设：`playwright`
   - 预设内容使用 stdio 方式：
     - `name`: `playwright`
     - `type`: `stdio`
     - `command`: `npx`
     - `args`: `["-y", "@playwright/mcp@latest"]`
     - `enabled`: `true`

2. 为 Linux 无图形环境预留第二个可选预设。
   - `playwright-sse-local`
   - 使用已有浏览器 MCP server 的 SSE 暴露方式
   - 不作为默认方案，只作为高级场景文档与模板

3. 在 MCP 新增入口中支持“插入预设模板”。
   - 默认空白模板替换为“预设列表 + 预填 JSON”
   - 第一阶段不做复杂向导，直接把 JSON 预设生成到编辑器中即可

原因：

- 当前 `MCPConfigEditorView` 只支持手工输入 JSON，门槛高。
- 预设模板属于最小改动，能显著提升浏览器接入成功率。

### 2. 在 `/mcp` 面板增加浏览器能力引导和快捷操作

涉及文件：

- `internal/ui/panels/mcp.go`
- `internal/ui/app.go`
- `internal/i18n/zh.go`
- `internal/i18n/en.go`

变更目标：

- 让用户从 MCP 面板直接知道“浏览器能力当前是否可用”和“如何快速启用”。

实现方式：

1. 扩展 MCP 面板的说明区域。
   - 增加浏览器能力提示：
     - 若已存在名为 `playwright` 的 MCP server，则显示“浏览器已配置”
     - 若不存在，则显示“可添加 Playwright 浏览器自动化”

2. 增加快捷键或操作项。
   - 新增 `Preset` 或 `Browser` 操作
   - 触发后直接打开 MCP 编辑器并填入 Playwright 预设 JSON

3. 增加状态提示。
   - 若配置存在但未启用，提示用户启用
   - 若配置已启用但 runtime 重载失败，沿用现有 `MCPReloadDoneMsg` 错误通道输出更明确的浏览器安装建议

原因：

- 现在 `/mcp` 面板更像通用配置面板，不具备“浏览器接入”这种高频场景的产品化体验。

### 3. 增加浏览器 MCP 检测与诊断摘要

涉及文件：

- 可能新增 `internal/tools/browser_status_tool.go`
- `internal/tools/definitions.go`
- `internal/tools/manager_types.go`
- `internal/ui/slash_runtime.go`
- 可能新增 `internal/tools/browser_status_tool_test.go`

变更目标：

- 提供浏览器能力的只读检测，不要求用户自己去推断 MCP 状态。

实现方式：

1. 新增只读工具或运行时 helper。
   - `browser_status`
   - 输出内容包括：
     - 是否配置了浏览器 MCP
     - 配置的 server 名称
     - 是否启用
     - MCP manager 当前是否已加载该 server
     - 若不可用，给出下一步建议

2. 在 `/doctor` 或 `/status` 相关输出中并入浏览器状态摘要。
   - 不改变现有命令语义，只在已有 runtime 摘要中追加浏览器部分

3. 失败提示要覆盖典型问题。
   - 未配置 `playwright` MCP
   - `npx`/Node 不存在
   - 首次运行需要下载 Playwright 依赖
   - server 已配置但未启用

原因：

- 当前 `mcp_status` 是通用级别，用户仍然需要知道应该找哪个 server。
- 浏览器能力是强感知能力，值得有专门的状态检查。

### 4. 在提示词层明确浏览器使用策略

涉及文件：

- `internal/runtime/prompt.go`
- `internal/runtime/prompt_dynamic.go`
- 可能涉及 `internal/runtime/orchestration.go`

变更目标：

- 让 agent 在合适任务中主动使用浏览器 MCP，而不是只会读网页源码或盲猜页面行为。

实现方式：

1. 在系统提示或动态提示中增加浏览器能力说明。
   - 当检测到配置中存在可用的 `playwright` MCP server 时，注入浏览器能力说明
   - 明确以下场景优先使用浏览器：
     - 页面交互验证
     - 登录流程验证
     - 前端页面实际行为确认
     - 需要点击、输入、选择、截图、等待页面变化的任务

2. 保持与现有 WebSearch/WebFetch 的边界清晰。
   - `web_fetch` 适合只读抓取网页内容
   - 浏览器 MCP 适合需要真实交互的网页任务

3. 增加使用约束。
   - 浏览器不可用时不要假装可用
   - 优先说明缺失并建议启用 `playwright` MCP

原因：

- 只有把能力写进提示词，agent 才会稳定选择浏览器工具链。

### 5. 产品化文档：说明“浏览器能力当前没有内置，推荐 Playwright MCP”

涉及文件：

- `README.md`
- `README.en.md`
- 可能新增 `internal/docs/mcp/BROWSER.md`

变更目标：

- 明确回答“现在是不是还没有”的问题，并给出官方推荐接入方式。

实现方式：

1. 在 README 中新增 Browser Automation 小节。
   - 说明 EOS 当前不内置浏览器驱动
   - 官方推荐通过 Playwright MCP 接入浏览器能力
   - 给出最小可用配置 JSON

2. 增加启用方式说明。
   - 通过 `/mcp` 面板新增
   - 或直接编辑配置文件
   - 或通过插件目录提供 `.mcp.json`

3. 增加常见问题说明。
   - 为什么不内置浏览器驱动
   - 为什么选 Playwright MCP
   - 何时用浏览器 MCP，何时只用 `web_fetch`

原因：

- 当前 README 虽有 MCP server 文档，但没有“浏览器能力接入”说明。

### 6. 支持插件方式分发浏览器能力

涉及文件：

- `internal/pkg/plugins/discovery.go`
- `internal/pkg/plugins/mcp.go`
- 可能新增测试但不需要修改核心逻辑
- 新增示例资源目录，例如 `assets/plugins/browser-playwright/.mcp.json` 或文档样例

变更目标：

- 除了全局配置，还支持以后把浏览器能力作为插件预设下发。

实现方式：

1. 不改现有插件扫描机制，只补标准样例。
2. 提供一个浏览器插件示例结构，包含：
   - `.claude-plugin/plugin.json`
   - `.mcp.json`
3. 文档说明将该目录放在：
   - `~/.eos/plugins/browser-playwright`
   - 或工作区 `.eos/plugins/browser-playwright`

原因：

- 当前插件系统已经支持 `.mcp.json` 注入，适合作为后续扩展点。
- 这样可以保持主程序轻量，同时支持后续企业内部分发浏览器预设。

### 7. 补充测试，确保浏览器接入不破坏现有 MCP 流程

涉及文件：

- 新增 `internal/ui/mcp_presets_test.go`
- 新增 `internal/tools/browser_status_tool_test.go`
- 视实现调整：
  - `internal/ui/features/slash/commands_test.go`
  - `internal/toolapi/impl/catalog_test.go`
  - `pkg/core/mcp_import_test.go`

测试重点：

1. Playwright 预设 JSON 生成正确。
2. MCP 编辑器入口能插入浏览器预设。
3. 浏览器状态检测在“未配置 / 已配置未启用 / 已启用”三种状态下输出正确。
4. `mcp_import` 与现有解析逻辑继续兼容。
5. README 或文档引用的最小配置与解析逻辑保持一致。

## Assumptions & Decisions

- 决策：不在 EOS 内自研浏览器内置工具，采用成熟外部 Playwright MCP 方案。
- 决策：默认浏览器方案使用 stdio 方式接入 `@playwright/mcp`。
- 决策：第一阶段重点做“接入体验产品化”，不是包装所有浏览器工具为 EOS 原生工具。
- 决策：浏览器能力默认 server 名称使用 `playwright`，便于提示词、诊断和 UI 统一识别。
- 决策：不修改现有通用 MCP 配置模型，浏览器仍然本质上是一个标准 MCP server。
- 决策：优先增加预设、检测、文档和提示词，不引入重型安装器或后台守护进程。
- 假设：运行环境具备 Node.js，并允许通过 `npx` 启动 `@playwright/mcp`。
- 假设：浏览器首次安装依赖仍由 Playwright MCP 自己负责，不在 EOS 内接管。

## Verification Steps

1. 单元测试
   - 运行与新增预设、状态检测、MCP 配置相关的测试。
   - 重点覆盖 `internal/ui`、`internal/tools`、`pkg/core`。

2. 配置验证
   - 通过 `/mcp` 面板插入 Playwright 预设。
   - 确认配置能被 `ListMCPServers()` 正确读取。

3. 运行时验证
   - 启用 Playwright MCP 后重载 runtime。
   - 确认 `mcp_status` 或新增 `browser_status` 能识别该 server。

4. Agent 能力验证
   - 在提示词中让 agent 执行一个简单网页交互任务。
   - 确认 agent 能优先选择浏览器 MCP，而不是仅用 `web_fetch`。

5. 回归验证
   - 现有 MCP 面板的新增、编辑、删除、启停行为不受影响。
   - 现有非浏览器 MCP server 的接入路径不受影响。

