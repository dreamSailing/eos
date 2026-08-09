# eos bridge manifest — IDE 桥接清单

`eos bridge manifest` 生成一份描述 EOS 接入信息的 JSON 清单，供 IDE / 平台侧宿主
**自动发现**如何启动并连接 EOS，无需人工查阅文档。

## 用法

```bash
eos bridge manifest --workspace "/abs/workspace" --access-mode workspace-write --approval-mode on-request
```

`--workspace` 可省略（默认当前目录）。`--transport` 默认 `stdio`。

## 输出示例

```jsonc
{
  "command": "eos serve --transport stdio --workspace \"/abs/workspace\"",
  "transport": "stdio",
  "protocol_version": "2026.1",
  "server_name": "eos-core",
  "methods": ["initialize", "shutdown", "workspace/list", "session/create", "turn/start", "tool/execute", ...],
  "capabilities": { /* 内核 initialize 返回的 capabilities */ },
  "defaults": {
    "workspace": "/abs/workspace",
    "access_mode": "workspace-write",
    "approval_mode": "on-request"
  }
}
```

## 字段语义

| 字段 | 类型 | 含义 | 来源 |
|---|---|---|---|
| `command` | string | 宿主启动 EOS 的完整命令行 | 由 manifest 生成器按 transport + workspace 拼装 |
| `transport` | string | 传输方式，当前 `stdio` | 命令 flag |
| `protocol_version` | string | 内核协议版本 | 内核 `initialize` 返回的 `protocol_version` |
| `server_name` | string | 内核服务名 | 内核 `initialize` 返回的 `server_name` |
| `methods` | []string | 内核支持的全部 JSON-RPC method | `jsonrpc.AllCoreMethods()`（与内核 initialize 的 methods 一致） |
| `capabilities` | object | 内核宣告的能力 | 内核 `initialize` 返回的 `capabilities` |
| `defaults` | object | 本次 manifest 的默认会话参数 | 命令 flag（workspace/access_mode/approval_mode） |

## 生成原理

manifest 生成器会：

1. 启动一次 eos-core sidecar（`engineprovider.Select`，Rust-only）。
2. 调用内核 `initialize` 拿到 `protocol_version` / `server_name` / `capabilities`。
3. 合并 `jsonrpc.AllCoreMethods()` 作为 `methods`。
4. 拼装 `command` 字符串。
5. JSON 输出到 stdout。
6. 关闭 sidecar 进程。

`methods` 与 `protocol_version` / `capabilities` 都来自内核，保证清单与内核实际能力一致。

## 宿主侧消费方式

IDE / 平台侧宿主拿到 manifest 后：

- 用 `command` 启动 `eos serve` 子进程，按 `transport` 建立 JSON-RPC 连接。
- 用 `methods` 判断是否支持所需能力（capability 缺口检查）。
- 用 `protocol_version` 做协议版本兼容判断。
- 用 `defaults` 作为会话默认参数。

## 接口契约

详细 JSON-RPC 方法签名、wire 格式、错误码见 [API.md](./API.md)。
