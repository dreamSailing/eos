#!/bin/bash
# embed_gopls.sh - Download and embed gopls binary for LSP support
# Usage: ./scripts/embed_gopls.sh

set -e

GOPSLS_VERSION="v0.21.1"
BIN_DIR="internal/lsp/binaries"
mkdir -p "$BIN_DIR"

echo "Installing gopls $GOPSLS_VERSION..."
go install "golang.org/x/tools/gopls@$GOPSLS_VERSION"

# Determine GOPATH
GOPATH=$(go env GOPATH)
if [ -z "$GOPATH" ]; then
    GOPATH="$HOME/go"
fi

# Detect current platform
GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)

BINARY_NAME="gopls-${GOOS}-${GOARCH}"
if [ "$GOOS" = "windows" ]; then
    BINARY_NAME="${BINARY_NAME}.exe"
    SOURCE="$GOPATH/bin/gopls.exe"
else
    SOURCE="$GOPATH/bin/gopls"
fi

DEST="$BIN_DIR/$BINARY_NAME"

echo "Copying gopls to $DEST..."
cp "$SOURCE" "$DEST"
chmod +x "$DEST"

echo "Embedded gopls binary created: $DEST"
echo ""
echo "To build with embedded gopls:"
echo "  go build -tags with_gopls"
