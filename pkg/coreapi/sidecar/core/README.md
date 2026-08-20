# Vendored eos-core sidecar binaries

此目录存放各平台编译好的 eos-core sidecar 二进制 + manifest.json，随 eos-cli 仓库
一起分发（vendored），用户 `go install` / clone 即得全平台 sidecar，零运行时下载。

## 目录结构

```
pkg/coreapi/sidecar/core/<target-triple>/
  ├── eos-core[.exe]     # release 编译、strip 过的内核二进制
  └── manifest.json       # sha256 + ed25519 signature + features 清单
```

## 支持平台（5 target）

| target triple | 平台 | 填充方式 |
|---|---|---|
| `x86_64-pc-windows-gnu` | Windows x86_64 | dev-rebuild.ps1（本机）或 CI |
| `x86_64-apple-darwin` | macOS Intel | CI（macos-13 runner） |
| `aarch64-apple-darwin` | macOS Apple Silicon | CI（macos-latest runner） |
| `x86_64-unknown-linux-gnu` | Linux x86_64 | CI（ubuntu-latest runner） |
| `aarch64-unknown-linux-gnu` | Linux ARM64 | CI（ubuntu-24.04-arm runner） |

壳层 resolver（`pkg/coreapi/sidecar/manifest.go`）按 `runtime.GOOS`/`GOARCH` 选 target：
- Linux 主选 **gnu**（对齐 release.yml 与 vendored 产物），回退 musl（兼容历史安装布局）。
- Windows 主选 msvc，回退 gnu。

## 填充方式

- **跨平台（darwin / linux-gnu）**：GitHub Actions workflow `sync-vendored-sidecar.yml`
  在各平台原生 runner 上编译 + Ed25519 签名 + 校验 sha256，自动 commit 进此目录。
  手动触发：Actions → sync-vendored-sidecar → Run workflow；或 eos-core 发版后自动。
  本机**无法**交叉编译 macOS（缺 macOS SDK），勿用本机脚本编这些 target。

- **Windows（本机开发迭代）**：`scripts/dev-rebuild.ps1` 一键编译+签名+分发到
  eos-cli + eos-app 的 windows-gnu 目录。仅 host target，用于改内核后快速本地验证。

## 安全约束

- 二进制必须签名（ed25519），壳层加载时 `RequireSignature: true` + `VerifyChecksum: true`。
- manifest.json 的 `sha256` 必须与二进制实际 sha256 一致，否则拒绝加载。
- 勿放置未签名 / sha256 不匹配 / debug 符号未 strip 的二进制。
- 勿放置 Rust 源码、Cargo 元数据、私钥。
