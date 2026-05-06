## Summary

为 EOS 增加标准 MCP Server 能力，使 `eos` 可以作为 MCP 服务端被其他 agent / 宿主调用。第一阶段按已确认方向落地：

- 兼容标准 MCP，而不是继续复用现有私有 JSON-RPC 协议作为对外协议
- 优先复用现有 `internal/serve`、`internal/toolapi`、`internal/tools` 的执行、审批、任务与目录能力
- 传输层目标包含 `stdio` 和 `SSE`
- 对外暴露 `tools` 与 `resources`
- 调用模型保留 EOS 的会话语义，而不是退化成完全无状态工具转发

未被用户继续确认的细节，本计划采用保守默认决策：

- 高风险审批与用户提问不自动放行
- 通过 MCP 工具 + 资源形式暴露待审批/待回答状态与解决入口
- 每个 MCP 连接自动创建一个默认 EOS 会话，同时允许显式传 `session_id` 覆盖默认会话

## Current State Analysis

### 现有可复用能力

- `internal/serve/server.go`
  - 已有完整的会话生命周期、工具目录、执行、审批、询问、任务管理与事件发送逻辑
  - 当前协议是自定义按行 JSON-RPC 2.0，方法名为 `session.create`、`tool.list`、`tool.execute`、`prompt.resolve` 等
  - 当前仅支持 `stdio`

- `internal/toolapi/interfaces.go`
  - 已抽象出 `Services` / `Catalog` / `Executor` / `Tasks`
  - 适合作为 MCP Server 的统一后端能力层

- `internal/toolapi/impl/catalog.go`
  - 已能列出 builtin/runtime/agent/plugin/skill/mcp/lsp 能力目录
  - 可直接作为 MCP `tools/list` 的来源之一

- `internal/toolapi/impl/executor.go`
  - 已能把结构化 `ToolCall` 映射为 EOS 工具执行结果 `ToolResult`
  - 是 MCP `tools/call` 的最直接复用点

- `internal/cli/serve.go`
  - 已有服务模式 CLI 入口，但只启动私有 `serve` 协议

- `internal/mcp/mcp.go`
  - 当前是 MCP client manager，用于把外部 MCP server 接入 EOS
  - 说明仓库已经依赖 `mcp-go`，但目前没有标准 MCP server 入口

- `internal/mcp/mcp_adapter.go` 与 `internal/mcp/streamable_http.go`
  - 已存在 MCP Streamable HTTP 客户端适配代码
  - 说明 MCP 语义已进入代码库，但方向仍偏“客户端”

### 现有缺口

- 缺少标准 MCP Server 实现
  - 当前没有 `tools/list` / `tools/call` / `resources/list` / `resources/read` 的服务端处理器
  - 当前没有标准 MCP 初始化握手与 capability 宣告

- 缺少 MCP Server 的 CLI 暴露
  - 现有根命令只注册了 `bridge` 与 `serve`
  - 没有 `eos mcp serve` 或等价入口

- 缺少 SSE 传输实现
  - 私有 `serve` 与现有 MCP client 代码都没有可直接复用的 MCP SSE server 端实现

- 会话语义未标准化到 MCP 资源/工具
  - 现有会话、审批、询问、任务都是私有 JSON-RPC 方法，不是 MCP tools/resources

## Proposed Changes

### 1. 新增标准 MCP Server 层

新增文件：

- `internal/mcp/server.go`
- `internal/mcp/server_session.go`
- `internal/mcp/server_tools.go`
- `internal/mcp/server_resources.go`
- `internal/mcp/server_transport_stdio.go`
- `internal/mcp/server_transport_sse.go`

变更目标：

- 基于 `github.com/mark3labs/mcp-go` 新增 EOS 自己的 MCP Server 实现
- 对外提供标准 MCP 初始化、工具枚举、工具调用、资源枚举、资源读取
- 将“协议适配”放在 `internal/mcp`，避免把私有 `serve` 与标准 MCP 混在同一层

实现方式：

- `server.go`
  - 定义 `Server`、`Options`、连接级上下文、默认会话装配逻辑
  - 持有 `toolapi.Services`
  - 负责创建每个连接的默认 EOS session 状态

- `server_session.go`
  - 从 `internal/serve/server.go` 抽出可复用的 session 状态模型与最小执行上下文
  - 明确 MCP 层的默认规则：
    - 连接建立后自动创建默认 session
    - 每次 `tools/call` 若未传 `session_id`，走默认 session
    - 若传 `session_id`，优先使用显式会话
  - 维护审批、待回答问题、任务、结果缓存、最近预览等会话态

- `server_tools.go`
  - 将 `toolapi.Catalog().List()` 结果映射为 MCP `Tool`
  - 将 `toolapi.Executor.Execute()` 结果映射为 MCP `CallToolResult`
  - 为需要会话的调用统一注入 `session_id`
  - 对 `ask_user_question`、高风险工具、审批流程做 MCP 语义封装

- `server_resources.go`
  - 暴露结构化资源而不是只靠工具返回文本
  - 第一阶段资源集合默认包含：
    - `eos://sessions`
    - `eos://sessions/{id}`
    - `eos://sessions/{id}/approvals`
    - `eos://sessions/{id}/inquiries`
    - `eos://sessions/{id}/tasks`
    - `eos://catalog/tools`
    - `eos://catalog/capabilities`
    - `eos://runtime/mcp-status`
    - `eos://runtime/lsp-status`
    - `eos://runtime/version`
  - `resources/read` 返回 JSON 文本资源，优先结构化、可被 agent 继续解析

- `server_transport_stdio.go`
  - 实现 MCP stdio server 启动入口

- `server_transport_sse.go`
  - 实现 MCP SSE server 启动入口
  - 若 `mcp-go` 已提供 SSE server transport，则直接复用
  - 若不满足 EOS 所需连接/会话控制，则只在 transport 层补 HTTP/SSE 封装，不重复业务逻辑

### 2. 抽取/复用现有 serve 的核心业务逻辑

涉及文件：

- `internal/serve/server.go`
- 可能新增 `internal/serve/session_core.go` 或者改由 `internal/mcp/server_session.go` 复用逻辑

变更目标：

- 避免在 MCP Server 中重复实现以下能力：
  - 工具访问控制
  - workspace 路径约束
  - 审批摘要与 TTL
  - 询问/审批状态缓存
  - 任务查询与取消

实现方式：

- 把现有 `serve.Server` 里与“协议耦合”弱、与“执行模型”强相关的逻辑下沉成共享 helper
- 重点复用这些行为：
  - `currentToolDefinitions`
  - `buildPreview`
  - `checkWorkspaceConstraints`
  - `isApproved`
  - `ensurePendingApproval`
  - `ensurePendingInquiry`
  - `sessionInfoLocked` 对应的会话摘要装配

约束：

- 不改动 `serve` 的现有对外协议行为
- 抽取后应保证 `serve` 现有测试仍然可以沿用或轻量修正

### 3. 定义 MCP 工具映射策略

涉及文件：

- `internal/mcp/server_tools.go`
- `internal/toolapi/impl/catalog.go`
- 视需要新增 `internal/mcp/server_schema.go`

变更目标：

- 把 EOS 现有工具目录稳定映射为标准 MCP tools
- 让其他 agent 能像调用普通 MCP server 一样调用 EOS

实现方式：

- 目录来源：
  - 使用 `toolapi.Services.Catalog().List()` 获取可见工具定义
  - 仅把 `Invocable == true` 的定义映射成 MCP tool
  - capability-only 项继续通过资源暴露，不伪装成可调用 tool

- MCP tool schema 设计：
  - 每个 EOS 工具保留原名，减少二次映射成本
  - 对所有工具统一追加可选参数：
    - `session_id`
    - `workspace_root`（仅当要显式覆盖默认工作区时启用）
  - 对高风险工具调用返回结构化结果：
    - 成功时：映射 `toolapi.ToolResult`
    - 需要审批时：返回 `isError=true` + 结构化审批信息，且同步写入相关 `eos://sessions/{id}/approvals` 资源
    - 需要用户回答时：返回 `isError=true` + 结构化 inquiry 信息，且同步写入 `eos://sessions/{id}/inquiries` 资源

- 新增 MCP 侧控制工具：
  - `eos_session_create`
  - `eos_session_list`
  - `eos_session_get`
  - `eos_session_close`
  - `eos_approval_resolve`
  - `eos_inquiry_resolve`
  - `eos_task_list`
  - `eos_task_kill`
  - `eos_task_resume`
  - `eos_task_close`

说明：

- 这样既保留“连接自动默认会话”的简单体验，也保留“显式会话控制”的高级能力
- 不把私有 JSON-RPC 方法直接暴露给外部客户端，而是以标准 MCP tool/resource 重新组织

### 4. 定义 MCP 资源模型

涉及文件：

- `internal/mcp/server_resources.go`

变更目标：

- 让外部 agent 不仅能调用工具，还能读取 EOS 当前运行态和会话态
- 满足用户已确认的“工具 + resources”范围

实现方式：

- 资源 URI 采用固定命名空间：
  - `eos://sessions`
  - `eos://sessions/{id}`
  - `eos://sessions/{id}/approvals`
  - `eos://sessions/{id}/inquiries`
  - `eos://sessions/{id}/tasks`
  - `eos://catalog/tools`
  - `eos://catalog/capabilities`
  - `eos://runtime/mcp-status`
  - `eos://runtime/lsp-status`
  - `eos://runtime/version`

- 资源内容类型：
  - 默认使用 `application/json`
  - 内容统一输出稳定 JSON 结构，避免只给文本描述

- 资源数据来源：
  - session / approval / inquiry / tasks：复用 `serve` 层抽取出的 session state
  - catalog：复用 `toolapi.Catalog`
  - mcp status：复用 `internal/tools/mcp_status_tool.go` / `internal/mcp/mcp.go` 相关能力
  - lsp / version：复用现有配置、探测与版本模块

### 5. 新增 CLI 入口并补传输选项

涉及文件：

- `internal/cli/root.go`
- 新增 `internal/cli/mcp.go`
- 可能轻调 `internal/cli/serve.go`

变更目标：

- 提供标准启动方式，使其他 agent 能直接配置 EOS 作为 MCP server

实现方式：

- 新增子命令：
  - `eos mcp serve --transport stdio --workspace /abs/path`
  - `eos mcp serve --transport sse --workspace /abs/path --listen 127.0.0.1:PORT`

- `mcp serve` 参数第一阶段建议包含：
  - `--transport`：`stdio` / `sse`
  - `--workspace`
  - `--allowed-tools`
  - `--sandbox-mode`
  - `--policy`
  - `--session-store`
  - `--require-approval-digest`
  - `--listen` / `--base-url`（SSE 使用）

- 保留现有 `eos serve`
  - 不改名、不破坏当前 bridge / IDE 集成
  - `eos serve` 继续服务私有协议
  - `eos mcp serve` 负责标准 MCP

### 6. 扩展文档与接入说明

涉及文件：

- `README.md`
- `README.en.md`
- 新增 `internal/docs/mcp/SERVER.md`

变更目标：

- 让外部 agent / 宿主能直接知道如何把 EOS 当作 MCP server 使用

实现方式：

- 在 README 增补：
  - EOS 既可消费外部 MCP，也可作为 MCP server 对外提供能力
  - stdio / SSE 启动示例
  - 客户端配置示例

- 新增 `internal/docs/mcp/SERVER.md`
  - 描述支持的 transport
  - 描述暴露的 tool/resource 范围
  - 描述默认会话与显式 `session_id` 规则
  - 描述审批/询问如何通过 `eos_approval_resolve` / `eos_inquiry_resolve` 闭环

### 7. 测试补齐

涉及文件：

- 新增 `internal/mcp/server_test.go`
- 新增 `internal/mcp/server_resources_test.go`
- 新增 `internal/mcp/server_tools_test.go`
- 视抽取情况调整：
  - `internal/serve/server_test.go`
  - `internal/serve/capabilities_test.go`

变更目标：

- 保证新增标准 MCP server 能力不破坏现有私有 `serve`

测试范围：

- MCP 初始化与 capability 宣告
- stdio transport 下的 `tools/list`
- `tools/call` 到 EOS builtin tool 的成功路径
- 审批必需场景能返回可解析的 pending 信息
- `ask_user_question` 能产生 inquiry 并通过 resolve 工具闭环
- `resources/list` / `resources/read` 能返回 session、catalog、runtime 资源
- 显式 `session_id` 与默认会话回退逻辑
- SSE transport 的基本连接与最小调用链

## Assumptions & Decisions

- 决策：标准 MCP server 与现有私有 `serve` 并存，不做协议替换
- 决策：优先通过新增 `eos mcp serve` 子命令落地，而不是把 `eos serve` 改造成双协议大杂烩
- 决策：工具执行后端继续复用 `toolapi.Services` 和 `tools.Manager`
- 决策：会话语义保留，但在 MCP 层采用“默认连接会话 + 可选显式 session_id”的折中模型
- 决策：resources 第一阶段同时覆盖“会话/目录”和“运行态”，避免后续再补第二套资源命名
- 假设：`mcp-go` 当前版本具备可用的 server 侧基础设施；若 SSE server transport 支持不足，只在传输封装层补齐，不改变上层接口设计
- 假设：第一阶段不暴露 MCP prompts；如果后续需要，可在当前资源与控制工具模型稳定后追加
- 假设：审批与用户交互不自动通过，必须由外部 agent 继续调用 EOS 提供的 resolve 工具闭环

## Verification Steps

实现后按以下顺序验证：

1. `go test ./internal/mcp/... ./internal/serve/... ./internal/toolapi/...`
2. `go test ./...`
3. `go build ./...`
4. 手工验证 stdio MCP：
   - 启动 `eos mcp serve --transport stdio --workspace /abs/workspace`
   - 使用标准 MCP 客户端执行 `initialize`
   - 校验 `tools/list`
   - 调用只读工具，如 `read`
   - 调用高风险工具并确认返回 pending approval
5. 手工验证 resources：
   - 校验 `resources/list`
   - 读取 `eos://catalog/tools`
   - 读取 `eos://sessions`
   - 在产生审批/询问后读取对应 session 资源
6. 手工验证 SSE：
   - 启动 `eos mcp serve --transport sse --listen 127.0.0.1:PORT --workspace /abs/workspace`
   - 用支持 SSE 的 MCP 客户端完成初始化、列工具、调工具、读资源
