<#
.SYNOPSIS
  Rebuild eos-core sidecar and sign-sync to both eos-cli and eos-app.

.DESCRIPTION
  After modifying eos-core-rs, run this script to one-click sync the kernel
  binary to both shells.
  Flow:
    1. Kill leftover eos-core.exe / eos.exe / eos-app.exe / go.exe processes.
    2. cargo build --workspace --release (in eos-core-rs).
    3. Sign-package to eos-cli pkg/coreapi/sidecar/core/<target>/.
    4. Sign-package to eos-app: output to temp dir, then distribute to
       eos-app/core/<target>/ (release package source) and
       eos-app/output/core/<target>/ (dev read path).
    5. Print both shells' sha256 for verification.
  Assumes all three repos are siblings under the same parent (e.g. C:\home\eos).

.PARAMETER CoreRepo
  eos-core-rs repo root. Defaults to ..\..\eos-core-rs relative to this script.

.PARAMETER CliRepo
  eos-cli repo root. Defaults to this script's repo root.

.PARAMETER AppRepo
  eos-app repo root. Defaults to eos-app sibling of eos-cli.

.PARAMETER SkipBuild
  Skip cargo build (already built, just re-sign and package).

.PARAMETER SkipKill
  Do not kill leftover processes.

.PARAMETER SkipApp
  Do not sync to eos-app (only eos-cli).

.EXAMPLE
  .\scripts\dev-rebuild.ps1
  .\scripts\dev-rebuild.ps1 -SkipBuild
  .\scripts\dev-rebuild.ps1 -SkipApp
#>
[CmdletBinding()]
param(
    [string]$CoreRepo = "",
    [string]$CliRepo = "",
    [string]$AppRepo = "",
    [switch]$SkipBuild,
    [switch]$SkipKill,
    [switch]$SkipApp
)

$ErrorActionPreference = "Stop"

# Resolve repo roots
$scriptDir = $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($CliRepo)) {
    $CliRepo = Split-Path -Parent $scriptDir
}
if ([string]::IsNullOrWhiteSpace($CoreRepo)) {
    $CoreRepo = Join-Path (Split-Path -Parent $CliRepo) "eos-core-rs"
}
if ([string]::IsNullOrWhiteSpace($AppRepo)) {
    $AppRepo = Join-Path (Split-Path -Parent $CliRepo) "eos-app"
}
$CoreRepo = (Resolve-Path $CoreRepo -ErrorAction Stop).Path
$CliRepo  = (Resolve-Path $CliRepo  -ErrorAction Stop).Path
$AppRepo  = (Resolve-Path $AppRepo  -ErrorAction Stop).Path

$privateKey = Join-Path $CoreRepo "scripts\eos-signing-key.pem"
if (-not (Test-Path $privateKey)) {
    throw "Ed25519 private key not found: $privateKey"
}

Write-Host "==> eos-core-rs : $CoreRepo"
Write-Host "==> eos-cli     : $CliRepo"
if (-not $SkipApp) { Write-Host "==> eos-app     : $AppRepo" }
Write-Host "==> signing key : $privateKey"

# 1. Kill leftover sidecar processes to avoid file locks.
if (-not $SkipKill) {
    $leaches = Get-Process -Name "eos-core","eos","eos-app","go" -ErrorAction SilentlyContinue
    if ($leaches) {
        Write-Host "==> killing leftover processes: $($leaches.ProcessName -join ', ')"
        $leaches | Stop-Process -Force -ErrorAction SilentlyContinue
        Start-Sleep -Milliseconds 800
    }
}

# 2. Release build the sidecar
if (-not $SkipBuild) {
    Write-Host "==> cargo build --workspace --release"
    Push-Location $CoreRepo
    try {
        & cargo build --workspace --release
        if ($LASTEXITCODE -ne 0) { throw "cargo build failed (exit $LASTEXITCODE)" }
    }
    finally { Pop-Location }
}

# Determine target triple (same logic as package-public-artifact.ps1)
$rustcVersion = & rustc -vV
$hostLine = $rustcVersion | Where-Object { $_ -like "host:*" } | Select-Object -First 1
$target = $hostLine.Substring("host:".Length).Trim()
$exeName = if ($target -like "*windows*") { "eos-core.exe" } else { "eos-core" }

$pkgScript = Join-Path $CoreRepo "scripts\package-public-artifact.ps1"
if (-not (Test-Path $pkgScript)) {
    throw "Package script not found: $pkgScript"
}

# 3. Sign-package to eos-cli vendored core dir
Write-Host "==> sign-package sidecar -> eos-cli"
& $pkgScript -Configuration release -PublicRepo $CliRepo -PrivateKeyPath $privateKey
if ($LASTEXITCODE -ne 0) { throw "Package/sign failed for eos-cli (exit $LASTEXITCODE)" }

# 4. Sync to eos-app (core/ package source + output/core/ dev read path)
if (-not $SkipApp) {
    Write-Host "==> sign-package sidecar -> eos-app"
    # Package script outputs to eos-app/pkg/coreapi/sidecar/core/<target>/, then we distribute.
    & $pkgScript -Configuration release -PublicRepo $AppRepo -PrivateKeyPath $privateKey
    if ($LASTEXITCODE -ne 0) { throw "Package/sign failed for eos-app (exit $LASTEXITCODE)" }

    $pkgCore = Join-Path $AppRepo "pkg\coreapi\sidecar\core\$target"
    $appCoreDir = Join-Path $AppRepo "core\$target"
    $outputCoreDir = Join-Path $AppRepo "output\core\$target"

    # Distribute to core/ (release package source)
    New-Item -ItemType Directory -Force -Path $appCoreDir | Out-Null
    Copy-Item -Force (Join-Path $pkgCore $exeName) (Join-Path $appCoreDir $exeName)
    Copy-Item -Force (Join-Path $pkgCore "manifest.json") (Join-Path $appCoreDir "manifest.json")
    Write-Host "  -> $appCoreDir"

    # Distribute to output/core/ (dev read path: wails3 dev compiles exe to output/)
    New-Item -ItemType Directory -Force -Path $outputCoreDir | Out-Null
    Copy-Item -Force (Join-Path $pkgCore $exeName) (Join-Path $outputCoreDir $exeName)
    Copy-Item -Force (Join-Path $pkgCore "manifest.json") (Join-Path $outputCoreDir "manifest.json")
    Write-Host "  -> $outputCoreDir"

    # Remove temp output (eos-app does not read pkg/.../core/; that is the package script default path)
    Remove-Item -Recurse -Force (Join-Path $AppRepo "pkg") -ErrorAction SilentlyContinue
}

# 5. Print sha256 for manual verification
Write-Host ""
Write-Host "==> SHA256 verification:" -ForegroundColor Cyan

$cliBin = Join-Path $CliRepo "pkg\coreapi\sidecar\core\$target\$exeName"
if (Test-Path $cliBin) {
    $cliHash = (Get-FileHash -Algorithm SHA256 $cliBin).Hash
    Write-Host "  eos-cli : $cliHash"
}
if (-not $SkipApp) {
    $appBin = Join-Path $AppRepo "core\$target\$exeName"
    if (Test-Path $appBin) {
        $appHash = (Get-FileHash -Algorithm SHA256 $appBin).Hash
        Write-Host "  eos-app : $appHash"
    }
}

Write-Host ""
Write-Host "Done. You can now 'go run .' (eos-cli) or 'wails3 dev' (eos-app)." -ForegroundColor Green
