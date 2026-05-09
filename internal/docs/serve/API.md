# CLI 对外 API 文档（serve / stdio JSON-RPC）

本文档说明 `eos serve` 当前对外暴露的能力、调用方式、请求/响应格式与常见错误。

## 1. 总览

- 传输方式：仅支持 `stdio`
- 协议：JSON-RPC 2.0（按行传输，一行一个 JSON）
- IDE / Remote 宿主建议先生成桥接清单：`eos bridge manifest --workspace "/abs/workspace"`
- 服务启动命令：

```bash
eos serve --transport stdio --workspace "/abs/workspace" --allowed-tools "read,bash" --policy "./policy.json" --require-approval-digest=true
```

- 必填参数：`--workspace`
- 可选参数：
  - `--allowed-tools`：服务端允许的工具白名单（逗号分隔）
  - `--policy`：策略文件路径（JSON）
  - `--require-approval-digest`：中/高风险工具是否要求审批摘要（默认 `true`）

## 2. 协议基础

### 2.1 Request

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tool.list",
  "params": {}
}
```

### 2.2 Response（成功）

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {}
}
```

### 2.3 Response（失败）

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32005,
    "message": "InvalidParams",
    "data": {
      "field": "workspacePath"
    }
  }
}
```

### 2.4 事件通知（Notification）

服务端会主动输出事件，格式如下（无 `id`）：

```json
{
  "jsonrpc": "2.0",
  "method": "event",
  "params": {
    "type": "ToolCall",
    "ts": 1730000000,
    "sessionID": "s_xxx"
  }
}
```

### 2.5 桥接清单（推荐给 IDE / 外部宿主）

为了避免宿主硬编码启动参数、方法名和能力声明，建议先执行：

```bash
eos bridge manifest --workspace "/abs/workspace" --allowed-tools "read,bash"
```

返回 JSON 清单会包含：

- `launch.command` / `launch.args`：如何启动 `eos serve`
- `protocolVersion`：当前 JSON-RPC 协议版本
- `sessionDefaults`：推荐的初始会话参数
- `serverCapabilities`：握手能力声明
- `methods`：当前支持的方法列表
- 可选 `tools` / `capabilities`：默认工作区下的工具与能力目录

## 3. 调用顺序（推荐）

1. `initialize`
2. `session.create`
3. `tool.list`（可选）
4. `tool.preflight`（建议）
5. 如需要审批：`prompt.resolve`
6. `tool.execute`
7. 需要时：`tool.cancel` / `task.list` / `task.kill`
8. 结束：`session.close`

> 注意：除 `initialize` 外，其他方法在初始化前调用会返回 `Unauthorized`。

## 4. 方法清单

## `initialize`

- 作用：初始化握手，启用后续调用
- 请求参数：任意对象（服务端当前不强校验）
- 返回字段：
  - `server.name`
  - `server.version`
  - `protocolVersion`（当前 `1.0`）
  - `capabilities`
    - `events`
    - `invoke`
    - `tools`
    - `confirmations`
    - `sessions`
    - `requests`
    - `tasks`
    - `capabilityCatalog`

请求示例：

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"client":{"name":"platform","version":"1.0.0"},"protocolVersion":"1.0"}}
```

---

## `session.create`

- 作用：创建会话并绑定工作区和执行策略
- 请求参数：
  - `workspacePath` string（可省略；省略时使用启动参数 `--workspace`）
  - `options.executionMode` string：`plan` / `auto`
  - `options.trustedWorkspace` bool
  - `options.maxConcurrentToolCalls` int
  - `options.requireApprovalDigest` bool
  - `options.confirmPolicyID` string
  - `options.allowedTools` string[]
- 返回字段：`sessionID`

请求示例：

```json
{"jsonrpc":"2.0","id":2,"method":"session.create","params":{"workspacePath":"/abs/workspace","options":{"allowedTools":["read","bash"],"executionMode":"auto","requireApprovalDigest":true}}}
```

成功示例：

```json
{"jsonrpc":"2.0","id":2,"result":{"sessionID":"s_123456789abc"}}
```

---

## `session.close`

- 作用：关闭会话并清理资源（会中运行任务会被取消）
- 请求参数：`sessionID`
- 返回字段：`ok`（bool）

---

## `tool.list`

- 作用：获取当前服务可用工具定义
- 请求参数：`sessionID`
- 返回字段：`tools[]`
  - `name`
  - `description`
  - `riskLevel`
  - `params`
  - `examples`

---

## `tool.preflight`

- 作用：执行前预检，返回风险、预览和审批摘要
- 请求参数：
  - `sessionID`
  - `call.id`
  - `call.tool`
  - `call.parameters`
- 返回字段：
  - `riskLevel`：`low` / `medium` / `high`
  - `preview`：预检信息（不同工具字段不同）
  - `approvalDigest`：审批摘要（`sha256:...`）
  - `ttlSeconds`：审批有效期（高风险 30 秒，其他 60 秒）
  - `requestID`：仅在需要审批时返回

请求示例：

```json
{"jsonrpc":"2.0","id":3,"method":"tool.preflight","params":{"sessionID":"s_123456789abc","call":{"id":"c_1","tool":"bash","parameters":{"command":"echo hi"}}}}
```

---

## `prompt.resolve`

- 作用：提交审批决策，放行或拒绝后续执行
- 请求参数：
  - `sessionID`
  - `requestID`
  - `decision`：`deny` / `allow_once` / `allow_session`
  - `approvalDigest`（必须与 preflight 返回一致）
  - `reason`（可选）
  - `policyID`（可选）
  - `correlationID`（可选）
  - `approverTraceID`（可选）
- 返回字段：`ok`（bool）

请求示例：

```json
{"jsonrpc":"2.0","id":4,"method":"prompt.resolve","params":{"sessionID":"s_123456789abc","requestID":"r_123456789abc","decision":"allow_once","approvalDigest":"sha256:...","policyID":"platform-policy"}}
```

---

## `tool.execute`

- 作用：执行工具调用
- 请求参数：
  - `sessionID`
  - `call.id`
  - `call.tool`
  - `call.parameters`
- 返回：工具执行结果对象（结构取决于工具；常见包含 `status`、`display` 等）
- 事件：
  - 执行开始前：`ToolCall`
  - 执行完成后：`ToolResult`

请求示例：

```json
{"jsonrpc":"2.0","id":5,"method":"tool.execute","params":{"sessionID":"s_123456789abc","call":{"id":"c_1","tool":"bash","parameters":{"command":"echo hi"}}}}
```

---

## `tool.cancel`

- 作用：取消正在执行的调用
- 请求参数：`sessionID`、`callID`
- 返回字段：`ok`（是否存在并成功触发取消）

---

## `task.list`

- 作用：查询后台任务
- 请求参数：`sessionID`
- 返回字段：`tasks[]`
  - `id`
  - `status`
  - `startedAt`（Unix 秒）
  - `label`
  - `canKill`

---

## `task.kill`

- 作用：终止后台任务
- 请求参数：`sessionID`、`taskID`
- 返回字段：`ok`（bool）

## 5. 审批与执行策略

## 5.1 默认原则

- `executionMode=plan` 时，非只读工具会被拒绝执行
- `executionMode=auto` 时，高风险工具默认需要审批
- `requireApprovalDigest=true` 时，中/高风险工具需要审批

## 5.2 审批摘要

- `approvalDigest` 基于 canonical JSON + SHA-256 计算
- 审批时提交的 `approvalDigest` 必须与预检一致，否则返回 `ConfirmationDigestMismatch`

## 5.3 allow_session

- `allow_session` 会在当前 session 内对同 digest 放行一段时间（当前 10 分钟）

## 6. 路径与安全约束

- 请求参数中的以下路径字段会被归一化并限制在 workspace 内：
  - `path`
  - `file`
  - `source`
  - `destination`
  - `working_dir`
  - `root`
- 超出 workspace 会返回 `WorkspaceViolation`
- 配置 policy 后可额外限制：
  - `bash` 的 `allowedCommands`
  - 路径 `denyPathGlobs`

## 7. 错误码

- `-32001 Unauthorized`：未初始化就调用其他方法
- `-32002 SessionNotFound`：sessionID 不存在
- `-32003 ToolNotAllowed`：工具不在允许范围或策略拒绝
- `-32004 MethodNotFound`：方法不存在
- `-32005 InvalidParams`：参数不合法
- `-32006 ConfirmationRequired`：需要审批
- `-32007 ConfirmationExpired`：审批已过期
- `-32008 ConfirmationDigestMismatch`：审批摘要不匹配
- `-32009 WorkspaceViolation`：路径越界
- `-32010 Conflict / TooManyConcurrentCalls`：冲突或并发超限
- `-32012 Internal`：内部错误

## 8. 端到端最小示例

按行发送以下请求（每行一个 JSON）：

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"client":{"name":"platform","version":"1.0.0"},"protocolVersion":"1.0"}}
{"jsonrpc":"2.0","id":2,"method":"session.create","params":{"workspacePath":"/abs/workspace","options":{"allowedTools":["read","bash"],"requireApprovalDigest":true}}}
{"jsonrpc":"2.0","id":3,"method":"tool.preflight","params":{"sessionID":"s_xxx","call":{"id":"c_1","tool":"bash","parameters":{"command":"echo hi"}}}}
{"jsonrpc":"2.0","id":4,"method":"prompt.resolve","params":{"sessionID":"s_xxx","requestID":"r_xxx","decision":"allow_once","approvalDigest":"sha256:..."}} 
{"jsonrpc":"2.0","id":5,"method":"tool.execute","params":{"sessionID":"s_xxx","call":{"id":"c_1","tool":"bash","parameters":{"command":"echo hi"}}}}
{"jsonrpc":"2.0","id":6,"method":"session.close","params":{"sessionID":"s_xxx"}}
```

## 9. 兼容性说明

- 当前仅支持 `stdio`，不支持 HTTP 路由调用
- 工具列表与参数定义以 `tool.list` 的实时返回为准
- IDE / Remote 宿主建议优先消费 `eos bridge manifest`，而不是自行拼接 `serve` 启动参数
