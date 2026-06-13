# Regenerate protocol types from Rust schema
# Prerequisites: Rust schema.json must exist
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir
$SchemaPath = Join-Path $RepoRoot "..\eos-core-rs\crates\eos-core-protocol\schema.json"

Write-Host "Generating Go protocol types from $SchemaPath..."
go run "$RepoRoot\cmd\protocol-gen" -schema "$SchemaPath"
Write-Host "Done. Run 'go test ./pkg/coreapi/generated/...' to verify."
