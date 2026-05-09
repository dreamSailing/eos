# EOS MCP Server

本文档说明如何把 `eos` 作为标准 MCP Server 暴露给其他 agent / 宿主调用。

## 启动方式

### stdio

```bash
eos mcp serve --transport stdio --workspace "/abs/workspace"
```

### SSE

```bash
eos mcp serve --transport sse --listen 127.0.0.1:8765 --workspace "/abs/workspace"
```

可选参数：

- `--allowed-tools "read,time_now,bash"`
- `--sandbox-mode workspace|full_access`
- `--policy /abs/policy.json`
- `--require-approval-digest=true`
- `--base-url http://127.0.0.1:8765`

## 暴露能力

### Tools

EOS 会把当前可执行工具目录映射为标准 MCP `tools/list` / `tools/call`，同时额外暴露一组控制工具：

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

### Resources

第一阶段提供以下资源：

- `eos://sessions`
- `eos://catalog/tools`
- `eos://catalog/capabilities`
- `eos://runtime/mcp-status`
- `eos://runtime/lsp-status`
- `eos://runtime/version`

资源模板：

- `eos://sessions/{id}`
- `eos://sessions/{id}/approvals`
- `eos://sessions/{id}/inquiries`
- `eos://sessions/{id}/tasks`

## Session 规则

- 每个 MCP 连接会自动创建一个默认 EOS session
- 工具调用若不传 `session_id`，默认落到当前连接的默认 session
- 如需显式多会话控制，先调用 `eos_session_create`，后续再传 `session_id`

## 审批与提问

- 高风险工具不会自动放行
- 服务端会返回 `isError=true` 的结构化结果，并写入对应 session 的 approvals 资源
- 外部 agent 需要调用 `eos_approval_resolve` 完成确认，再重试原工具调用

- `ask_user_question` 不会直接阻塞等待用户
- 服务端会返回 pending inquiry
- 外部 agent 调用 `eos_inquiry_resolve` 提交答案后，再重试 `ask_user_question` 即可拿到结果
