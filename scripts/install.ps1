# EOS CLI 官方安装脚本（Windows PowerShell）
#
# 用法：
#   irm https://raw.githubusercontent.com/dreamSailing/eos/main/scripts/install.ps1 | iex
#   .\install.ps1 -Version v1.0.0-beta.3
#
# 行为：GitHub Releases 拉取 windows 归档 → SHA256 校验 → 安装到
#   %LOCALAPPDATA%\Programs\eos（eos.exe + core\）→ 用户 PATH 追加该目录。
# `eos update` 之后原地自升级。

param(
    [string]$Version = "",
    [string]$InstallDir = ""
)

$ErrorActionPreference = "Stop"
$Repo = "dreamSailing/eos"

if (-not $InstallDir) {
    $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\eos"
}

$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "ARM64" { "arm64" }
    default { "amd64" }
}

try {
    if (-not $Version) {
        Write-Host "正在获取最新版本..."
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" `
            -Headers @{ "User-Agent" = "EOS-Install" }
        $Version = $release.tag_name
        if (-not $Version) { throw "无法获取最新版本号" }
    }
    Write-Host "目标版本: $Version (windows/$Arch)"

    $verNum = $Version.TrimStart("v")
    $asset = "eos-cli_v${verNum}_windows-${Arch}.zip"
    $baseUrl = "https://github.com/$Repo/releases/download/$Version"

    $tmp = New-Item -ItemType Directory -Force -Path (Join-Path $env:TEMP ([System.Guid]::NewGuid().ToString()))
    $assetPath = Join-Path $tmp $asset

    Write-Host "下载 $asset ..."
    Invoke-WebRequest -Uri "$baseUrl/$asset" -OutFile $assetPath -UseBasicParsing
    if (-not (Test-Path $assetPath)) { throw "下载失败（$Version 可能没有 windows-$Arch 归档）" }

    Write-Host "校验 SHA256 ..."
    $sumsPath = Join-Path $tmp "SHA256SUMS.txt"
    Invoke-WebRequest -Uri "$baseUrl/SHA256SUMS.txt" -OutFile $sumsPath -UseBasicParsing
    $want = $null
    foreach ($line in (Get-Content $sumsPath)) {
        $fields = $line -split '\s+'
        if ($fields.Count -ge 2 -and $fields[1].Trim() -eq $asset) { $want = $fields[0].Trim().ToLower(); break }
    }
    if (-not $want) { throw "SHA256SUMS.txt 中没有 $asset 条目" }
    $got = (Get-FileHash -Path $assetPath -Algorithm SHA256).Hash.ToLower()
    if ($got -ne $want) { throw "校验不匹配：期望 $want，实际 $got" }

    Write-Host "安装到 $InstallDir ..."
    $extractDir = Join-Path $tmp "extract"
    Expand-Archive -Path $assetPath -DestinationPath $extractDir -Force
    $eosBin = Get-ChildItem -Path $extractDir -Recurse -Filter "eos.exe" | Select-Object -First 1
    if (-not $eosBin) { throw "归档中未找到 eos.exe" }
    $stageRoot = Split-Path -Parent $eosBin.FullName

    # 旧版本移走后再落新文件（运行中的 eos-core.exe 无法即时删除时残留目录下次清理）
    if (Test-Path $InstallDir) {
        $stale = "$InstallDir.old-$(Get-Date -Format 'yyyyMMddHHmmss')"
        try { Move-Item -Path $InstallDir -Destination $stale -ErrorAction Stop } catch {
            throw "无法移开旧版本（eos 可能正在运行），请关闭后重试: $_"
        }
    }
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item -Path (Join-Path $stageRoot "*") -Destination $InstallDir -Recurse -Force
    Get-ChildItem -Path (Split-Path -Parent $InstallDir) -Filter "eos.old-*" -Directory -ErrorAction SilentlyContinue |
        Remove-Item -Recurse -Force -ErrorAction SilentlyContinue

    # 用户 PATH 追加安装目录
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
        Write-Host "已将 $InstallDir 加入用户 PATH（新开的终端生效）"
    }

    & (Join-Path $InstallDir "eos.exe") version
    Write-Host ""
    Write-Host "安装完成。运行 eos 开始使用；eos update 可自升级到最新版。"
}
finally {
    if ($tmp -and (Test-Path $tmp)) { Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue }
}
