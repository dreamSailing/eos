# EOS

Go 语言实现的终端 AI 编码助手，基于 CloudWeGo Eino 做多代理编排，提供可交互 TUI、工具调用、安全门禁与工作区上下文能力。

- 项目仓库：https://github.com/dreamSailing/eos
- 问题反馈：https://github.com/dreamSailing/eos/issues
- 版本发布：https://github.com/dreamSailing/eos/releases

## 为什么不用 Claude Code？

| 痛点 | EOS |
|---|---|
| 国内需要梯子 | 开箱即用 |
| 只支持 Claude | 主流模型全支持 |
| 安装依赖 Node | Go 开发，零依赖 |
| 配置复杂 | 填 Key 就能用 |
| Claude Code MCP + 视觉不支持 | 已修复此痛点 |
| Claude Code 无网络搜索 | 已集成 |

## 核心能力

- 交互式 TUI：AI/Bash 双模式、面板系统、流式输出与 Markdown 渲染
- 三种执行模式：`manual` / `plan` / `auto`（可在界面中循环切换）
- 多代理协同：planner、developer、tester、reviewer 等子代理分工
- 工具体系：文件读写/编辑、搜索、Git、Shell、后台任务、MCP 调用等
- 安全控制：高风险工具调用分级与确认，支持会话级授权
- 上下文索引：代码索引、文件监听、上下文压缩与会话持久化
- 可选 LSP 能力：支持 `without_lsp`、默认 LSP、`with_gopls` 嵌入版本

## 环境要求

- Go 1.25+
- 可访问的 OpenAI 兼容接口（`EOS_API_BASE`、`EOS_API_KEY`、`EOS_MODEL`）

## 快速开始

### 1) 编译

```bash
git clone https://github.com/dreamSailing/eos.git
cd eos
go mod tidy
go build -o eos
```

Windows:

```powershell
.\eos.exe
```

macOS / Linux:

```bash
./eos
```

### 2) 配置模型

方式 A：环境变量

```bash
export EOS_API_BASE="https://api.openai.com/v1"
export EOS_API_KEY="sk-..."
export EOS_MODEL="gpt-4o-mini"
```

方式 B：`~/.eos.json`

```json
{
  "models": [
    {
      "name": "default",
      "api_base": "https://api.openai.com/v1",
      "api_key": "sk-...",
      "model": "gpt-4o-mini"
    }
  ],
  "active_model": "default"
}
```

## 构建变体（LSP）

- 最小版（无 LSP）  
  `go build -tags without_lsp -o eos`
- 默认版（启用 LSP 框架）  
  `go build -o eos`
- Go 增强版（嵌入 gopls）  
  `go build -tags with_gopls -o eos`

项目内提供了 gopls 嵌入脚本：`scripts/embed_gopls.sh`、`scripts/embed_gopls.bat`。

## 常用交互

### 快捷键

- `F2`：切换 AI / Bash 模式
- `Alt+M`：切换执行模式（manual → plan → auto）
- `Alt+V`：粘贴剪贴板图片
- `Alt+H`：展开/折叠思考内容
- `?`：打开帮助面板
- `Ctrl+O`：切换实时信息面板样式

### 常用斜杠命令

- `/help` `/clear` `/exit`
- `/history`（或 `/versions`）
- `/models` `/mcp` `/ctx` `/cost` `/tasks`
- `/workspace list|add|remove|use <path>`
- `/settings` `/lsp` `/rules` `/lang` `/compact`
- `/init`：在当前工作区初始化 `EOS.md`

## 服务模式 API

- CLI 对外 API（`eos serve`）：[internal/docs/serve/API.md](./internal/docs/serve/API.md)
- IDE bridge 最小接入：先生成桥接清单 `eos bridge manifest --workspace "/abs/workspace"`，详见 [internal/docs/serve/IDE_BRIDGE.md](./internal/docs/serve/IDE_BRIDGE.md)

## 项目结构（简版）

```text
internal/
  cli/       Cobra 入口
  ui/        TUI 交互与面板
  bridge/    UI 与 runtime 桥接
  runtime/   Eino 编排与工具调度
  tools/     工具定义与执行
  context/   代码索引与监听
  session/   会话上下文管理
  lsp/       LSP 管理与嵌入支持
```

## 开发与测试

```bash
go test ./...
go build ./...
```

## 开源发布注意事项

- 运行时会在工作目录生成 `.eos/` 数据（会话、检查点、版本快照等）
- 请确保 `.eos/`、`.eos.json`、`.env`、日志和本地配置不进入版本控制
- 提交前建议做一次敏感信息检查（API Key、私钥、证书、绝对路径等）

## 许可证

本项目采用 EOS 非商用许可证 v1.1，详见 [LICENSE](./LICENSE)：

- 个人/非商业用途可免费使用（包含安装包使用）
- 允许自行编译、修改和分发非商业版本
- 衍生作品必须以相同许可证开源发布
- 禁止任何商业使用（含企业内部生产用途、收费服务、SaaS、二次商业分发）
- 商业使用必须获得版权人单独书面授权

## 联系方式

- 问题反馈：https://github.com/dreamSailing/eos/issues
- 商业合作/授权咨询：smart-os@qq.com
