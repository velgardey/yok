#!/bin/bash
set -euo pipefail

# Set colors
BLUE='\033[0;34m'
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Variables
GITHUB_REPO="velgardey/yok"
INSTALL_DIR="/usr/local/bin"

echo -e "${BLUE}Installing Yok CLI...${NC}"

fail() {
    echo -e "${RED}$1${NC}" >&2
    exit 1
}

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

# Map architecture to Go arch
if [ "$ARCH" = "x86_64" ]; then
    ARCH="amd64"
elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
    ARCH="arm64"
else
    fail "Unsupported architecture: $ARCH"
fi

# Get latest release info
echo -e "${BLUE}Fetching the latest version...${NC}"
LATEST=$(curl -fsSL "https://api.github.com/repos/$GITHUB_REPO/releases/latest" | grep tag_name | cut -d '"' -f 4 || true)

if [ -z "$LATEST" ]; then
    fail "Failed to get the latest version. Please check your connection."
fi

echo -e "${BLUE}Latest version: $LATEST${NC}"

# Create temporary directory
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

# Download the archive and the checksums published by goreleaser
VERSION="${LATEST#v}"
ARCHIVE_NAME="yok_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/$GITHUB_REPO/releases/download/$LATEST"

echo -e "${BLUE}Downloading $ARCHIVE_NAME...${NC}"
curl -fsSL "$BASE_URL/$ARCHIVE_NAME" -o "$TMP_DIR/$ARCHIVE_NAME" || fail "Failed to download $BASE_URL/$ARCHIVE_NAME"
curl -fsSL "$BASE_URL/checksums.txt" -o "$TMP_DIR/checksums.txt" || fail "Failed to download checksums.txt"

# Verify the download against the published sha256
echo -e "${BLUE}Verifying download...${NC}"
EXPECTED=$(grep " $ARCHIVE_NAME\$" "$TMP_DIR/checksums.txt" | awk '{print $1}')
ACTUAL=$(sha256sum "$TMP_DIR/$ARCHIVE_NAME" | awk '{print $1}')
if [ -z "$EXPECTED" ]; then
    fail "No checksum found for $ARCHIVE_NAME in checksums.txt"
fi
if [ "$EXPECTED" != "$ACTUAL" ]; then
    fail "Checksum mismatch for $ARCHIVE_NAME (expected $EXPECTED, got $ACTUAL)"
fi

# Extract the binary
echo -e "${BLUE}Extracting...${NC}"
tar -xzf "$TMP_DIR/$ARCHIVE_NAME" -C "$TMP_DIR"

# Install the binary
echo -e "${BLUE}Installing to $INSTALL_DIR...${NC}"
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_DIR/yok" "$INSTALL_DIR/"
else
    echo -e "${BLUE}Sudo access required to install to $INSTALL_DIR${NC}"
    sudo mv "$TMP_DIR/yok" "$INSTALL_DIR/"
fi

chmod +x "$INSTALL_DIR/yok"

echo -e "${GREEN}✅ Yok CLI installed successfully!${NC}"
echo -e "${BLUE}Run 'yok --help' to get started${NC}"
