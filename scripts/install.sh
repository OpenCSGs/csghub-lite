#!/bin/sh
# csghub-lite install script
# Usage: curl -fsSL https://raw.githubusercontent.com/opencsgs/csghub-lite/main/scripts/install.sh | sh

set -eu

REPO="opencsgs/csghub-lite"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="csghub-lite"

# Colors (when terminal supports them)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { printf "${GREEN}[INFO]${NC} %s\n" "$1"; }
warn() { printf "${YELLOW}[WARN]${NC} %s\n" "$1"; }
error() { printf "${RED}[ERROR]${NC} %s\n" "$1" >&2; exit 1; }

detect_os() {
    OS="$(uname -s)"
    case "$OS" in
        Linux)  echo "linux" ;;
        Darwin) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *) error "Unsupported operating system: $OS" ;;
    esac
}

detect_arch() {
    ARCH="$(uname -m)"
    case "$ARCH" in
        x86_64|amd64)  echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac
}

get_latest_version() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'
    else
        error "curl or wget is required"
    fi
}

download() {
    URL="$1"
    DEST="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "$DEST" "$URL"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$DEST" "$URL"
    else
        error "curl or wget is required"
    fi
}

check_llama_server() {
    if command -v llama-server >/dev/null 2>&1; then
        info "llama-server found: $(command -v llama-server)"
        return
    fi

    warn "llama-server not found. It is required for model inference."
    OS="$(detect_os)"
    case "$OS" in
        darwin)
            warn "Install it with: brew install llama.cpp"
            ;;
        linux)
            warn "Install it from: https://github.com/ggml-org/llama.cpp/releases"
            ;;
        *)
            warn "Download it from: https://github.com/ggml-org/llama.cpp/releases"
            ;;
    esac
}

main() {
    info "Installing ${BINARY_NAME}..."

    OS="$(detect_os)"
    ARCH="$(detect_arch)"
    info "Detected OS: ${OS}, Arch: ${ARCH}"

    VERSION="${CSGHUB_LITE_VERSION:-}"
    if [ -z "$VERSION" ]; then
        info "Fetching latest version..."
        VERSION="$(get_latest_version)"
        if [ -z "$VERSION" ]; then
            error "Could not determine latest version. Set CSGHUB_LITE_VERSION env var manually."
        fi
    fi
    info "Version: ${VERSION}"

    # Determine archive name and extension
    EXT="tar.gz"
    if [ "$OS" = "windows" ]; then
        EXT="zip"
    fi
    ARCHIVE_NAME="${BINARY_NAME}_${VERSION#v}_${OS}_${ARCH}.${EXT}"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE_NAME}"

    info "Downloading ${DOWNLOAD_URL}..."
    TMPDIR="$(mktemp -d)"
    ARCHIVE_PATH="${TMPDIR}/${ARCHIVE_NAME}"
    download "$DOWNLOAD_URL" "$ARCHIVE_PATH"

    info "Extracting..."
    case "$EXT" in
        tar.gz)
            tar xzf "$ARCHIVE_PATH" -C "$TMPDIR"
            ;;
        zip)
            unzip -q "$ARCHIVE_PATH" -d "$TMPDIR"
            ;;
    esac

    # Find the binary
    BINARY_PATH="$(find "$TMPDIR" -name "$BINARY_NAME" -type f | head -1)"
    if [ -z "$BINARY_PATH" ]; then
        error "Binary not found in archive"
    fi
    chmod +x "$BINARY_PATH"

    # Install to INSTALL_DIR (may require sudo)
    TARGET="${INSTALL_DIR}/${BINARY_NAME}"
    if [ -w "$INSTALL_DIR" ]; then
        mv "$BINARY_PATH" "$TARGET"
    else
        info "Requires sudo to install to ${INSTALL_DIR}"
        sudo mv "$BINARY_PATH" "$TARGET"
    fi

    rm -rf "$TMPDIR"

    info "Installed ${BINARY_NAME} ${VERSION} to ${TARGET}"
    info ""
    info "Quick start:"
    info "  ${BINARY_NAME} --version          # Check version"
    info "  ${BINARY_NAME} login              # Set CSGHub token"
    info "  ${BINARY_NAME} search qwen        # Search models"
    info "  ${BINARY_NAME} run Qwen/Qwen3-0.6B-GGUF  # Run a model"
    info ""

    check_llama_server
}

main "$@"
