# `aarch64-unknown-linux-musl` vendored sidecar（Linux ARM64 静态链接）

此目录存放 Linux ARM64（aarch64-unknown-linux-musl，静态链接，树莓派/ARM 服务器）的 eos-core sidecar 二进制 + manifest.json。

## 填充方式

由 GitHub Actions workflow `sync-vendored-sidecar.yml` 编译 + Ed25519 签名 + 校验 sha256 后自动 vendored 进此目录。

- 触发：手动（Actions → sync-vendored-sidecar → Run workflow），或 eos-core 发版后自动。
- 编译方式：ubuntu ARM runner（`ubuntu-24.04-arm`）原生编译，或 ubuntu-latest + `cross`（Docker）交叉编译。
- 本机交叉编译 ARM Linux 难度高（ring 的 aarch64 汇编 + musl 工具链），走 CI。

## 为什么是 musl 不是 gnu

对齐 codex（`rust-release.yml`）与 eos-core-rs 现有 CI（`build-multiplatform.yml`）的选择：
musl 静态链接，部署免 glibc 版本兼容问题，且避开 openssl-sys 的交叉编译痛点。
壳层 resolver（`manifest.go`）Linux 主选 musl，回退 gnu（兼容历史 gnu 二进制）。

## 内容约定

- `eos-core`（无 .exe 后缀，Linux 可执行文件）
- `manifest.json`（含 sha256 + ed25519 signature + features）

勿手动编辑或放置未签名二进制。壳层加载时会校验 sha256 + 签名（`VerifyChecksum: true` + `RequireSignature: true`）。
