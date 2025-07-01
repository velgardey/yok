#!/bin/bash
set -e

# Set colors
BLUE='\033[0;34m'
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Variables
GITHUB_REPO="velgardey/yok"
INSTALL_DIR="/usr/local/bin"

echo -e "${BLUE}Installing Yok CLI...${NC}"

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

# Map architecture to Go arch
if [ "$ARCH" = "x86_64" ]; then
    ARCH="amd64"
elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
    ARCH="arm64"
else
    echo -e "${RED}Unsupported architecture: $ARCH${NC}"
    exit 1
fi

# Get latest release info
echo -e "${BLUE}Fetching the latest version...${NC}"
LATEST=$(curl -s https://api.github.com/repos/$GITHUB_REPO/releases/latest | grep tag_name | cut -d '"' -f 4)

if [ -z "$LATEST" ]; then
    echo -e "${RED}Failed to get the latest version. Please check your connection.${NC}"
    exit 1
fi

echo -e "${BLUE}Latest version: $LATEST${NC}"

# Create temporary directory
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

# Download the archive
ARCHIVE_NAME="yok_${LATEST#v}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/download/$LATEST/$ARCHIVE_NAME"

echo -e "${BLUE}Downloading $ARCHIVE_NAME...${NC}"
if ! curl -sL "$DOWNLOAD_URL" -o "$TMP_DIR/$ARCHIVE_NAME"; then
    echo -e "${RED}Failed to download $DOWNLOAD_URL${NC}"
    exit 1
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