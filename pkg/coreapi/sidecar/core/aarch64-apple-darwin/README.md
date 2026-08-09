# `aarch64-apple-darwin` vendored sidecar（macOS Apple Silicon）

此目录存放 macOS Apple Silicon（aarch64-apple-darwin，M1/M2/M3）的 eos-core sidecar 二进制 + manifest.json。

## 填充方式

由 GitHub Actions workflow `sync-vendored-sidecar.yml` 在 **macos-latest 原生 runner**
（Apple Silicon）上编译 + Ed25519 签名 + 校验 sha256 后自动 vendored 进此目录。

- 触发：手动（Actions → sync-vendored-sidecar → Run workflow），或 eos-core 发版后自动。
- 本机**无法**交叉编译 macOS（缺 macOS SDK，法律+技术双重不可行），勿尝试用 dev-rebuild.ps1 编此 target。

## 内容约定

- `eos-core`（无 .exe 后缀，macOS 可执行文件）
- `manifest.json`（含 sha256 + ed25519 signature + features）

勿手动编辑或放置未签名二进制。壳层加载时会校验 sha256 + 签名（`VerifyChecksum: true` + `RequireSignature: true`）。
