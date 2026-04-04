# IDE Bridge 最小接入

本文档说明如何用 `vb-coding bridge manifest` 把 `vb-coding serve` 接到 IDE、自动化宿主或其他本地平台进程。

## 目标

桥接清单解决两个问题：

- 宿主不需要硬编码 `serve` 启动参数
- 宿主可以在连接前获知协议版本、能力声明与默认会话参数

## 1. 生成桥接清单

```bash
vb-coding bridge manifest --workspace "/abs/workspace" --allowed-tools "read,bash"
```

常用可选参数：

- `--policy <path>`：附带策略文件
- `--require-approval-digest=false`：关闭中高风险摘要校验
- `--command <path>`：覆盖宿主实际要启动的可执行路径
- `--out <path>`：写入文件而不是输出到 stdout
- `--include-tools=false`
- `--include-capabilities=false`

如果你是在开发期通过 `go run . bridge manifest ...` 生成清单，建议同时传 `--command /abs/path/to/vb-coding`，否则清单里会记录临时构建产物路径。

## 2. 清单结构

桥接清单是 JSON，核心字段包括：

- `launch.command` / `launch.args`
- `protocolVersion`
- `sessionDefaults.workspacePath`
- `sessionDefaults.allowedTools`
- `sessionDefaults.requireApprovalDigest`
- `serverCapabilities`
- `methods`

如果保留默认值，还会包含：

- `tools`：默认工作区和默认权限下可直接执行的工具目录
- `capabilities`：默认工作区下可观察的完整能力目录

## 3. 宿主接入顺序

宿主推荐按以下顺序工作：

1. 读取桥接清单
2. 按 `launch.command` + `launch.args` 启动 `vb-coding serve`
3. 通过 stdio 按行发送 JSON-RPC 2.0 请求
4. 调用 `initialize`
5. 调用 `session.create`
6. 根据需要调用 `capability.list` / `tool.list` / `tool.preflight` / `tool.execute`

## 4. 最小宿主要求

- 能启动本地进程
- 能同时读写该进程的 stdin/stdout
- 能处理按行分隔的 JSON-RPC 消息
- 能在审批场景下调用 `prompt.resolve` 或 `approval.resolve`

## 5. 当前边界

- 当前只支持 `stdio`
- 清单描述的是“本地 bridge”，不是 HTTP 网关
- VS Code / JetBrains 的专用插件壳还未内置，但宿主接入所需的启动和协议元数据已经稳定输出
