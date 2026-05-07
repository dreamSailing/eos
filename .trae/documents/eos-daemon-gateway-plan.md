## Summary

为 EOS 增加“后台常驻 + 统一 HTTP 网关 + Web 控制台 + 定时任务”能力，使 `eos` 可以以守护进程方式长期运行，在默认工作区中提供非交互式服务，并通过网页操作后台 EOS，适合作为 MCP Server 的长期宿主。

本计划基于已确认方向：

- 使用显式后台入口，而不是隐式改写所有现有命令行为
- 统一 HTTP 网关同时承载后台管理 API、网页控制台，以及对外的 MCP 网络入口
- 后台模式默认工作区使用 `internal/config/workspaces.go` 中的默认工作区
- 第一版定时任务同时支持两类目标：
  - 触发 EOS 会话/工具调用
  - 执行任意 Shell 命令
- 服务端以非交互方式运行，不依赖 TUI

对于用户跳过未确认项，本计划采用保守默认决策：

- HTTP 网关默认只监听 `127.0.0.1`
- 第一版不做登录系统，先不引入复杂认证；后续可预留静态 token 扩展点
- Web 控制台第一版聚焦运维与控制，不做完整配置中心
- 调度底层使用标准 5 段 cron 表达式；网页可后续再补“简单模式”编辑

## Current State Analysis

### 现有可复用能力

- `internal/cli/root.go`
  - 当前根命令已注册 `bridge`、`mcp`、`serve`、`update`
  - 可以自然扩展新的 `daemon` 子命令而不破坏现有 CLI

- `internal/cli/mcp.go`
  - 已有 `eos mcp serve`
  - 已支持 `stdio` 与 `sse`
  - 已要求传入 `--workspace`
  - 说明 EOS 已能前台作为 MCP Server 运行

- `internal/mcp/server.go`
  - 标准 MCP Server 已存在
  - 支持 `stdio` 和 `sse`
  - `ServerOptions` 已包含 `ListenAddr`、`BaseURL`、`DefaultWorkspacePath`
  - 适合作为后台守护进程中“对外 MCP 网络能力”的核心复用点

- `internal/mcp/server_transport_sse.go`
  - 已实现 HTTP SSE 监听
  - 当前仅负责 MCP SSE server 生命周期，不包含后台管理 API 或 Web 页面

- `internal/mcp/streamable_http.go`
  - 已有 Streamable HTTP 客户端
  - 说明仓库已经具备“HTTP 化 MCP 交互”相关经验，可为后续网关转发或自检复用

- `internal/serve/server.go`
  - 已有会话、审批、询问、任务、结果缓存、事件通知、工具执行等完整服务端模型
  - 会话摘要、待审批、待提问、任务状态等都已经具备结构化表示
  - 很适合作为 Web 控制台与后台 API 的领域模型参考

- `internal/serve/session_store.go`
  - 已支持 session 状态持久化与恢复
  - 当前默认落盘位置与工作区绑定：`<workspace>/.eos/serve/sessions.json`
  - 说明守护进程具备“跨重启恢复状态”的实现基础

- `internal/config/workspaces.go`
  - 已提供 `DefaultWorkspacePath()`
  - 已提供 `EnsureDefaultWorkspaceDir()`
  - 已明确默认工作区为 `~/.eos/workspace`
  - 满足“后台启动时工作区为默认工作区”的基础能力

- `internal/config/config.go`
  - 已有全局配置读写能力
  - 已支持 MCP 客户端配置、工作区状态等持久化
  - 适合作为守护进程状态文件、网关配置、调度配置的统一配置入口

- `internal/tools/bg/manager.go`
  - 已实现后台 Shell 任务管理器
  - 已支持 `Start/List/Info/Tail/Kill/CleanupFinished`
  - 可直接复用到“守护进程中的命令型定时任务”和任务日志查看

- `internal/tools/bg_tools.go`
  - 已将后台任务管理暴露为结构化工具
  - 可为后台 API 与网页控制台提供一致的行为参考

- `internal/toolapi/interfaces.go` 与 `internal/toolapi/impl/tasks.go`
  - 已有统一任务视图抽象 `Tasks`
  - 当前可聚合 shell task、todo、agent task
  - 可作为 Web 控制台任务面板的底层读取接口

### 现有缺口

- 缺少守护进程入口
  - 当前没有 `eos daemon start/status/stop`
  - 现有 `eos mcp serve` 和 `eos serve` 都是前台阻塞运行

- 缺少统一 HTTP 网关
  - 当前仓库没有通用 `http.ServeMux` / `HandleFunc` / 静态文件服务实现
  - 唯一现成 HTTP 服务是 MCP SSE transport，不含后台 API、网页资源与统一路由

- 缺少 Web 前端资源
  - 当前仓库没有前端目录、HTML 模板、静态 JS/CSS 或嵌入式静态站点
  - 需要从零增加最小控制台页面

- 缺少守护进程状态持久化
  - session 可持久化，但进程自身 PID、监听地址、运行模式、定时任务定义等尚无专门状态模型

- 缺少定时任务调度器
  - 当前只有后台任务执行器，没有 cron 调度、任务定义、启停、持久化和重载逻辑

- 缺少“后台默认工作区”自动化逻辑
  - 目前 `internal/cli/mcp.go` 强制要求 `--workspace`
  - 尚未提供 daemon 模式自动回落到 `DefaultWorkspacePath()`

- 缺少面向网页的后台管理 API
  - 当前的对外接口是 MCP 协议与私有 `serve` JSON-RPC
  - 没有适合浏览器直接调用的 REST/JSON API

## Proposed Changes

### 1. 新增守护进程命令与状态管理

涉及文件：

- 新增 `internal/cli/daemon.go`
- 更新 `internal/cli/root.go`
- 新增 `internal/daemon/manager.go`
- 新增 `internal/daemon/state.go`
- 新增 `internal/daemon/lock.go`

变更目标：

- 提供显式后台入口：
  - `eos daemon start`
  - `eos daemon status`
  - `eos daemon stop`
  - 可选 `eos daemon restart`
- 让 EOS 后台进程成为统一宿主，长期运行 MCP 网关与定时调度器

实现方式：

- `internal/cli/daemon.go`
  - 新增 `daemon` 根子命令
  - `start` 默认使用 `config.DefaultWorkspacePath()`
  - 如果默认工作区不存在，调用后续实现中的确保逻辑创建
  - 参数建议包含：
    - `--workspace`：可选，省略时回落默认工作区
    - `--listen`：HTTP 网关监听地址，默认 `127.0.0.1:8765` 或独立后台端口
    - `--mcp-base-url`：可选，对外显示/反代场景使用
    - `--session-store`
    - `--state-file`
    - `--schedule-file`
    - `--allowed-tools`
    - `--sandbox-mode`
    - `--policy`
  - `status` / `stop` 通过状态文件和 PID/锁文件识别后台实例

- `internal/daemon/manager.go`
  - 负责后台实例启动、停止、探活、状态读取
  - 统一封装“前台初始化 -> 后台化/子进程托管 -> 写状态文件”
  - Linux 下优先采用 `exec.Command` 派生子进程并脱离当前命令会话的方式；避免依赖 systemd

- `internal/daemon/state.go`
  - 定义守护进程状态文件结构，例如：
    - `pid`
    - `started_at`
    - `listen_addr`
    - `workspace`
    - `session_store_path`
    - `schedule_store_path`
    - `mcp_sse_path`
    - `web_base_url`
  - 落盘位置建议放在：
    - `~/.eos/daemon/state.json`
    - 或 `<workspace>/.eos/daemon/state.json`
  - 本计划采用“全局守护进程 + 默认工作区宿主”的思路，优先使用全局状态目录 `~/.eos/daemon`

- `internal/daemon/lock.go`
  - 用文件锁避免重复启动多个后台守护进程

决策：

- 第一版按“单实例守护进程”设计
- 单实例后台只承载一个默认工作区
- 如需多工作区守护进程，后续再扩展多实例或多 tenant 模型

### 2. 新增统一 HTTP 网关宿主

涉及文件：

- 新增 `internal/gateway/server.go`
- 新增 `internal/gateway/routes.go`
- 新增 `internal/gateway/types.go`
- 新增 `internal/gateway/context.go`
- 视复用情况更新 `internal/mcp/server.go`
- 视复用情况更新 `internal/mcp/server_transport_sse.go`

变更目标：

- 提供一个统一的 HTTP 服务器，挂载：
  - Web 控制台页面
  - 后台管理 API
  - MCP SSE 接入点
- 避免再让守护进程同时维护多个彼此独立的 HTTP 监听器

实现方式：

- `internal/gateway/server.go`
  - 基于 `http.Server` + `http.ServeMux`
  - 统一组装所有路由
  - 负责生命周期管理、关闭、健康检查

- `internal/gateway/routes.go`
  - 规划建议路由：
    - `GET /healthz`
    - `GET /api/status`
    - `GET /api/sessions`
    - `GET /api/sessions/{id}`
    - `POST /api/approvals/{id}/resolve`
    - `POST /api/inquiries/{id}/resolve`
    - `GET /api/tasks`
    - `GET /api/tasks/{id}/logs`
    - `POST /api/tasks/{id}/kill`
    - `GET /api/schedules`
    - `POST /api/schedules`
    - `PUT /api/schedules/{id}`
    - `DELETE /api/schedules/{id}`
    - `POST /api/schedules/{id}/trigger`
    - `GET /` 与静态资源路由
    - `/mcp/sse` 或 `/mcp` 作为 MCP HTTP 入口

- `internal/gateway/context.go`
  - 封装共享依赖：
    - MCP Server
    - 会话读写服务
    - 守护进程状态读取器
    - 调度器
    - 后台任务管理器

与现有 MCP 的关系：

- 网关不重写 MCP 核心逻辑
- 复用 `internal/mcp.Server`
- 对于 HTTP MCP 入口，优先采用将 `internal/mcp/server_transport_sse.go` 调整为“可挂载 handler”模式
- 若当前 `mcp-go` SSE server 只能自带启动整个 HTTP server，则在 `internal/mcp` 增加一个“返回 handler/attach mux”包装层，让网关统一监听

### 3. 让后台模式默认工作区自动生效

涉及文件：

- 更新 `internal/cli/daemon.go`
- 可能更新 `internal/cli/mcp.go`
- 更新 `internal/config/workspaces.go`

变更目标：

- 后台启动时，若未显式指定工作区，则自动使用默认工作区
- 守护进程启动前自动确保默认工作区存在

实现方式：

- `daemon start` 中直接调用：
  - `config.DefaultWorkspacePath()`
  - `config.EnsureDefaultWorkspaceDir()`
- 启动守护进程时将该路径注入：
  - MCP Server `DefaultWorkspacePath`
  - 会话存储默认路径
  - 调度任务默认工作目录

兼容性决策：

- 不强改现有 `eos mcp serve --workspace required` 行为
- “默认工作区自动生效”仅先用于新的 `daemon start`
- 后续如需要，再决定是否让 `eos mcp serve` 也支持省略 `--workspace`

### 4. 为守护进程抽取后台领域服务

涉及文件：

- 新增 `internal/daemon/service.go`
- 可能新增 `internal/daemon/runtime.go`
- 复用：
  - `internal/mcp/server.go`
  - `internal/serve/server.go`
  - `internal/toolapi/impl/services.go`
  - `internal/tools/bg/manager.go`

变更目标：

- 把守护进程中的几个长期运行组件组合在一起：
  - MCP Server
  - HTTP 网关
  - 定时调度器
  - 统一状态存储

实现方式：

- 定义 `DaemonService` 或等价结构，持有：
  - `toolapi.Services`
  - `*mcp.Server`
  - `*gateway.Server`
  - `*scheduler.Service`
  - 守护进程状态仓库
- 提供：
  - `Start(ctx)`
  - `Shutdown(ctx)`
  - `Status()`

说明：

- 这里不建议直接复用 `internal/serve.Server` 作为外部 API，因为它是按行 JSON-RPC over stdio
- 但其会话/审批/任务/预览模型需要复用或参考，以便后台 API 与 Web 展示的结构稳定

### 5. 新增定时任务调度器

涉及文件：

- 新增 `internal/scheduler/service.go`
- 新增 `internal/scheduler/store.go`
- 新增 `internal/scheduler/types.go`
- 新增 `internal/scheduler/cron.go`
- 新增 `internal/scheduler/runner_eos.go`
- 新增 `internal/scheduler/runner_shell.go`

变更目标：

- 后台模式支持持久化、可管理、可触发的定时任务
- 同时覆盖：
  - EOS 会话/工具调用类任务
  - Shell 命令类任务

实现方式：

- `types.go`
  - 定义调度任务实体：
    - `id`
    - `name`
    - `enabled`
    - `cron`
    - `timezone`（第一版可先固定本地时区，字段保留）
    - `kind`：`eos_call` / `shell`
    - `workspace`
    - `payload`
    - `last_run_at`
    - `next_run_at`
    - `last_status`
    - `last_error`

- `store.go`
  - JSON 文件持久化
  - 推荐路径：`~/.eos/daemon/schedules.json`

- `cron.go`
  - 封装 cron 表达式解析与下一次触发时间计算
  - 若仓库中尚无 cron 依赖，则新增轻量依赖，例如标准 cron 解析库

- `service.go`
  - 后台启动时加载所有任务
  - 维护调度循环
  - 提供 `List/Create/Update/Delete/Enable/Disable/TriggerNow`

- `runner_eos.go`
  - 负责创建或复用后台 EOS session，并发起指定工具调用
  - 复用 `toolapi.Executor` 和/或 `internal/mcp.Server` 的会话能力
  - 第一版建议直接通过 `toolapi.Executor` 执行单个或一组结构化工具调用，避免“自己通过 HTTP 再调自己”造成环路

- `runner_shell.go`
  - 复用 `internal/tools/bg/manager.go`
  - 将 shell 定时任务作为后台任务启动
  - 保留日志 tail 能力供网页查看

关键决策：

- EOS 调度任务不依赖浏览器或交互输入
- 如果任务触发审批/提问，第一版行为为：
  - 记录为 pending
  - 在网页控制台展示需要人工处理
  - 不自动跳过、不自动放行

### 6. 新增面向浏览器的后台管理 API

涉及文件：

- 新增 `internal/gateway/api_status.go`
- 新增 `internal/gateway/api_sessions.go`
- 新增 `internal/gateway/api_tasks.go`
- 新增 `internal/gateway/api_schedules.go`
- 新增 `internal/gateway/api_prompts.go`

变更目标：

- 为 Web 控制台提供稳定 JSON API
- 同时也方便未来脚本或其他系统通过 HTTP 管理后台 EOS

接口范围：

- `status`
  - 返回守护进程状态、监听地址、默认工作区、MCP 入口信息、版本

- `sessions`
  - 列出 session
  - 查看单个 session
  - 返回状态、标题、预览、待审批、待提问、运行中请求等摘要
  - 数据模型尽量复用 `internal/serve/server.go` 中 `sessionInfoLocked` 的语义

- `prompts`
  - 列出待审批、待提问
  - 提交审批决定
  - 提交问题答案

- `tasks`
  - 列表
  - 查询日志 tail
  - 杀掉 shell 后台任务
  - 复用 `toolapi.Tasks()` 与 `bg.Manager`

- `schedules`
  - 增删改查
  - 启停
  - 立即执行
  - 查看最近执行结果

说明：

- 第一版不做复杂 REST 版本治理，统一走 JSON
- 错误结构保持简洁，优先满足网页控制台调用

### 7. 新增 Web 控制台

涉及文件：

- 新增 `internal/gateway/web.go`
- 新增 `internal/gateway/web_embed.go`
- 新增目录 `internal/gateway/web/`
  - `index.html`
  - `app.css`
  - `app.js`

变更目标：

- 提供一个轻量网页，用于通过网关操作后台 EOS
- 第一版覆盖核心运维能力，而不是完整系统设置

页面范围：

- 首页仪表盘
  - 守护进程状态
  - 默认工作区
  - MCP 入口地址
  - 待处理审批/提问数量
  - 定时任务数量与最近执行概况

- Session 面板
  - 列表
  - 基本详情
  - 待审批/待提问可视化

- 任务面板
  - 当前后台任务列表
  - 查看 shell 日志 tail
  - 终止可终止任务

- 定时任务面板
  - 新增/编辑/删除/启停
  - 支持两种任务类型：
    - EOS 调用
    - Shell 命令

- 提示处理面板
  - 审批 allow once / allow session / deny
  - 询问回答 option/text

实现方式：

- 采用 Go `embed` 嵌入静态页面
- 页面尽量原生 HTML/CSS/JS，避免额外前端构建链
- 所有操作通过网关 JSON API 完成

约束：

- 第一版不引入 SPA 框架和 npm 构建
- 目标是“可直接内置到 Go 二进制并随守护进程分发”

### 8. 将 MCP HTTP 能力挂到统一网关下

涉及文件：

- 更新 `internal/mcp/server_transport_sse.go`
- 可能新增 `internal/mcp/server_transport_http.go`
- 更新 `internal/gateway/routes.go`

变更目标：

- 通过统一 HTTP 网关对外提供 MCP 服务
- 让网页控制台和 MCP 客户端共用同一后台实例与监听端口

实现方式：

- 将现有 `RunSSE(ctx)` 拆成两层：
  - 一层负责创建 MCP 的 HTTP handler/transport 适配器
  - 一层负责独立启动 HTTP server（保留给前台 `eos mcp serve`）
- 守护进程模式下由 `gateway.Server` 统一挂载 MCP 路由
- 前台 `eos mcp serve --transport sse` 继续保持原有行为，不被破坏

结果：

- `eos mcp serve --transport sse` 继续可独立使用
- `eos daemon start` 则通过统一网关暴露：
  - Web 控制台
  - 后台管理 API
  - MCP SSE 入口

### 9. 补充守护进程与网关的测试

涉及文件：

- 新增 `internal/daemon/manager_test.go`
- 新增 `internal/daemon/state_test.go`
- 新增 `internal/gateway/server_test.go`
- 新增 `internal/gateway/api_schedules_test.go`
- 新增 `internal/gateway/api_sessions_test.go`
- 新增 `internal/scheduler/service_test.go`
- 新增 `internal/scheduler/store_test.go`
- 视需要调整：
  - `internal/mcp/server_test.go`
  - `internal/serve/server_test.go`

测试范围：

- 守护进程状态文件读写与单实例锁
- `daemon start/status/stop` 的最小行为
- 默认工作区自动回落与自动创建
- HTTP 网关 `healthz` / `status`
- Web 静态资源可访问
- MCP 路由成功挂载在统一网关下
- 定时任务 CRUD、持久化、重载
- cron 触发 EOS 调用任务
- cron 触发 shell 命令任务
- 待审批/待提问在 API 中可见并可闭环处理

## Assumptions & Decisions

- 决策：新增 `eos daemon`，而不是让现有 `eos mcp serve` 默认后台化
- 决策：后台模式默认工作区使用 `config.DefaultWorkspacePath()`
- 决策：后台模式优先服务 MCP 使用场景，因此守护进程只承载一个默认工作区
- 决策：统一 HTTP 网关承载 Web、管理 API 与 MCP 网络入口
- 决策：第一版网页控制台聚焦“运行态控制”，不做完整配置编辑中心
- 决策：定时任务底层使用 cron 表达式存储
- 决策：Shell 定时任务通过现有 `bg.Manager` 执行
- 决策：EOS 定时任务通过内部执行器触发，不通过 HTTP 自调自身
- 决策：审批/询问在后台模式下仍保持人工闭环，不自动通过
- 决策：第一版默认仅监听 `127.0.0.1`，不实现登录系统
- 假设：`mcp-go` 当前 SSE server 能拆分或包装为可挂载到现有 `http.ServeMux` 的形式；若不能，则在 `internal/mcp` 增加适配层
- 假设：仓库可以接受新增轻量 cron 解析依赖；若不希望加依赖，则改为内置最小 cron 解析实现
- 假设：Linux 沙箱/用户环境下允许通过普通子进程实现后台化，不要求 systemd 集成

## Verification Steps

实现后按以下顺序验证：

1. `go test ./internal/daemon/... ./internal/gateway/... ./internal/scheduler/...`
2. `go test ./internal/mcp/... ./internal/serve/... ./internal/toolapi/...`
3. `go test ./...`
4. `go build ./...`
5. 手工验证守护进程：
   - `eos daemon start`
   - 确认默认工作区自动生效
   - `eos daemon status`
   - `eos daemon stop`
6. 手工验证统一网关：
   - 打开 `http://127.0.0.1:<port>/`
   - 确认首页能显示后台状态、工作区、MCP 入口
   - `GET /healthz` 返回正常
7. 手工验证 MCP：
   - 用 MCP 客户端连接统一网关暴露的 SSE 入口
   - 执行 `initialize`
   - 列出工具
   - 调用只读工具
8. 手工验证审批/提问：
   - 触发高风险工具调用
   - 确认网页/API 中可见 pending approval
   - 在网页上完成 resolve 后调用继续推进
9. 手工验证定时任务：
   - 新建一个 shell cron 任务
   - 新建一个 EOS cron 任务
   - 校验任务持久化、重启恢复、立即触发、最近结果展示
