# Rust Core Migration RC PR Plan

This branch is in release-candidate cleanup for the Go-to-Rust core migration.
Use this file as the review and staging guide before merging.

## Current Verified Gates

- `EOS_RELEASE_ARTIFACT_CHECK=1 go test ./internal/architecture/... -count=1`
- `go test ./internal/architecture/... ./internal/cli/... ./internal/ui/... ./pkg/coreapi/... -count=1`
- `go test ./... -count=1`
- `go run . -p "RC smoke hello" --output-format json`
- `go run . exec "RC exec smoke" --output json --timeout 30s`
- In `../eos-core-rs`: `cargo test --workspace`
- In `../eos-core-rs`: `cargo clippy --workspace --all-targets -- -D warnings`

Latest cleanup verification:

- `git add -n` in `../eos-core-rs` includes only the explicit release scripts
  and excludes `scripts/eos-signing-key*.pem`.
- `git add -n` in this repo includes the vendored Windows sidecar executable
  and `manifest.json`.
- `EOS_RELEASE_ARTIFACT_CHECK=1 go test ./internal/architecture/... -count=1`
  passed after the staging-plan cleanup.
- `go test ./internal/cli/... -count=1` passed after the staging-plan cleanup.

## Suggested Review Groups

1. Rust private core repository

   Scope: `../eos-core-rs`.

   Include the full Rust workspace: `Cargo.toml`, `Cargo.lock`, `crates/`,
   release scripts, `.gitignore`, `README.md`, `ARCHITECTURE.md`, and
   `TASKS.md`. Do not include generated signing keys.

   Review focus: dependency direction, app-server method coverage,
   runtime/store/tool/sandbox behavior, and artifact packaging script.

2. Public protocol, sidecar, and artifact boundary

   Scope:

   - `.gitignore`
   - `installer.iss`
   - `pkg/coreapi/generated/`
   - `pkg/coreapi/sidecar/`
   - `pkg/protocol/jsonrpc/methods.go`
   - `pkg/protocol/jsonrpc/methods_test.go`
   - `scripts/regen-protocol.ps1`

   Review focus: manifest checksum/signature enforcement, installer packaging,
   method coverage, process lifecycle, notification delivery, Go DTO
   generation, and signed vendored artifact.

3. Go production cutover and TUI facade

   Scope:

   - `internal/cli/print.go`
   - `internal/cli/exec.go`
   - `internal/cli/root.go`
   - `internal/ui/adapter/core_client.go`
   - `internal/ui/startup.go`
   - `internal/ui/app.go`
   - `internal/ui/slash_runtime.go`
   - related `internal/ui/**` tests

   Review focus: Rust-only engine selection, no silent legacy fallback,
   headless session creation, `turn.*` event compatibility, adapter event
   lifecycle cleanup, and UI import boundaries.

4. Legacy isolation and parity fixtures

   Scope:

   - `LEGACY.md`
   - `cmd/eos-tool-host/`
   - `internal/bridge/`
   - `internal/cli/root_legacy.go`
   - `internal/cli/app_server.go`
   - `pkg/core/`
   - `pkg/coreapi/parity/`
   - `pkg/coreapi/sidecar/toolhost/legacy_host.go`
   - `internal/ui/adapter/runtime_legacy_test.go`

   Review focus: `//go:build legacy` isolation, default build exclusion,
   parity-only usage, and no production imports from TUI/CLI.

5. Architecture and regression gates

   Scope:

   - `internal/architecture/`
   - `pkg/coreapi/jsonrpc/e2e_chain_test.go`
   - `pkg/coreapi/sidecar/*_test.go`
   - `pkg/coreapi/engineprovider/*_test.go`
   - `pkg/protocol/jsonrpc/message_test.go`

   Review focus: release artifact hard fail behavior, missing binary/method
   failures, no source/key leakage, e2e JSON-RPC chain, and event delivery.

## Suggested Staging Commands

Stage in topic order instead of using `git add .`.

```powershell
# 1. Public protocol / sidecar / artifact boundary
git add .gitignore MIGRATION_RC_PR_PLAN.md
git add installer.iss
git add cmd/protocol-gen/main.go pkg/coreapi/generated pkg/coreapi/engineprovider pkg/coreapi/sidecar
git add pkg/protocol/jsonrpc/methods.go pkg/protocol/jsonrpc/methods_test.go scripts/regen-protocol.ps1

# 2. Go CLI / TUI cutover
git add internal/cli/print.go internal/cli/print_test.go internal/cli/exec.go internal/cli/exec_test.go internal/cli/root.go internal/cli/root_legacy.go
git add internal/ui/adapter/core_client.go internal/ui/adapter/boundary_test.go internal/ui/adapter/runtime_events.go internal/ui/adapter/runtime_jsonrpc_test.go internal/ui/adapter/runtime_legacy_test.go
git add internal/ui/app.go internal/ui/app_model_test.go internal/ui/app_model_test_engine_test.go internal/ui/slash_runtime.go internal/ui/startup.go
git rm internal/ui/adapter/runtime.go internal/ui/adapter/runtime_events_test.go

# 3. Legacy isolation and parity fixtures
git add LEGACY.md cmd/eos-tool-host internal/bridge pkg/core pkg/coreapi/parity
git add internal/cli/app_server.go internal/cli/app_server_test.go

# 4. Architecture / regression gates
git add internal/architecture internal/serve/capabilities_test.go internal/serve/server_options_test.go internal/serve/server_test.go
git add pkg/coreapi/jsonrpc/e2e_chain_test.go
git add pkg/coreapi/jsonrpc/server.go pkg/coreapi/jsonrpc/server_test.go
git add pkg/coreapi/types.go pkg/coreapi/types_test.go pkg/coreapi/agent_model_runner.go pkg/coreapi/agent_model_runner_test.go
git add pkg/protocol/jsonrpc/message.go pkg/protocol/jsonrpc/message_test.go
```

For the private Rust repository:

```powershell
git add .gitignore ARCHITECTURE.md Cargo.toml Cargo.lock README.md TASKS.md crates
git add scripts/check-architecture.ps1 scripts/generate-keys.ps1 scripts/package-public-artifact.ps1
```

## Files To Exclude Or Inspect Before Commit

- `dev/null`: untracked 32 MB file; likely an accidental artifact and should
  not be included unless someone can explain its purpose.
- Root-level local binaries such as `eos.exe`, `eos_test.exe`,
  `eos-tool-host.exe`, `main.exe`, `parity.test.exe`, and `vb-coding.exe`.
  These remain ignored local build outputs.
- `docs/`: currently ignored by `.gitignore`. Local updates in this directory
  are useful reference material but will not enter the PR unless force-added or
  moved to a tracked location.
- Any `*.tmp`, `*.log`, private keys, Cargo metadata under `eos-cli`, or Rust
  debug symbols.
- In `../eos-core-rs`, exclude generated signing material such as
  `scripts/eos-signing-key.pem` and `scripts/eos-signing-key.pub.pem`.

## Artifact Notes

- `pkg/coreapi/sidecar/binaries/x86_64-pc-windows-gnu/eos-core.exe` is the
  signed vendored Windows sidecar artifact and must be included with its
  adjacent `manifest.json`.
- `.gitignore` intentionally allows only this vendored sidecar executable path;
  it does not unignore arbitrary `.exe` build products.
- Before public distribution, confirm the manifest signature was produced with
  the production Ed25519 key rather than a smoke-test key.

## Remaining Post-RC Debt

- Retire `internal/runtime`, `internal/tools`, `internal/serve`, and
  `internal/daemon` after the Rust HTTP/gateway path replaces the hidden legacy
  endpoints.
- Delete `pkg/core`, `internal/bridge`, `cmd/eos-tool-host`, and
  `pkg/coreapi/parity` after the parity harness is no longer needed.
- Decide whether ignored `docs/` content should remain local-only or be moved
  into tracked release docs.
