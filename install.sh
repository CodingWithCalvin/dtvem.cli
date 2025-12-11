#!/bin/bash
set -e

# dtvem installer for macOS and Linux
# Usage: curl -fsSL https://raw.githubusercontent.com/dtvem/dtvem/main/install.sh | bash

REPO="dtvem/dtvem"
INSTALL_DIR="$HOME/.dtvem/bin"

# This will be replaced with the actual version during release
# Format: DTVEM_RELEASE_VERSION="1.0.0"
# Leave empty to fetch latest
DTVEM_RELEASE_VERSION=""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

info() {
    echo -e "${CYAN}→${NC} $1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
}

warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Darwin*)
            echo "darwin"
            ;;
        Linux*)
            echo "linux"
            ;;
        *)
            error "Unsupported operating system: $(uname -s)"
            exit 1
            ;;
    esac
}

# Detect architecture
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)
            echo "amd64"
            ;;
        aarch64|arm64)
            echo "arm64"
            ;;
        *)
            error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac
}

# Get latest release version from GitHub
get_latest_version() {
    if command -v curl &> /dev/null; then
        curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
    elif command -v wget &> /dev/null; then
        wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
    else
        error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi
}

# Download file
download() {
    local url=$1
    local output=$2

    if command -v curl &> /dev/null; then
        curl -fsSL "$url" -o "$output"
    elif command -v wget &> /dev/null; then
        wget -q "$url" -O "$output"
    fi
}

# Verify SHA256 checksum
verify_checksum() {
    local file=$1
    local checksum_file=$2

    if [ ! -f "$checksum_file" ]; then
        error "Checksum file not found: $checksum_file"
        return 1
    fi

    # Extract expected hash from checksum file (format: "hash  filename")
    local expected_hash
    expected_hash=$(awk '{print $1}' "$checksum_file")

    # Calculate actual hash
    local actual_hash
    if command -v sha256sum &> /dev/null; then
        actual_hash=$(sha256sum "$file" | awk '{print $1}')
    elif command -v shasum &> /dev/null; then
        actual_hash=$(shasum -a 256 "$file" | awk '{print $1}')
    else
        warning "Neither sha256sum nor shasum found - skipping checksum verification"
        return 0
    fi

    if [ "$expected_hash" != "$actual_hash" ]; then
        error "Checksum verification failed!"
        error "Expected: $expected_hash"
        error "Actual:   $actual_hash"
        return 1
    fi

    return 0
}

main() {
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}   dtvem installer${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""

    # Detect platform
    OS=$(detect_os)
    ARCH=$(detect_arch)
    info "Detected platform: ${OS}-${ARCH}"

    # Get version (priority: DTVEM_VERSION env var > hardcoded > fetch latest)
    if [ -n "$DTVEM_VERSION" ]; then
        VERSION="$DTVEM_VERSION"
        info "Installing user-specified version: $VERSION"
    elif [ -n "$DTVEM_RELEASE_VERSION" ]; then
        VERSION="$DTVEM_RELEASE_VERSION"
        info "Installing release version: $VERSION"
    else
        info "Fetching latest release..."
        VERSION=$(get_latest_version)
        if [ -z "$VERSION" ]; then
            error "Failed to fetch latest version"
            exit 1
        fi
        success "Latest version: $VERSION"
    fi

    # Strip "v" prefix from version for archive name (GitHub releases use v1.0.0 in paths, but archives are named 1.0.0)
    VERSION_NO_V="${VERSION#v}"

    # Construct download URL
    if [ "$OS" = "darwin" ]; then
        PLATFORM_NAME="macos"
    else
        PLATFORM_NAME="linux"
    fi

    ARCHIVE_NAME="dtvem-${VERSION_NO_V}-${PLATFORM_NAME}-${ARCH}.tar.gz"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE_NAME}"

    info "Download URL: $DOWNLOAD_URL"

    # Create temporary directory
    TMP_DIR=$(mktemp -d)
    trap 'rm -rf $TMP_DIR' EXIT

    # Download archive
    info "Downloading dtvem..."
    ARCHIVE_PATH="$TMP_DIR/$ARCHIVE_NAME"

    if ! download "$DOWNLOAD_URL" "$ARCHIVE_PATH"; then
        error "Failed to download dtvem"
        error "URL: $DOWNLOAD_URL"
        exit 1
    fi

    success "Downloaded successfully"

    # Download and verify checksum
    CHECKSUM_URL="${DOWNLOAD_URL}.sha256"
    CHECKSUM_PATH="$TMP_DIR/${ARCHIVE_NAME}.sha256"

    info "Downloading checksum..."
    if ! download "$CHECKSUM_URL" "$CHECKSUM_PATH"; then
        error "Failed to download checksum file"
        error "URL: $CHECKSUM_URL"
        exit 1
    fi

    info "Verifying checksum..."
    if ! verify_checksum "$ARCHIVE_PATH" "$CHECKSUM_PATH"; then
        error "Archive integrity check failed - aborting installation"
        exit 1
    fi
    success "Checksum verified"

    # Extract archive
    info "Extracting archive..."
    tar -xzf "$ARCHIVE_PATH" -C "$TMP_DIR"
    success "Extracted successfully"

    # Create install directory
    mkdir -p "$INSTALL_DIR"

    # Install binaries
    info "Installing to $INSTALL_DIR..."

    if [ -f "$TMP_DIR/dtvem" ]; then
        mv "$TMP_DIR/dtvem" "$INSTALL_DIR/dtvem"
        chmod +x "$INSTALL_DIR/dtvem"
    else
        error "dtvem binary not found in archive"
        exit 1
    fi

    if [ -f "$TMP_DIR/dtvem-shim" ]; then
        mv "$TMP_DIR/dtvem-shim" "$INSTALL_DIR/dtvem-shim"
        chmod +x "$INSTALL_DIR/dtvem-shim"
    else
        warning "dtvem-shim binary not found in archive"
    fi

    success "Installation complete!"

    # Add install directory to PATH
    echo ""
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        info "Adding $INSTALL_DIR to PATH..."

        # Detect shell
        SHELL_NAME=$(basename "$SHELL")
        case "$SHELL_NAME" in
            bash)
                SHELL_CONFIG="$HOME/.bashrc"
                [ -f "$HOME/.bash_profile" ] && SHELL_CONFIG="$HOME/.bash_profile"
                EXPORT_CMD="export PATH=\"$INSTALL_DIR:\$PATH\""
                ;;
            zsh)
                SHELL_CONFIG="$HOME/.zshrc"
                EXPORT_CMD="export PATH=\"$INSTALL_DIR:\$PATH\""
                ;;
            fish)
                SHELL_CONFIG="$HOME/.config/fish/config.fish"
                EXPORT_CMD="set -gx PATH \"$INSTALL_DIR\" \$PATH"
                mkdir -p "$(dirname "$SHELL_CONFIG")"
                ;;
            *)
                warning "Unknown shell: $SHELL_NAME"
                info "Please add this to your shell config manually:"
                echo "    export PATH=\"$INSTALL_DIR:\$PATH\""
                SHELL_CONFIG=""
                ;;
        esac

        if [ -n "$SHELL_CONFIG" ]; then
            # Check if already in config
            if ! grep -q "$INSTALL_DIR" "$SHELL_CONFIG" 2>/dev/null; then
                {
                    echo ""
                    echo "# Added by dtvem installer"
                    echo "$EXPORT_CMD"
                } >> "$SHELL_CONFIG"
                success "Added to $SHELL_CONFIG"
            else
                info "Already in $SHELL_CONFIG"
            fi
        fi

        # Temporarily add to PATH for this session
        export PATH="$INSTALL_DIR:$PATH"
    else
        info "$INSTALL_DIR is already in PATH"
    fi

    # Run init to add shims directory to PATH
    echo ""
    info "Running dtvem init to add shims directory to PATH..."
    if "$INSTALL_DIR/dtvem" init; then
        success "dtvem is ready to use!"
        info "Both ~/.dtvem/bin and ~/.dtvem/shims have been added to PATH"
    else
        warning "dtvem init failed - you may need to run it manually"
    fi

    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}   Installation successful!${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    info "Next steps:"
    echo "  1. Restart your terminal (or source your shell config)"
    echo "  2. Run: dtvem install python 3.11.0"
    echo "  3. Run: dtvem global python 3.11.0"
    echo ""
    info "For help, run: dtvem help"
    echo ""
}

main
