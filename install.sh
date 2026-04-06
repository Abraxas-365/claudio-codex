#!/bin/sh
set -e

REPO="Abraxas-365/claudio-codex"
INSTALL_DIR="$HOME/.claudio/plugins"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
    darwin|linux) ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Get latest release tag
LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$LATEST" ]; then
    echo "Failed to fetch latest release"
    exit 1
fi

ASSET="claudio-codex-${OS}-${ARCH}"
URL="https://github.com/$REPO/releases/download/$LATEST/$ASSET"

echo "Downloading claudio-codex $LATEST for $OS/$ARCH..."
mkdir -p "$INSTALL_DIR"

curl -fsSL "$URL" -o "$INSTALL_DIR/claudio-codex"
chmod +x "$INSTALL_DIR/claudio-codex"

echo "Installed claudio-codex to $INSTALL_DIR/claudio-codex"
echo "Claudio will automatically discover it on next startup."
