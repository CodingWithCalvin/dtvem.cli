#!/bin/bash
set -e

# dtvem installer for macOS and Linux
# Usage:
#   Standard: curl -fsSL https://raw.githubusercontent.com/CodingWithCalvin/dtvem.cli/main/install.sh | bash
#   User install: curl -fsSL https://raw.githubusercontent.com/CodingWithCalvin/dtvem.cli/main/install.sh | bash -s -- --user-install

REPO="CodingWithCalvin/dtvem.cli"
USER_INSTALL=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --user-install)
            USER_INSTALL=true
            shift
            ;;
        *)
            shift
            ;;
    esac
done

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

# Get dtvem root directory
# On Linux, respects XDG_DATA_HOME if set (defaults to ~/.local/share/dtvem)
# On macOS, uses ~/.dtvem
get_dtvem_root() {
    # Check for DTVEM_ROOT environment variable first (overrides all)
    if [ -n "$DTVEM_ROOT" ]; then
        echo "$DTVEM_ROOT"
        return
    fi

    local os
    os=$(uname -s)

    if [ "$os" = "Linux" ]; then
        # Respect XDG Base Directory specification
        if [ -n "$XDG_DATA_HOME" ]; then
            echo "$XDG_DATA_HOME/dtvem"
        else
            # XDG default: ~/.local/share
            echo "$HOME/.local/share/dtvem"
        fi
    else
        # macOS and others: use ~/.dtvem
        echo "$HOME/.dtvem"
    fi
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

# Fetch URL content
fetch() {
    local url=$1
    if command -v curl &> /dev/null; then
        curl -fsSL "$url"
    elif command -v wget &> /dev/null; then
        wget -qO- "$url"
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

# Get release info from GitHub API
# Sets RELEASE_TAG and populates asset digests
get_release_info() {
    local version=$1
    local api_url

    if [ -z "$version" ]; then
        api_url="https://api.github.com/repos/${REPO}/releases/latest"
    else
        api_url="https://api.github.com/repos/${REPO}/releases/tags/${version}"
    fi

    RELEASE_JSON=$(fetch "$api_url")
    if [ -z "$RELEASE_JSON" ]; then
        error "Failed to fetch release information"
        exit 1
    fi

    # Extract tag name
    RELEASE_TAG=$(echo "$RELEASE_JSON" | grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
}

# Get SHA256 digest for an asset from the release JSON
# GitHub API returns digest in format: "sha256:hash"
get_asset_digest() {
    local asset_name=$1

    # Extract the digest for the specific asset
    # The JSON structure has assets array with name and digest fields
    # We use grep/sed to parse without requiring jq
    local digest
    digest=$(echo "$RELEASE_JSON" | \
        grep -A 5 "\"name\"[[:space:]]*:[[:space:]]*\"${asset_name}\"" | \
        grep -o '"digest"[[:space:]]*:[[:space:]]*"sha256:[^"]*"' | \
        head -1 | \
        sed 's/.*"digest"[[:space:]]*:[[:space:]]*"sha256:\([^"]*\)".*/\1/')

    if [ -z "$digest" ]; then
        return 1
    fi

    echo "$digest"
}

# Verify SHA256 checksum using GitHub API digest
verify_checksum() {
    local file=$1
    local expected_hash=$2

    if [ -z "$expected_hash" ]; then
        warning "No checksum available from GitHub API - skipping verification"
        return 0
    fi

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

    # Determine install directory based on platform and XDG
    DTVEM_ROOT=$(get_dtvem_root)
    INSTALL_DIR="$DTVEM_ROOT/bin"
    SHIMS_DIR="$DTVEM_ROOT/shims"
    info "Install directory: $INSTALL_DIR"

    # Determine version to install
    local requested_version=""
    if [ -n "$DTVEM_VERSION" ]; then
        requested_version="$DTVEM_VERSION"
        info "Installing user-specified version: $requested_version"
    elif [ -n "$DTVEM_RELEASE_VERSION" ]; then
        requested_version="$DTVEM_RELEASE_VERSION"
        info "Installing release version: $requested_version"
    else
        info "Fetching latest release..."
    fi

    # Get release info from GitHub API
    get_release_info "$requested_version"
    VERSION="$RELEASE_TAG"

    if [ -z "$VERSION" ]; then
        error "Failed to determine version"
        exit 1
    fi

    if [ -z "$requested_version" ]; then
        success "Latest version: $VERSION"
    fi

    # Strip "v" prefix from version for archive name
    VERSION_NO_V="${VERSION#v}"

    # Construct asset name
    if [ "$OS" = "darwin" ]; then
        PLATFORM_NAME="macos"
    else
        PLATFORM_NAME="linux"
    fi

    ARCHIVE_NAME="dtvem-${VERSION_NO_V}-${PLATFORM_NAME}-${ARCH}.tar.gz"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE_NAME}"

    info "Download URL: $DOWNLOAD_URL"

    # Get expected checksum from GitHub API
    info "Fetching checksum from GitHub API..."
    EXPECTED_HASH=$(get_asset_digest "$ARCHIVE_NAME")
    if [ -n "$EXPECTED_HASH" ]; then
        success "Got checksum: ${EXPECTED_HASH:0:16}..."
    else
        warning "Checksum not available from API (may be an older release)"
    fi

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

    # Verify checksum
    info "Verifying checksum..."
    if ! verify_checksum "$ARCHIVE_PATH" "$EXPECTED_HASH"; then
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
    if [ "$USER_INSTALL" = true ]; then
        info "Using user-level PATH"
        if "$INSTALL_DIR/dtvem" init --user -y; then
            success "dtvem is ready to use!"
            info "Both $INSTALL_DIR and $SHIMS_DIR have been added to PATH"
        else
            warning "dtvem init failed - you may need to run it manually"
        fi
    else
        if "$INSTALL_DIR/dtvem" init; then
            success "dtvem is ready to use!"
            info "Both $INSTALL_DIR and $SHIMS_DIR have been added to PATH"
        else
            warning "dtvem init failed - you may need to run it manually"
        fi
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
