#!/usr/bin/env bash
# Build and install labrador into user bin directories (~/.local/bin and ~/.cargo/bin).
#
#   ./install.sh                    # build + install to ~/.local/bin and ~/.cargo/bin
#   LABRADOR_PREFIX=/opt/bin ./install.sh
#
set -euo pipefail
cd "$(dirname "$0")"

echo "Building optimized Labrador binary..."
mkdir -p bin
BUILD_TAGS=""
if [ -d "internal/providers/private" ]; then
    BUILD_TAGS="-tags private"
fi
go build $BUILD_TAGS -ldflags="-s -w" -o bin/labrador ./cmd/labrador

prefix="${LABRADOR_PREFIX:-}"

if [ -n "$prefix" ]; then
    mkdir -p "$prefix"
    install -m 755 bin/labrador "$prefix/labrador"
    echo "Installed Labrador -> $prefix/labrador"
else
    # Install to both ~/.local/bin and ~/.cargo/bin for seamless integration with Dewey and user PATH
    mkdir -p "$HOME/.local/bin" "$HOME/.cargo/bin"
    install -m 755 bin/labrador "$HOME/.local/bin/labrador"
    install -m 755 bin/labrador "$HOME/.cargo/bin/labrador"
    echo "Installed Labrador -> $HOME/.local/bin/labrador and $HOME/.cargo/bin/labrador"

    # Clean up any stale mise version-specific copies to avoid PATH shadowing
    rm -f "$HOME"/.local/share/mise/installs/go/*/bin/labrador 2>/dev/null || true
fi

# Refresh mise shims if mise is installed
if command -v mise >/dev/null 2>&1; then
    mise reshim 2>/dev/null || true
fi

echo "Installation complete!"
