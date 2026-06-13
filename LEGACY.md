# Legacy Go Fixtures — Rust-only Production Cutover

> 本文档列出 EOS Rust-only 切换后仍保留的 Go legacy 文件，并说明保留理由与未来去向。
>
> 维护守则：
> - production 路径（main.go / internal/cli / internal/ui）不得新增对以下文件的 import。
> - **Build tag 隔离**：以下标记了 `//go:build legacy` 的文件在默认构建中**完全不参与编译**，
>   只有 `go build -tags legacy` 时才会被编译。由 `TestLegacyPackagesHaveBuildTags` 守护。
> - `internal/architecture` 守护的 `TestCLIProductionPathsForbidLegacyGoCore`、
>   `TestDefaultBuildDoesNotReferenceGoAppServerOrToolHost`、`TestLegacyPackagesHaveBuildTags`
>   会拒绝任何新增违规。
> - 任何新增的 legacy usage 必须先在 `cliLegacyCoreExceptions` 加白并写明理由。

## 1. 生产路径现状

| 入口                     | 实现                              | Legacy 是否可达         |
| ------------------------ | --------------------------------- | ----------------------- |
| `eos` (TUI)              | `internal/ui` → `sidecarclient`   | ❌ 否（与 `pkg/core` 隔离） |
| `eos print`              | `internal/cli/print.go` → sidecar | ❌ 否（`AllowFallback=false`） |
| `eos exec`               | `internal/cli/exec.go` → sidecar  | ❌ 否                    |
| `eos app-server`         | `internal/cli/app_server.go`      | ⚠️ parity 模式 / env=1 时 (**`//go:build legacy`**) |
| `eos bridge manifest`    | `internal/cli/bridge.go`          | ⚠️ JSON-only metadata   |
| `eos serve`              | `internal/cli/serve.go`           | ⚠️ `internal/serve`（详见下） |
| `eos daemon`             | `internal/cli/daemon.go`          | ⚠️ HTTP gateway（详见下） |

`eos app-server`、`eos serve`、`eos daemon` 都是 hidden 内部命令；它们在 production 启动
路径里都不会自动调用 legacy 路径。

## 2. 保留的 Legacy 文件

### 2.1 `pkg/core`（Go sharedcore 运行时）— `//go:build legacy`

> **所有文件已添加 `//go:build legacy`**，默认构建不编译。

| 文件                  | 保留原因                                            | 未来去向                              |
| --------------------- | --------------------------------------------------- | ------------------------------------- |
| `core.go`             | parity harness 与 *_test.go 中使用 `core.NewRuntime` / `NewLegacyEngine` | 删除（与 eos-core 100% 对等时）       |
| `engine_adapter.go`   | parity harness 用 `core.NewLegacyEngine` 构造 `coreapi.Engine` 实现 | 同上                                  |
| `jsonrpc.go`          | parity harness 把 legacy engine 接到 JSON-RPC 上对比 | 同上                                  |
| `tasks_legacy.go`     | parity 仅在 Go side 验证 task backend | 同上                                  |
| `*_test.go`           | 单元测试 + 黄金测试 + 协议对比                      | 保留到 parity fixture 完全退役        |
| `core_jsonrpc_golden_test.go` | 同上                                          | 保留到 parity fixture 完全退役        |
| `gui_features.go`     | legacy engine adapter / parity test 引用 `Attachment` / `MemorySnapshot` 等类型 | 随 `pkg/core` 统一归档                |

### 2.2 `internal/bridge`（Go 旧 runtime core）— `//go:build legacy`

> **所有文件已添加 `//go:build legacy`**，默认构建不编译。

| 文件                          | 保留原因                                  | 未来去向                              |
| ----------------------------- | ----------------------------------------- | ------------------------------------- |
| `runtime_core.go`             | parity harness / eino agent 流程需要     | 归档                                  |
| `runtime_*.go`                | 同上                                      | 归档                                  |
| `guarded_cmd.go`              | parity 用 guarded git command             | 归档                                  |
| `runtime_lsp*.go`             | legacy LSP stub                           | 归档                                  |
| `*_test.go`                   | 单元测试                                  | 保留到 parity fixture 完全退役        |

### 2.3 `internal/runtime`（eino runtime + agent registry）

- 全部文件作为 parity harness / fixture 保留。
- 未来 eino 完全退役时统一删除。

### 2.4 `internal/tools`（Go 工具管理器）

- 全部文件作为 `internal/toolapi/impl/legacy_bridge.go` 与 eos-tool-host 的实现依赖。
- 未来由 coreapi.Tools 替换时统一归档。

### 2.5 `cmd/eos-tool-host`（独立 Go 二进制）— `//go:build legacy`

- 状态：仅作为 `EOS_TOOL_HOST_FAKE=1` 下的 dev/test fixture（FakeHost）。
- **所有文件已添加 `//go:build legacy`**，默认构建不编译。
- production 路径不引用该二进制（`TestDefaultBuildDoesNotReferenceGoAppServerOrToolHost` 守护）。
- 真实工具执行由 eos-core（Rust sidecar）通过 `tool/execute` JSON-RPC 完成。

### 2.6 `internal/serve`（eos serve HTTP gateway）

- `serve.go` / `coreapi_bridge.go` 当前承载 `eos serve` 与 `eos bridge manifest`。
- 这两个命令是 hidden，未来计划由 eos-core Rust HTTP gateway 取代。
- 内部依赖 `internal/tools`、`internal/serve/internal/pkg/...`，属于 legacy 边界。

### 2.7 `internal/daemon`（HTTP gateway daemon）

- `manager.go` 提供 `eos daemon start/status/stop` 子命令。
- 由 `internal/serve` 复用组件；隐藏 production 调用入口。
- 未来计划由 Rust 端 gateway 取代。

### 2.8 `internal/toolapi/impl/legacy_bridge.go` + `catalog.go`

- 唯一被允许直接 import `internal/tools` / `internal/runtime` 的非测试文件（见
  `TestToolAPIImplDependencyBoundary`）。
- 迁移完成后这两文件也归档。

### 2.9 `pkg/coreapi/sidecar/toolhost/`

- `legacy_host.go`：把 `tools.Manager` 暴露成 `toolhost.ToolHost`。dev/test 路径使用。
  **已添加 `//go:build legacy`**，默认构建不编译。
- `host.go` / `server.go` / `fake_host`：sidecar toolhost 协议层。
- `legacy_host_test.go` / `server_test.go`：单元测试。
- 全部在 eos-tool-host fixture 链路中复用；production 不依赖。

### 2.10 `pkg/coreapi/parity/` — `//go:build legacy`

- 整个包作为 parity harness：同时启动 legacy + sidecar，对比核心 operation 的等价性。
- `parity_test.go`、`harness.go`、`expected_gaps.go` 都是 fixture。
- **所有文件已添加 `//go:build legacy`**，默认构建不编译。
- 当 sidecar 100% 等价于 legacy 时整个包可以归档。

### 2.11 `internal/ui/adapter/runtime_legacy_test.go` + `runtime_jsonrpc_test.go` — `//go:build legacy`

- TUI adapter 的 legacy fixture 测试：保证 `RuntimeAdapter`（wrap InProcessClient over Go runtime）
  在测试场景下仍能跑通 JSON-RPC。
- **已添加 `//go:build legacy`**，默认构建不编译。
- production 路径只用 `CoreClientAdapter`（wrap eos-core --app-server --stdio）。
- architecture test `TestUIDirectRuntimeCouplingDoesNotSpread` 排除 `_test.go` 文件，
  因此这两个 legacy 测试文件不违反 import boundary。

## 3. Architecture Tests 守护列表

| 测试                                      | 守护内容                                                                 |
| ----------------------------------------- | ------------------------------------------------------------------------ |
| `TestUIDirectRuntimeCouplingDoesNotSpread`| `internal/ui/**` 不得 import `pkg/core` / `internal/runtime` / `internal/tools` / `internal/bridge` |
| `TestCLIHeadlessNoBridgeImport`           | `internal/cli/**` 不得 import `internal/bridge` / `internal/tools` / `internal/session`（保留为 test fixture 时可豁免） |
| `TestCLIProductionPathsForbidLegacyGoCore`| `internal/cli/**` 除 `app_server.go` 外不得 import `pkg/core` / `internal/bridge` / `internal/runtime` / `internal/tools` |
| `TestDefaultBuildDoesNotReferenceGoAppServerOrToolHost` | production 路径不得出现 "eos-tool-host" / "EOS_TOOL_HOST" 字符串，也不得 import `cmd/eos-tool-host` |
| `TestCLIDoesNotStartLegacyBridgeRuntimeInProduction` | production CLI 不得调用 `core.NewRuntime` / `bridge.NewRuntimeCore` 等构造 |
| `TestCLIDefaultCoreEngineFlagForbidsLegacy` | `eos app-server --core-engine` 默认值必须为空 / auto / rust，禁用 legacy / parity |
| `TestEngineProviderRejectsMissingRustBinary`        | `engineprovider.Select(AllowFallback=false)` 在缺 eos-core 时必须 error |
| `TestEngineProviderRejectsInitializeFailure`        | 子进程 initialize 失败时必须 error，不回退 legacy |
| `TestEngineProviderRejectsMissingRequiredMethods`   | 缺 required methods 时必须返回 `ErrMissingMethods` |
| `TestEngineProviderRejectsProtocolMismatch`         | 协议 method 缺失时必须 error |
| `TestEngineProviderModeAutoDefaultsToRustOnly`      | `ModeAuto` 在 production 解析为 rust-only |
| `TestEngineProviderResolveModeRejectsUnknown`       | 未知 mode（typo）必须 error                             |
| `TestPrintExecAllowFallback_DefaultsToFalse`        | `printExecAllowFallback` 默认 false，env=1 才允许     |
| `TestStartRustOnlyEngineFailsWithoutRustBinary`     | `startRustOnlyEngine` 缺 binary 直接 error            |
| `TestExecFailsWithoutRustBinary`                    | `eos exec` 缺 binary 直接 error                       |
| `TestProcessExecutionCallSitesAreClassified`        | `exec.Command` / `utils.Command` 等调用点白名单       |
| `TestLegacyPackagesHaveBuildTags`                   | 指定 legacy 目录和文件必须有 `//go:build legacy` 标签  |
| `TestLegacyBuildTagImportConsistency`               | 导入 legacy 包的文件必须有 `//go:build legacy` 标签    |
| `TestClosedRustCoreSourceIsNotVendored`             | 不得 vendoring Rust 源 / .rlib / .pdb                  |
| `TestNoSecretsOrPrivateKeysVendored`                | 不得 commit 私钥                                       |

## 4. Packaging / CLI 入口失败模式

`eos` 默认启动路径在以下情况必须 fail-fast，**绝不**回退 Go sharedcore：

1. **缺 Rust binary**：`EOS_CORE_PATH` 未设置、文件不存在、文件不可执行
   → `startRustOnlyEngine` / `engineprovider.Select` 返回 error，
   TUI/print/exec 直接退出 1。
2. **协议不匹配**：`eos-core --app-server --stdio` 启动后 initialize 报协议错误
   → `engineprovider.Select` 返回 error。
3. **required methods 缺失**：eos-core initialize 返回的 method 列表不包含
   `sidecarclient.RequiredMethods` 的子集
   → `engineprovider.Select` 返回 `ErrMissingMethods`，提示 `missing: <method1>, <method2>, ...`。
4. **dev 显式 fallback**：parity 模式 (`--core-engine=parity`) 或
   `EOS_CORE_ALLOW_FALLBACK=1` 才允许 selectLegacy 兜底，且仅限 dev/test。

`internal/cli/print_test.go` 与 `internal/cli/exec_test.go` 的
`TestStartRustOnlyEngineFailsWithoutRustBinary` / `TestExecFailsWithoutRustBinary` /
`TestPrintExecAllowFallback_DefaultsToFalse` 是 packaging 的核心断言。

## 5. 切换进度 Checklist

- [x] `engineprovider.Select(ModeAuto, AllowFallback=false)` 拒绝 legacy 回退
- [x] `engineprovider.Select(ModeLegacy/Parity)` 在 `AllowFallback=false` 时拒绝
- [x] `internal/ui` (含 adapter) 不再 import legacy 路径（architecture test 守护）
- [x] `internal/cli/{print,exec}.go` 走 sidecar，不再 import `pkg/core` / `internal/bridge`
- [x] `internal/cli/app_server.go` lazy-init sharedcore（仅在 parity / dev 开启时）
- [x] `cmd/eos-tool-host` 走 `EOS_TOOL_HOST_FAKE=1` + FakeHost，不再被 production 引用
- [x] `pkg/coreapi/parity` 保留作为 fixture
- [x] `pkg/core` 仍保留供 parity harness / 测试使用，不被 production 启动路径 import
- [x] **Build tag 隔离**：`pkg/core`、`cmd/eos-tool-host`、`pkg/coreapi/parity`、`internal/bridge`、`legacy_host.go`、`app_server.go` 全部添加 `//go:build legacy`
- [x] **Architecture tests**：`TestLegacyPackagesHaveBuildTags` + `TestLegacyBuildTagImportConsistency` 守护 tag 覆盖率（含 `internal/bridge`）
- [x] **默认构建验证**：`go build ./...` 不包含任何 legacy 包
- [x] **Release artifact gate**：vendored `eos-core.exe` manifest 覆盖 `generated.CoreMethods()` 全量方法，checksum 与 Ed25519 signature 通过 release gate
- [ ] `internal/runtime` / `internal/tools` 整体归档（等待 Rust 侧完全替代；当前仍被 `toolapi/impl/legacy_bridge.go` 引用）
- [ ] `internal/serve` / `internal/daemon` 由 Rust HTTP gateway 替代后归档
- [ ] `internal/toolapi/impl/catalog.go` 迁移遗留依赖到 `legacy_bridge.go` 模式

## 6. Release Gate 状态（2026-06-05 +08:00）

| Gate 命令 | 范围 | 结果 | 备注 |
| --- | --- | --- | --- |
| `EOS_RELEASE_ARTIFACT_CHECK=1 go test ./internal/architecture/... -count=1` | Release artifact / leakage gate | ✅ 全 ok | manifest 覆盖全量 generated core methods；checksum/signature gate 通过 |
| `cargo test --workspace` | `eos-core-rs` 全 workspace | ✅ 0 failed | 默认 workspace 测试通过 |
| `cargo clippy --workspace --all-targets -- -D warnings` | `eos-core-rs` 全 workspace | ✅ 0 warnings | `-D warnings` 严格模式 |
| `go test ./internal/architecture/... ./internal/cli/... ./internal/ui/... ./pkg/coreapi/... -count=1` | production-critical Go 目录 | ✅ 全 ok | Rust-only TUI/CLI/sidecar/protocol gate |
| `go test ./... -count=1` | `eos-cli` 全包 | ✅ 全 ok | 包含 legacy fixture 与 vendored sidecar smoke |
| `go run . -p "RC smoke hello" --output-format json` | packaged headless print smoke | ✅ 全 ok | 使用 vendored `eos-core.exe`，返回 fake model response |
| `go run . exec "RC exec smoke" --output json --timeout 30s` | packaged headless exec smoke | ✅ 全 ok | 使用 vendored `eos-core.exe`，返回 fake model response |

运行时间（首次冷跑）：

- cargo test：约 14s
- cargo clippy：0.57s（缓存命中）
- go test (4 dirs)：约 13s
- go test ./...：约 2 分钟（含 e2e stdio flow）

复现命令（与 release checklist 一致）：

```bash
# Rust
cd eos-core-rs
cargo test --workspace
cargo clippy --workspace --all-targets -- -D warnings

# Go (production-critical 4 dirs)
cd eos-cli
EOS_RELEASE_ARTIFACT_CHECK=1 go test ./internal/architecture/... -count=1
go test ./internal/architecture/... ./internal/cli/... ./internal/ui/... ./pkg/coreapi/... -count=1

# Go (full)
go test ./... -count=1

# Headless packaged smoke
go run . -p "RC smoke hello" --output-format json
go run . exec "RC exec smoke" --output json --timeout 30s
```

## 7. E2E 修复记录（race → sync dispatch）

`pkg/coreapi/jsonrpc/e2e_chain_test.go::TestE2EChainCoreClientAdapterDrivesSidecarEngine`
的 4 个子测试（`drive-letter` / `drive-letter-with-spaces` / `forward-slash` / `unc-path`）在
`go test ./...` 串行执行时偶发失败，错误信息：

```
e2e_chain_test.go:453: chain did not invoke "event/subscribe" via the JSON-RPC
sidecar; legacy Go fallback may be in play
```

**根因**：`CoreClientAdapter` 的事件 pump 在构造函数里以 `go a.pumpNotifications()`
异步启动；测试 `a.Events()` 返回前不保证 `engine.Events().Subscribe()`（dispatch
`event/subscribe` 的 JSON-RPC 调用）已完成。`assertChainMethods` 紧接着断言
`event/subscribe` 已落库，存在明显竞态。

**修复**（`internal/ui/adapter/core_client.go`）：

- 新增 `pumpOnce sync.Once` / `pumpReady chan struct{}` / `pumpClosed chan struct{}`
  三个同步原语。
- `ensurePumpStarted()` 第一次调用时启动 `runNotificationPump()` goroutine；
  `runNotificationPump()` 在 `engine.Events().Subscribe()` 完成（无论成功/失败）
  之后**关闭** `pumpReady`，调用方 `<-pumpReady` 后再返回。
- 构造函数不再启动 pump，**首次调用 `Events()` / `Invoke()`** 才会同步触发
  `event/subscribe`。
- 这样 `a.Events()` 返回时，`event/subscribe` 一定已经 dispatch 到 sidecar，
  测试断言可稳定通过。

**回归验证**：

- `go test -run TestE2EChainCoreClientAdapterDrivesSidecarEngine -race -count=20 -v ./pkg/coreapi/jsonrpc` → 20/20 PASS
- `go test ./... -count=1` → 5/5 连续 PASS
- `go build ./...` → EXIT 0
- `go vet ./...` → 1 个 pre-existing warning（`internal/ui/app_model_test_engine_test.go:41` 的 `Call passes lock by value`，与本次修复无关，遗留 fixture）

## 8. Go Legacy 6 类盘点（2026-06-04）

> 取数方法：
>
> - 「default build 可达」= `go list -deps ./cmd/protocol-gen` 中是否出现（代表 `eos` 默认编译路径可达）
> - 「legacy tag 覆盖」= `git grep '^//go:build legacy' -- '*.go'` 是否覆盖该目录所有非 `_test.go.tmp` 文件
> - 「能否删除」= 当前是否还有 production 路径或 architecture test 守护引用

| 类别 | 非临时 .go 文件数 | 默认 build 可达 | 全部 `//go:build legacy` | 当前可删除？ | 删除阻塞原因 | 建议去向 |
| --- | ---: | --- | --- | --- | --- | --- |
| `pkg/core` | 16 | ❌ 否（仅 `-tags legacy`） | ✅ 是 | ✅ **可以删**（build 维度） | `pkg/coreapi/parity` 仍以 `-tags legacy` 引用做 harness；删了 parity 也无法继续运行 | 待 parity harness 退役后随 `parity` 一起归档 |
| `cmd/eos-tool-host` | 5 | ❌ 否（仅 `-tags legacy`） | ✅ 是 | ✅ **可以删**（build 维度） | dev/test 仍以 `EOS_TOOL_HOST_FAKE=1` 启用 FakeHost；如果删，dev fixture 只能改走 in-process fake | 同上：随 parity 退役归档，或把 FakeHost 移入 sidecar/toolhost 下作为 `legacy` 包内 fixture |
| `internal/bridge` | 52 | ❌ 否（仅 `-tags legacy`） | ✅ 是 | ✅ **可以删**（build 维度） | `pkg/core`（legacy tag 下）还在引用；删 `bridge` 之前必须先删 `pkg/core` 内的 `core.NewRuntime` / `core.NewLegacyEngine` 引用 | 随 `pkg/core` 一起归档 |
| `internal/runtime` | 67 | ✅ 是（production 路径） | ❌ **否** | ❌ **不能删** | `internal/cli/{serve,bridge,app_server}.go`（hidden 命令）、`internal/toolapi/impl/{legacy_bridge,catalog}.go`、`internal/session/context_{add,compress}.go`、`internal/pkg/git/git.go` 等仍直接 import；`internal/runtime` 自身大量 `_test.go` | 等待 Rust 侧 eos-core 完全替代后，整体归档。归档前先把 `internal/toolapi/impl` 之外的所有依赖迁走 |
| `internal/tools` | 101 | ✅ 是（production 路径） | ❌ **否** | ❌ **不能删** | 同 `internal/runtime`：`internal/cli/{serve,bridge}.go`、`internal/toolapi/impl/{legacy_bridge,catalog,tasks}.go`、`internal/serve/{server,coreapi_bridge,bridge_manifest_test}.go` 等 | 同上：随 `internal/runtime` 整体归档 |
| `internal/toolapi/impl/legacy_bridge.go` | 1 | ✅ 是（production 路径） | ❌ **否** | ❌ **不能删**（受 `TestToolAPIImplDependencyBoundary` 守护，是 toolapi/impl 内唯一被允许 import `internal/tools` / `internal/runtime` 的非测试文件） | `internal/tools` / `internal/runtime` 还存在，`legacy_bridge.go` 是把它们收敛到一个文件的「单点」；删 `legacy_bridge.go` 等于把 `internal/tools` / `internal/runtime` 散落到 `executor.go` / `tasks.go` / `services.go`，违反 architecture test | 随 `internal/runtime` / `internal/tools` 一起归档 |

### 8.1 默认 build 实际依赖的 EOS 内部包（验证）

```text
$ go list -deps github.com/dreamSailing/eos/cmd/protocol-gen
# 仅 protocol-gen，不依赖任何 internal/*

$ go list -deps github.com/dreamSailing/eos | grep '^github.com/dreamSailing/eos/' | sort
github.com/dreamSailing/eos/internal/ai
github.com/dreamSailing/eos/internal/browser
github.com/dreamSailing/eos/internal/cli
github.com/dreamSailing/eos/internal/config
github.com/dreamSailing/eos/internal/context
github.com/dreamSailing/eos/internal/daemon
github.com/dreamSailing/eos/internal/gateway
github.com/dreamSailing/eos/internal/hooks
github.com/dreamSailing/eos/internal/i18n
github.com/dreamSailing/eos/internal/lsp
github.com/dreamSailing/eos/internal/mcp
github.com/dreamSailing/eos/internal/memory
github.com/dreamSailing/eos/internal/pkg/...
github.com/dreamSailing/eos/internal/runtime        ← 保留
github.com/dreamSailing/eos/internal/scheduler
github.com/dreamSailing/eos/internal/search
github.com/dreamSailing/eos/internal/serve          ← 保留（hidden HTTP gateway）
github.com/dreamSailing/eos/internal/session
github.com/dreamSailing/eos/internal/skills
github.com/dreamSailing/eos/internal/store
github.com/dreamSailing/eos/internal/toolapi
github.com/dreamSailing/eos/internal/toolapi/impl   ← 保留
github.com/dreamSailing/eos/internal/tools         ← 保留
github.com/dreamSailing/eos/internal/tools/{bg,fileops,git,shell}
github.com/dreamSailing/eos/internal/ui/...
github.com/dreamSailing/eos/pkg/agentcore/...
github.com/dreamSailing/eos/pkg/coreapi/...
github.com/dreamSailing/eos/pkg/protocol/...
github.com/dreamSailing/eos/pkg/sandbox
```

**未出现**（已被 build tag 隔离）：`internal/bridge`、`pkg/core`（除 `pkg/coreapi/*`）、
`cmd/eos-tool-host`、`pkg/coreapi/parity`、`pkg/coreapi/sidecar/toolhost/legacy_host.go`、
`internal/cli/app_server.go`。

### 8.2 Deleted / Retained / Blocked 清单

**可以删（build 维度无影响，仍属于 legacy 边界）**

- `pkg/core`（16 文件） — 等 parity harness 退役
- `cmd/eos-tool-host`（5 文件） — 等 parity harness 退役 / FakeHost 移位
- `internal/bridge`（52 文件） — 等 `pkg/core` 退役

**保留（default build 必需，等价「Retained」）**

- `internal/runtime`（67 文件） — 当前 production 路径 + tests 仍依赖
- `internal/tools`（101 文件） — 同上
- `internal/toolapi/impl/legacy_bridge.go`（1 文件） — `TestToolAPIImplDependencyBoundary` 守护的唯一被允许 import `internal/runtime` / `internal/tools` 的非测试文件

**附带清理（与 build 无关，仅 repo 卫生）**

- 383 个 `*.tmp` 零字节文件散落在 `internal/{bridge,runtime,tools}/`、`pkg/core/`、`internal/ui/adapter/` 等 — 编辑器 crash recovery 残留，**不会**被 `go build` 编译进 binary。可在 `go test` 之前 `git clean -f -- '*.tmp'` 批量删除。

### 8.3 阻塞 `internal/runtime` / `internal/tools` 归档的具体调用点

> 取数方法：`Select-String` 扫描所有 `*.go`（排除 `*.tmp` 与 `_tmp_wails_react_ts`）的 import 段，
> 排除 `internal/architecture/*`（守护）、`internal/bridge/*`、`pkg/core/*`、`cmd/eos-tool-host/*`、
> `internal/runtime/*`、`internal/tools/*` 自身以及 `runtime_legacy_test.go` / `runtime_jsonrpc_test.go` fixture。
> 同时排除显式 `internal/ui/adapter/boundary_test.go`（`architecture/import_boundary_test.go` 的补集）。

| 受影响文件 | 类别 | 说明 |
| --- | --- | --- |
| `internal/cli/serve.go` | production path（hidden cmd） | `eos serve` 入口 |
| `internal/cli/bridge.go` | production path（hidden cmd） | `eos bridge manifest` 入口 |
| `internal/cli/app_server.go` | production path（hidden cmd, `//go:build legacy`） | `eos app-server` 入口；`legacy` tag 下仍拉 `internal/runtime` |
| `internal/daemon/service.go` | production path（hidden cmd） | `eos daemon` 入口 |
| `internal/serve/server.go` | production path | `eos serve` 内部实现 |
| `internal/serve/coreapi_bridge.go` | production path | `eos serve` 与 eos-core 的桥接 |
| `internal/session/context_add.go` | production path | 会话上下文管理 |
| `internal/session/context_compress.go` | production path | 会话上下文压缩 |
| `internal/pkg/git/git.go` | production path | 内部 git 工具 |
| `internal/toolapi/impl/legacy_bridge.go` | production path | `TestToolAPIImplDependencyBoundary` 允许的唯一收敛点 |
| `internal/toolapi/impl/catalog.go` | production path | catalog 实现 |
| `internal/toolapi/impl/executor.go` | production path | tool 执行入口 |
| `internal/toolapi/impl/services.go` | production path | tool service 注册 |
| `internal/toolapi/impl/tasks.go` | production path | task 执行 |
| `internal/toolapi/impl/bridge.go` | production path | tool bridge |
| 上述目录内若干 `*_test.go` | test fixture | 单元测试与 parity fixture |
| `pkg/core/engine_adapter.go` | legacy tag 下 | parity engine adapter |
| `pkg/core/core.go` | legacy tag 下 | parity NewRuntime 入口 |

要把这 6 类（`internal/runtime` / `internal/tools` / `internal/serve` / `internal/daemon` /
`internal/toolapi/impl/{legacy_bridge,catalog,...}` / `internal/cli/{serve,bridge,app_server}`）
整体归档，必须先把以下 production 入口迁到 eos-core 端：

- `eos serve`（hidden） → eos-core Rust HTTP gateway
- `eos daemon`（hidden） → eos-core Rust gateway / sidecar lifecycle
- `eos app-server --core-engine=parity`（hidden, `//go:build legacy`）→ 删除命令（已经被 `engineprovider.Select(ModeAuto)` 完全拒掉 legacy 回退）
- `eos bridge manifest`（hidden）→ eos-core JSON-RPC `bridge/manifest` 方法
- `internal/toolapi/impl/{executor,tasks,services,bridge}.go` → 全部走 eos-core JSON-RPC
- `internal/session/context_{add,compress}.go` → eos-core store layer
- `internal/pkg/git/git.go` → eos-core `services/git`（已存在，见 §6 cargo test 输出）

### 8.4 Tag 覆盖率自检

`internal/architecture/legacy_tag_test.go::TestLegacyPackagesHaveBuildTags` 当前守护
4 个目录（`pkg/core`、`cmd/eos-tool-host`、`pkg/coreapi/parity`、`internal/bridge`）+ 6 个文件
（`legacy_host.go` / `legacy_host_test.go` / `app_server.go` / `app_server_test.go` /
`runtime_legacy_test.go` / `runtime_jsonrpc_test.go`）。本次盘点再次确认：

- 4 个目录所有非临时 `.go` 文件 100% 带 `//go:build legacy` ✅
- 6 个 legacy 单文件均带 `//go:build legacy` ✅
- `TestLegacyBuildTagImportConsistency` 通过（无 `legacy` 文件被 default build 引用）✅

## 9. 下一步动作

- [ ] 把 `internal/cli/{serve,bridge}.go` 与 `internal/serve/*` 的实现迁到 eos-core Rust HTTP gateway
- [ ] 把 `internal/toolapi/impl/{executor,tasks,services,bridge}.go` 迁到 eos-core JSON-RPC，然后归档 `internal/runtime` / `internal/tools`
- [ ] `eos app-server` 隐藏命令从 default build 中彻底删除（保留 `//go:build legacy` fixture 一段时间后可删）
- [ ] `git clean -f -- '*.tmp'` 清理 383 个 0 字节残留
- [ ] （可选）把 `cmd/eos-tool-host` FakeHost 收编到 `pkg/coreapi/sidecar/toolhost/` 的 `legacy_host.go` 内，删除 `cmd/eos-tool-host` 整个目录
