# eos mcp serve — 标准 MCP Server

`eos mcp serve` 把 EOS 作为**标准 MCP（Model Context Protocol）Server** 暴露给外部
agent 或宿主。任何符合 MCP 规范的客户端（Claude Desktop、Cursor、自研 agent 等）
都能像调用普通 MCP server 一样调用 EOS 的工具能力。

## 与 eos serve 的区别

| | `eos serve` | `eos mcp serve` |
|---|---|---|
| 协议 | EOS 私有 JSON-RPC（`session/create` 等） | 标准 MCP（`tools/list`、`tools/call`） |
| 适合 | 深度集成、需要 turn 编排和事件流的宿主 | 任意 MCP 客户端接入 |
| transport | stdio | stdio + sse |
| 能力范围 | 全部 ~135 个 method | 工具调用（MVP） |

## Transport

```bash
# stdio（本地宿主）
eos mcp serve --transport stdio --workspace "/abs/workspace"

# sse（远程/网络宿主）
eos mcp serve --transport sse --listen 127.0.0.1:8765 --workspace "/abs/workspace"
```

基于 [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) 实现，内置 stdio 与 sse transport。

## 暴露能力（MVP 范围）

首期只暴露 MCP 标准 `tools/*`，不暴露 `resources/*` / `prompts/*`。

### tools/list

映射 EOS 工具目录（`tool/catalog`）。只暴露 `Invocable == true` 的工具（capability-only
项不伪装成可调用 tool）。每个工具保留原名，schema 从 EOS 工具的参数定义构造。

### tools/call

调用 EOS 工具执行器（`tool/execute`）：

1. 从 MCP 请求取 tool name + arguments。
2. 注入会话 ID（见「会话语义」）。
3. 调用 `engine.Tools().Execute(ctx, ToolRequest{SessionID, Name, Args})`。
4. 把 `ToolResult` 映射为 MCP `CallToolResult`。

## 会话语义

EOS 工具执行依赖会话上下文。MCP server 采用「**连接默认会话 + 可选显式覆盖**」模型：

- 每个 MCP server 启动时创建一个默认 EOS 会话（`session/current` 失败则 `session/create`）。
- 所有 `tools/call` 默认绑定到该会话。
- 调用方可通过工具参数的 `_meta.session_id` 显式指定其它会话（覆盖默认）。

## 审批与询问（首期不自动放行）

对高风险工具调用、需要用户审批或回答询问的场景，MVP **不自动放行**：

- 工具执行返回非 success 状态时，MCP `CallToolResult` 标记 `isError=true`，content 携带结构化提示（pending approval / needs input 等）。
- 外部 agent 据此决定后续动作（提示用户、换工具、或通过 `eos serve` 的 `approval/respond` 闭环）。

> 设计原则（AGENTS.md「充分信任但不做无谓限制」）：不替用户自动批准高风险操作，
> 但也不把 agent 关进笼子——能力边界交给模型和 prompt，审批只防低级事故。

## MVP 边界（明确不在首期）

以下能力留待后续迭代，**首期不实现**，本文档也不展开：

- MCP `resources/list` / `resources/read`（`eos://sessions`、`eos://catalog/tools` 等）
- MCP `prompts/list` / `prompts/get`
- MCP 控制工具集（`eos_session_create`、`eos_approval_resolve`、`eos_inquiry_resolve`、`eos_task_*` 等）
- SSE 连接级会话隔离精细化（MVP 用单默认会话）
- `eos mcp list/add`（管理外部 MCP server 的客户端命令）—— `eos mcp` 父命令已预留扩展位

## 客户端配置示例

### stdio（Claude Desktop 风格）

```jsonc
{
  "mcpServers": {
    "eos": {
      "command": "eos",
      "args": ["mcp", "serve", "--transport", "stdio", "--workspace", "/abs/workspace"]
    }
  }
}
```

### sse

客户端指向 `http://127.0.0.1:8765`（由 `--listen` 指定），按 MCP SSE transport 连接。

## 启动选项

| flag | 说明 |
|---|---|
| `--transport` | `stdio`（默认）或 `sse` |
| `--workspace` | 工作区根目录（默认当前目录） |
| `--listen` | SSE 监听地址（默认 `127.0.0.1:8765`，仅 sse） |
| `--access-mode` | `read-only` / `workspace-write` / `danger-full-access` |
| `--approval-mode` | `untrusted` / `on-failure` / `on-request` / `never` |
| `--sandbox-mode` | `workspace` / `full_access`（legacy 别名） |
| `--dangerously-skip-permissions` | 等价 `--access-mode danger-full-access --approval-mode never` |
