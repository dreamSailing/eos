# eos serve — 本地工具服务 JSON-RPC API

`eos serve` 把 EOS 作为本地工具服务运行。它启动 eos-core 内核 sidecar 子进程，
对外（IDE bridge、平台侧 agent、其它宿主）按 stdio 暴露与内核一致的 JSON-RPC 2.0 接口。

## 角色定位

`eos serve` 层是**透传代理**，不做任何业务裁决：

```
外部 JSON-RPC 客户端  ──stdio──▶  eos serve (Go)  ──pipe──▶  eos-core 内核 (Rust sidecar)
        ◀──response/notification──        ◀──response/notification──
```

- 所有请求由 serve 层原样转发给同一个内核 sidecar 进程，结果原样返回。
- 所有业务裁决（命令策略、审批、沙箱、turn 编排）都在 Rust 内核，serve 层不插手。
- 因此本接口的方法集、wire 格式、错误码与内核协议**永远自动一致**，无需在 Go 层维护映射。

## Transport

当前只支持 `stdio`（`--transport stdio`）：

- 帧协议：JSON-RPC 2.0 over `Content-Length` 分帧（LSP 风格），复用 `pkg/protocol/jsonrpc.Stream`。
- stdin 读请求，stdout 写响应/通知，stderr 由内核 tracing 日志占用（落盘到 `~/.eos/logs/core/eos-core.log`）。

## 方法集

透传 `jsonrpc.AllCoreMethods()` 的全部方法（约 135 个），分组概览：

| 分组 | 代表方法 |
|---|---|
| 生命周期 | `initialize`、`shutdown` |
| 工作区 | `workspace/list`、`workspace/add`、`workspace/use`、`workspace/set_foreground`、`workspace/trust`、`workspace/changes` |
| 会话 | `session/create`、`session/resume`、`session/list`、`session/current`、`session/set_current`、`session/delete`、`session/rename`、`session/set_meta`、`session/messages/load`、`session/messages/save` |
| Turn | `turn/start`、`turn/interrupt` |
| 工具 | `tool/catalog`、`tool/execute`、`tool/traces`、`tool/stats` |
| 权限/审批 | `permission/snapshot`、`permission/pending_review`、`permission/access_mode/set`、`permission/approval_mode/set`、`approval/respond`、`approval/list`、`approval/preview` |
| 询问 | `inquiry/respond` |
| 任务 | `task/list`、`task/todos`、`task/tail`、`task/kill`、`task/cleanup` |
| 事件 | `event/subscribe`、`event/unsubscribe` |
| 配置 | `config/get_rules`、`config/save_rules`、`config/settings/get`、`config/settings/save`、`config/reload` |
| 模型 | `model/list`、`model/activate`、`model/upsert`、`model/delete`、`model/context`、`model/catalog` |
| MCP 管理 | `mcp/list`、`mcp/upsert`、`mcp/import_json`、`mcp/delete`、`mcp/set_enabled` |
| Agent | `agent/spawn`、`agent/input`、`agent/wait`、`agent/run`、`agent/tool/execute`、`agent/list`、`agent/close`、`agent/control` |
| 编排 | `orchestrator/start`、`orchestrator/cancel` |
| 内存/上下文/版本/usage/state/modes/sandbox/lsp/insights/git/roles/diagnostics | 见 `pkg/protocol/jsonrpc/methods.go` 完整列表 |

完整 method 清单可由 `eos bridge manifest` 输出获取（见 IDE_BRIDGE.md）。

## initialize 握手

客户端连接后必须先发 `initialize`，响应结构：

```jsonc
{
  "server_name": "eos-core",
  "protocol_version": "<内核版本，如 2026.1>",
  "methods": ["initialize", "shutdown", "workspace/list", ...],
  "capabilities": { /* 内核宣告的能力，由内核填充 */ }
}
```

`methods` 即内核支持的全部 method。`protocol_version` 与 `capabilities` 由内核 `initialize` 返回，
serve 层透传，不篡改。

## 事件通知

客户端通过 `event/subscribe` 订阅事件后，内核产生的事件以 JSON-RPC notification 形式推送：

```jsonc
{
  "method": "event",
  "params": {
    "version": "v1",
    "event_id": "evt_xxx",
    "event_type": "item.delta",
    "session_id": "...",
    "turn_id": "...",
    "timestamp": "...",
    "source": "core",
    "payload": { /* 事件载荷 */ }
  }
}
```

事件 envelope 结构见 `pkg/protocol/protocol.go` 的 `Envelope`。

## 错误码

复用标准 JSON-RPC 2.0 错误码（`pkg/protocol/jsonrpc/message.go`）：

- `-32700` Parse error
- `-32600` Invalid request
- `-32601` Method not found
- `-32602` Invalid params
- `-32603` Internal error

未在 `AllCoreMethods()` 中的 method 返回 `-32601`。

## 最小调用示例

```jsonc
// 1. 握手
→ {"id":1,"method":"initialize"}
← {"id":1,"result":{"server_name":"eos-core","protocol_version":"...","methods":[...]}}

// 2. 创建会话
→ {"id":2,"method":"session/create","params":{"workspace_root":"/abs/path","title":"bridge"}}
← {"id":2,"result":{"id":"sess-xxx","workspace_root":"/abs/path",...}}

// 3. 启动 turn
→ {"id":3,"method":"turn/start","params":{"session_id":"sess-xxx","turn_id":"t1","input":"hello"}}
← {"id":3,"result":{"id":"t1","session_id":"sess-xxx","status":"running",...}}

// 4. 订阅事件（流式增量）
→ {"id":4,"method":"event/subscribe","params":{"session_id":"sess-xxx","turn_id":"t1"}}
← {"id":4,"result":{"subscription_id":"sub-1"}}
← {"method":"event","params":{"event_type":"item.delta","payload":{"delta":"Hel"},...}}
← {"method":"event","params":{"event_type":"request.completed",...}}
```

## 与 eos mcp serve 的区别

- `eos serve`：暴露 EOS 私有 JSON-RPC（方法名 `session/create` 等），适合深度集成、需要 turn 编排和事件流的宿主。
- `eos mcp serve`：暴露标准 MCP 协议（`tools/list`、`tools/call`），适合任意 MCP 客户端接入。见 `internal/docs/mcp/SERVER.md`。
