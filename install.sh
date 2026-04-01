#!/bin/bash
set -e

VERSION="2.0.0"
REPO="visvioce/ccg"
BINARY_NAME="ccg"
INSTALL_DIR="/usr/local/bin"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case $ARCH in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

case $OS in
    linux|darwin)
        ;;
    mingw*|msys*|cygwin*)
        OS="windows"
        ;;
    *)
        echo "Unsupported OS: $OS"
        exit 1
        ;;
esac

DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${VERSION}/${BINARY_NAME}-${OS}-${ARCH}"

if [ "$OS" = "windows" ]; then
    DOWNLOAD_URL="${DOWNLOAD_URL}.exe"
    BINARY_NAME="${BINARY_NAME}.exe"
fi

echo "CCG Installer v${VERSION}"
echo "========================"
echo ""
echo "Detected system: ${OS}/${ARCH}"
echo ""

# Check if running as root for installation
if [ "$EUID" -ne 0 ] && [ -w "$INSTALL_DIR" ]; then
    SUDO=""
elif [ "$EUID" -ne 0 ]; then
    SUDO="sudo"
    echo "Note: sudo will be used for installation to ${INSTALL_DIR}"
else
    SUDO=""
fi

# Create temp directory
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

echo "Downloading ${BINARY_NAME}..."
if command -v curl &> /dev/null; then
    curl -L -o "$TMP_DIR/$BINARY_NAME" "$DOWNLOAD_URL"
elif command -v wget &> /dev/null; then
    wget -O "$TMP_DIR/$BINARY_NAME" "$DOWNLOAD_URL"
else
    echo "Error: curl or wget is required"
    exit 1
fi

chmod +x "$TMP_DIR/$BINARY_NAME"

echo "Installing to ${INSTALL_DIR}..."
$SUDO cp "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/"

echo ""
echo "CCG v${VERSION} installed successfully!"
echo ""
echo "Usage:"
echo "  ccg start       - Start the CCG server"
echo "  ccg stop        - Stop the CCG server"
echo "  ccg status      - Show server status"
echo "  ccg model       - Interactive model selection"
echo "  ccg tui         - Open Terminal UI"
echo ""
echo "Configuration: ~/.claude-code-router/config.json"
echo ""
