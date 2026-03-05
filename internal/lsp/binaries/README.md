# LSP Binaries

This directory contains LSP server binaries for embedding.

## Quick Start

To embed gopls binary in your build:

**Linux/macOS:**
```bash
./scripts/embed_gopls.sh
```

**Windows:**
```batch
scripts\embed_gopls.bat
```

Then build with:
```bash
go build -tags with_gopls
```

## Manual Installation

If you want to manually add gopls for a specific platform:

```bash
# Install gopls
go install golang.org/x/tools/gopls@v0.21.1

# Copy to binaries directory with appropriate name
# Linux/macOS:
cp $(go env GOPATH)/bin/gopls internal/lsp/binaries/gopls-$(go env GOOS)-$(go env GOARCH)

# Windows:
copy %GOPATH%\bin\gopls.exe internal\lsp\binaries\gopls-windows-amd64.exe
```

## Platform Naming Convention

Binaries must be named following this convention:

| Platform | Binary Name |
|----------|-------------|
| Windows AMD64 | `gopls-windows-amd64.exe` |
| Windows ARM64 | `gopls-windows-arm64.exe` |
| macOS AMD64 | `gopls-darwin-amd64` |
| macOS ARM64 (Apple Silicon) | `gopls-darwin-arm64` |
| Linux AMD64 | `gopls-linux-amd64` |
| Linux ARM64 | `gopls-linux-arm64` |

## Build Variants

1. **Default build** (`go build`):
   - LSP enabled with external server detection
   - Requires gopls in PATH or GOPATH/bin
   - Size: ~15-20 MB

2. **without_lsp build** (`go build -tags without_lsp`):
   - LSP completely disabled
   - Minimal dependencies
   - Size: ~10-12 MB

3. **with_gopls build** (`go build -tags with_gopls`):
   - LSP with embedded gopls binary
   - Works without external gopls installation
   - Size: ~25-30 MB (depends on platform)

## Verification

To verify the embedded gopls is working:

```bash
# Build with embedded gopls
go build -tags with_gopls

# Check the diagnostics panel
./vb-coding
# Type: /diagnostics
# Should show "LSP enabled" if gopls is embedded and working
```

## Troubleshooting

**"embedded gopls not found" error:**
- Ensure the binary files are in `internal/lsp/binaries/`
- Check the binary name matches your platform (see table above)
- Run the embed script again

**"permission denied" error:**
- Make sure the binary has execute permission:
  ```bash
  chmod +x internal/lsp/binaries/gopls-*
  ```

**LSP not working:**
- Check if gopls is installed: `which gopls` or `where gopls`
- Verify Go version: `go version` (requires Go 1.21+)
- Check diagnostics panel for error messages
