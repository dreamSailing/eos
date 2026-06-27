<#
.SYNOPSIS
  Dev 重建 eos-core sidecar 并签名打包到 eos-cli 的 vendored binaries 目录。

.DESCRIPTION
  改完 eos-core-rs 后跑这个脚本，go run . 就能用上新 sidecar。
  流程：
    1. 杀掉残留的 eos-core.exe / eos.exe / go.exe 进程（避免占用二进制文件）。
    2. cargo build --workspace --release（在 eos-core-rs）。
    3. 用仓库内 Ed25519 私钥签名打包到 pkg\coreapi\sidecar\binaries\<target>\。
  默认假设 eos-core-rs 与 eos-cli 是同级目录（C:\home\eos）。

.PARAMETER CoreRepo
  eos-core-rs 仓库根目录。默认 ..\eos-core-rs（相对本脚本）。

.PARAMETER CliRepo
  eos-cli 仓库根目录。默认本脚本所在仓库根。

.PARAMETER SkipBuild
  跳过 cargo build（已构建好，只想重新签名打包时用）。

.PARAMETER SkipKill
  不杀残留进程（自己已确认没残留时用）。

.EXAMPLE
  .\scripts\dev-rebuild.ps1
  .\scripts\dev-rebuild.ps1 -SkipBuild
#>
[CmdletBinding()]
param(
    [string]$CoreRepo = "",
    [string]$CliRepo = "",
    [switch]$SkipBuild,
    [switch]$SkipKill
)

$ErrorActionPreference = "Stop"

# 解析仓库根目录
$scriptDir = $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($CliRepo)) {
    $CliRepo = Split-Path -Parent $scriptDir
}
if ([string]::IsNullOrWhiteSpace($CoreRepo)) {
    $CoreRepo = Join-Path (Split-Path -Parent $CliRepo) "eos-core-rs"
}
$CoreRepo = (Resolve-Path $CoreRepo -ErrorAction Stop).Path
$CliRepo  = (Resolve-Path $CliRepo  -ErrorAction Stop).Path

$privateKey = Join-Path $CoreRepo "scripts\eos-signing-key.pem"
if (-not (Test-Path $privateKey)) {
    throw "Ed25519 私钥不存在: $privateKey"
}

Write-Host "==> eos-core-rs : $CoreRepo"
Write-Host "==> eos-cli     : $CliRepo"
Write-Host "==> 签名密钥     : $privateKey"

# 1. 杀掉残留 sidecar 进程，避免占用 binaries 目录里的 eos-core.exe。
if (-not $SkipKill) {
    $leaches = Get-Process -Name "eos-core","eos","go" -ErrorAction SilentlyContinue
    if ($leaches) {
        Write-Host "==> 终止残留进程: $($leaches.ProcessName -join ', ')"
        $leaches | Stop-Process -Force -ErrorAction SilentlyContinue
        Start-Sleep -Milliseconds 800
    }
}

# 2. release 构建 sidecar
if (-not $SkipBuild) {
    Write-Host "==> cargo build --workspace --release"
    Push-Location $CoreRepo
    try {
        & cargo build --workspace --release
        if ($LASTEXITCODE -ne 0) { throw "cargo build 失败 (exit $LASTEXITCODE)" }
    }
    finally { Pop-Location }
}

# 3. 签名打包到 eos-cli vendored binaries
$pkgScript = Join-Path $CoreRepo "scripts\package-public-artifact.ps1"
if (-not (Test-Path $pkgScript)) {
    throw "打包脚本不存在: $pkgScript"
}
Write-Host "==> 签名打包 sidecar -> $CliRepo"
& $pkgScript -Configuration release -PublicRepo $CliRepo -PrivateKeyPath $privateKey
if ($LASTEXITCODE -ne 0) { throw "打包签名失败 (exit $LASTEXITCODE)" }

Write-Host ""
Write-Host "完成。现在可以 go run . 了。" -ForegroundColor Green
