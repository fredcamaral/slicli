#!/bin/bash

# slicli Installation Script
# Supports Linux, macOS, and Windows (via WSL)

set -e

# Configuration
REPO="fredcamaral/slicli"
INSTALL_DIR="$HOME/.local/bin"
CONFIG_DIR="$HOME/.config/slicli"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Utility functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Detect platform
detect_platform() {
    local os
    local arch
    
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)
    
    case $os in
        linux*)
            os="linux"
            ;;
        darwin*)
            os="darwin"
            ;;
        cygwin*|mingw*|msys*)
            os="windows"
            ;;
        *)
            log_error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
    
    case $arch in
        x86_64|amd64)
            arch="amd64"
            ;;
        arm64|aarch64)
            arch="arm64"
            ;;
        *)
            log_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
    
    echo "${os}-${arch}"
}

# Get latest release version
get_latest_version() {
    local version
    version=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [ -z "$version" ]; then
        log_error "Failed to get latest version"
        exit 1
    fi
    
    echo "$version"
}

# Download and extract release
download_release() {
    local version=$1
    local platform=$2
    local filename="slicli-${version}-${platform}"
    local url="https://github.com/$REPO/releases/download/$version/${filename}.tar.gz"
    local temp_dir="/tmp/slicli-install"
    
    log_info "Downloading slicli $version for $platform..."
    
    # Create temp directory
    mkdir -p "$temp_dir"
    cd "$temp_dir"
    
    # Download and extract
    if command -v curl >/dev/null 2>&1; then
        curl -fSL "$url" -o "${filename}.tar.gz"
    elif command -v wget >/dev/null 2>&1; then
        wget "$url" -O "${filename}.tar.gz"
    else
        log_error "Neither curl nor wget is available"
        exit 1
    fi
    
    tar -xzf "${filename}.tar.gz"
    cd "$filename"
    
    echo "$temp_dir/$filename"
}

# Install slicli
install_slicli() {
    local extract_dir=$1
    
    log_info "Installing slicli..."
    
    # Create directories
    mkdir -p "$INSTALL_DIR"
    mkdir -p "$CONFIG_DIR"
    
    # Install binary
    cp "$extract_dir/slicli" "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/slicli"
    
    # Install themes and plugins
    if [ -d "$extract_dir/themes" ]; then
        cp -r "$extract_dir/themes" "$CONFIG_DIR/"
    fi
    
    if [ -d "$extract_dir/plugins" ]; then
        cp -r "$extract_dir/plugins" "$CONFIG_DIR/"
    fi
    
    # Install default config
    if [ -f "$extract_dir/examples/default.toml" ]; then
        if [ ! -f "$CONFIG_DIR/config.toml" ]; then
            cp "$extract_dir/examples/default.toml" "$CONFIG_DIR/config.toml"
        else
            log_warn "Config file already exists, skipping default config installation"
        fi
    fi
    
    # Install examples
    if [ -d "$extract_dir/examples" ]; then
        cp -r "$extract_dir/examples" "$CONFIG_DIR/"
    fi
}

# Check dependencies
check_dependencies() {
    log_info "Checking dependencies..."
    
    # Check for Chrome/Chromium (optional but recommended)
    if command -v google-chrome >/dev/null 2>&1 || \
       command -v chromium >/dev/null 2>&1 || \
       command -v chromium-browser >/dev/null 2>&1; then
        log_success "Chrome/Chromium found - PDF/image export will work"
    else
        log_warn "Chrome/Chromium not found - PDF/image export will use fallback mode"
        echo "  Install Chrome or Chromium for full export functionality:"
        echo "  - Ubuntu/Debian: sudo apt install chromium-browser"
        echo "  - macOS: brew install --cask google-chrome"
        echo "  - Or download from: https://www.google.com/chrome/"
    fi
}

# Update PATH
update_path() {
    local shell_config
    
    # Detect shell and config file
    case $SHELL in
        */bash)
            shell_config="$HOME/.bashrc"
            ;;
        */zsh)
            shell_config="$HOME/.zshrc"
            ;;
        */fish)
            shell_config="$HOME/.config/fish/config.fish"
            ;;
        *)
            shell_config="$HOME/.profile"
            ;;
    esac
    
    # Check if PATH already contains install directory
    if echo "$PATH" | grep -q "$INSTALL_DIR"; then
        log_success "PATH already includes $INSTALL_DIR"
        return
    fi
    
    # Add to PATH in shell config
    if [ -f "$shell_config" ]; then
        if ! grep -q "$INSTALL_DIR" "$shell_config"; then
            echo "" >> "$shell_config"
            echo "# Added by slicli installer" >> "$shell_config"
            echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$shell_config"
            log_success "Added $INSTALL_DIR to PATH in $shell_config"
        fi
    else
        log_warn "Could not update PATH automatically"
        echo "Please add the following to your shell configuration:"
        echo "export PATH=\"$INSTALL_DIR:\$PATH\""
    fi
}

# Cleanup
cleanup() {
    if [ -d "/tmp/slicli-install" ]; then
        rm -rf "/tmp/slicli-install"
    fi
}

# Main installation function
main() {
    echo "🚀 slicli Installation Script"
    echo "================================"
    
    # Detect platform
    local platform
    platform=$(detect_platform)
    log_info "Detected platform: $platform"
    
    # Get latest version
    local version
    version=$(get_latest_version)
    log_info "Latest version: $version"
    
    # Download and extract
    local extract_dir
    extract_dir=$(download_release "$version" "$platform")
    
    # Install
    install_slicli "$extract_dir"
    
    # Check dependencies
    check_dependencies
    
    # Update PATH
    update_path
    
    # Cleanup
    cleanup
    
    echo ""
    log_success "slicli $version installed successfully!"
    echo ""
    echo "📋 Next steps:"
    echo "  1. Restart your terminal or run: source ~/.bashrc (or equivalent)"
    echo "  2. Verify installation: slicli --version"
    echo "  3. Get started: slicli --help"
    echo "  4. Try example: slicli serve ~/.config/slicli/examples/simple-ppt/presentation.md"
    echo ""
    echo "📁 Installation locations:"
    echo "  Binary: $INSTALL_DIR/slicli"
    echo "  Config: $CONFIG_DIR/"
    echo "  Themes: $CONFIG_DIR/themes/"
    echo "  Plugins: $CONFIG_DIR/plugins/"
    echo ""
    echo "📖 Documentation: https://github.com/$REPO"
}

# Run with error handling
trap cleanup EXIT
main "$@"