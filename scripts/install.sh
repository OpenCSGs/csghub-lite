#!/bin/sh
# csghub-lite install script
# Usage: curl -fsSL https://raw.githubusercontent.com/opencsgs/csghub-lite/main/scripts/install.sh | sh
set -eu

REPO="${REPO:-OpenCSGs/csghub-lite}"
INSTALL_DIR="${INSTALL_DIR:-}"
INSTALL_DIR_DEFAULT="/usr/local/bin"
BINARY_NAME="${BINARY_NAME:-csghub-lite}"
LLAMA_CPP_REPO="ggml-org/llama.cpp"

GITHUB_API="https://api.github.com/repos"
GITLAB_HOST="https://git-devops.opencsg.com"
GITLAB_API="${GITLAB_HOST}/api/v4/projects"
GITLAB_CSGHUB_ID="392"
GITLAB_LLAMA_ID="393"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info() { printf "${GREEN}[INFO]${NC} %s\n" "$1"; }
warn() { printf "${YELLOW}[WARN]${NC} %s\n" "$1"; }
error() { printf "${RED}[ERROR]${NC} %s\n" "$1" >&2; exit 1; }
step() { printf "${CYAN}[%s/%s]${NC} %s\n" "$1" "$2" "$3"; }

detect_os() {
    case "$(uname -s)" in
        Linux)  echo "linux" ;;
        Darwin) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *) error "Unsupported operating system: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) error "Unsupported architecture: $(uname -m)" ;;
    esac
}

detect_region() {
    _region="${CSGHUB_LITE_REGION:-}"
    if [ -n "$_region" ]; then echo "$_region"; return; fi
    _country="$(curl -fsSL --connect-timeout 3 --max-time 5 https://ipinfo.io/country 2>/dev/null | tr -d '[:space:]' || true)"
    if [ "$_country" = "CN" ]; then
        echo "CN"
    elif [ -n "$_country" ]; then
        echo "INTL"
    else
        echo "CN"
    fi
}

download() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --connect-timeout 10 --max-time 120 -o "$2" "$1"
    elif command -v wget >/dev/null 2>&1; then
        wget --timeout=10 -qO "$2" "$1"
    else
        error "curl or wget is required"
    fi
}

download_text() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --connect-timeout 10 --max-time 30 "$1"
    elif command -v wget >/dev/null 2>&1; then
        wget --timeout=10 -qO- "$1"
    else
        error "curl or wget is required"
    fi
}

# Try downloading a file from multiple URLs in order (first arg = destination)
try_download() {
    _td_dest="$1"; shift
    for _td_url in "$@"; do
        if download "$_td_url" "$_td_dest"; then
            return 0
        fi
    done
    return 1
}

# Try fetching text from multiple URLs in order
try_download_text() {
    for _tdt_url in "$@"; do
        _tdt_result="$(download_text "$_tdt_url" 2>/dev/null || true)"
        if [ -n "$_tdt_result" ]; then
            printf "%s\n" "$_tdt_result"
            return 0
        fi
    done
    return 1
}

# Region-aware file download: GitLab first for CN, GitHub first for INTL
region_download() {
    _rd_dest="$1"
    _rd_github="$2"
    _rd_gitlab="$3"
    if [ "$REGION" = "CN" ]; then
        try_download "$_rd_dest" "$_rd_gitlab" "$_rd_github"
    else
        try_download "$_rd_dest" "$_rd_github" "$_rd_gitlab"
    fi
}

# Region-aware text download
region_download_text() {
    _rdt_github="$1"
    _rdt_gitlab="$2"
    if [ "$REGION" = "CN" ]; then
        try_download_text "$_rdt_gitlab" "$_rdt_github"
    else
        try_download_text "$_rdt_github" "$_rdt_gitlab"
    fi
}

get_latest_version() {
    _gh_url="${GITHUB_API}/${REPO}/releases/latest"
    _gl_url="${GITLAB_API}/${GITLAB_CSGHUB_ID}/releases/permalink/latest"
    _json="$(region_download_text "$_gh_url" "$_gl_url" 2>/dev/null || true)"
    if [ -n "$_json" ]; then
        _tag="$(printf "%s\n" "$_json" | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
        if [ -n "$_tag" ]; then
            printf "%s\n" "$_tag"
            return 0
        fi
    fi
    return 1
}

install_llama_server() {
    _existing_llama="$(command -v llama-server 2>/dev/null || true)"
    if [ -n "$_existing_llama" ]; then
        info "llama-server found at ${_existing_llama}, upgrading to latest version..."
    else
        warn "llama-server not found. It is required for model inference."
    fi

    _auto="${CSGHUB_LITE_AUTO_INSTALL_LLAMA_SERVER:-1}"
    if [ "$_auto" != "1" ]; then
        warn "Auto-install disabled (CSGHUB_LITE_AUTO_INSTALL_LLAMA_SERVER=${_auto})."
        return
    fi

    _custom="${CSGHUB_LITE_LLAMA_CPP_INSTALL_CMD:-}"
    if [ -n "$_custom" ]; then
        info "Installing llama.cpp via custom command..."
        if sh -c "$_custom" >/dev/null 2>&1 && command -v llama-server >/dev/null 2>&1; then
            info "llama-server installed successfully."
            return
        fi
        warn "Custom install command failed."
    fi

    OS="$(detect_os)"
    ARCH="$(detect_arch)"
    info "Downloading llama.cpp for ${OS}/${ARCH}..."

    _gh_url="${GITHUB_API}/${LLAMA_CPP_REPO}/releases/latest"
    _gl_url="${GITLAB_API}/${GITLAB_LLAMA_ID}/releases/permalink/latest"
    _llama_json="$(region_download_text "$_gh_url" "$_gl_url" 2>/dev/null || true)"
    if [ -z "$_llama_json" ]; then
        warn "Failed to query llama.cpp release metadata."
        warn "Install manually from: https://github.com/${LLAMA_CPP_REPO}/releases"
        return
    fi

    _llama_tag="$(printf "%s\n" "$_llama_json" | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
    if [ -z "$_llama_tag" ]; then
        warn "Failed to parse llama.cpp release tag."
        return
    fi

    _llama_asset=""
    case "$OS" in
        darwin)
            case "$ARCH" in
                amd64) _llama_asset="llama-${_llama_tag}-bin-macos-x64.tar.gz" ;;
                arm64) _llama_asset="llama-${_llama_tag}-bin-macos-arm64.tar.gz" ;;
            esac ;;
        linux)
            case "$ARCH" in
                amd64)
                    if command -v nvidia-smi >/dev/null 2>&1; then
                        _llama_asset="llama-${_llama_tag}-bin-ubuntu-vulkan-x64.tar.gz"
                        info "NVIDIA GPU detected, using Vulkan build for GPU acceleration."
                    else
                        _llama_asset="llama-${_llama_tag}-bin-ubuntu-x64.tar.gz"
                    fi ;;
                arm64) _llama_asset="llama-${_llama_tag}-bin-ubuntu-arm64.tar.gz" ;;
            esac ;;
    esac
    if [ -z "$_llama_asset" ]; then
        warn "No compatible llama.cpp asset for ${OS}/${ARCH}."
        warn "Install manually from: https://github.com/${LLAMA_CPP_REPO}/releases"
        return
    fi

    _github_dl="https://github.com/${LLAMA_CPP_REPO}/releases/download/${_llama_tag}/${_llama_asset}"
    _gitlab_dl="${GITLAB_API}/${GITLAB_LLAMA_ID}/packages/generic/llama-cpp/${_llama_tag}/${_llama_asset}"

    _tmpdir="$(mktemp -d)"
    _archive="${_tmpdir}/${_llama_asset}"
    if ! region_download "$_archive" "$_github_dl" "$_gitlab_dl"; then
        warn "Failed to download llama.cpp."
        warn "Install manually from: https://github.com/${LLAMA_CPP_REPO}/releases"
        rm -rf "$_tmpdir"
        return
    fi

    tar xzf "$_archive" -C "$_tmpdir"
    _llama_bin="$(find "$_tmpdir" -name "llama-server" -type f | head -1)"
    if [ -z "$_llama_bin" ]; then
        warn "llama-server not found in archive."
        rm -rf "$_tmpdir"
        return
    fi
    _llama_extract_dir="$(dirname "$_llama_bin")"
    chmod +x "$_llama_bin"

    _llama_dir="${CSGHUB_LITE_LLAMA_SERVER_INSTALL_DIR:-}"
    if [ -z "$_llama_dir" ]; then
        if [ -n "$_existing_llama" ]; then
            _llama_dir="$(dirname "$_existing_llama")"
        elif command -v csghub-lite >/dev/null 2>&1; then
            _llama_dir="$(dirname "$(command -v csghub-lite)")"
        else
            _llama_dir="${INSTALL_DIR_DEFAULT}"
        fi
    fi
    mkdir -p "$_llama_dir"

    # Install llama-server binary and all shared libraries it depends on
    if [ -w "$_llama_dir" ]; then
        mv "$_llama_bin" "$_llama_dir/"
        find "$_llama_extract_dir" -name "*.dylib" -o -name "*.so" -o -name "*.so.*" | while read -r _lib; do
            mv "$_lib" "$_llama_dir/"
        done
    else
        info "Requires sudo to install llama-server."
        sudo mv "$_llama_bin" "$_llama_dir/"
        find "$_llama_extract_dir" -name "*.dylib" -o -name "*.so" -o -name "*.so.*" | while read -r _lib; do
            sudo mv "$_lib" "$_llama_dir/"
        done
    fi

    # Fix @rpath on macOS so llama-server can find co-located dylibs
    if [ "$OS" = "darwin" ] && command -v install_name_tool >/dev/null 2>&1; then
        _llama_installed="${_llama_dir}/llama-server"
        if [ -f "$_llama_installed" ]; then
            # Add @executable_path to rpath (ignore error if already present)
            if [ -w "$_llama_installed" ]; then
                install_name_tool -add_rpath @executable_path "$_llama_installed" 2>/dev/null || true
            else
                sudo install_name_tool -add_rpath @executable_path "$_llama_installed" 2>/dev/null || true
            fi
        fi
    fi
    rm -rf "$_tmpdir"
    info "llama-server installed successfully."
}

check_existing() {
    _existing="$(command -v "$BINARY_NAME" 2>/dev/null || true)"
    if [ -z "$_existing" ]; then
        return 0
    fi

    _old_ver="$("$_existing" --version 2>/dev/null | head -1 || echo "unknown")"

    printf "\n"
    warn "Existing installation detected:"
    printf "  ${BOLD}Binary:${NC}  %s\n" "$_existing"
    printf "  ${BOLD}Version:${NC} %s\n" "$_old_ver"

    _has_running=false
    if pgrep -x "$BINARY_NAME" >/dev/null 2>&1; then
        _has_running=true
        warn "Running ${BINARY_NAME} process(es) detected."
    fi

    if [ "${CSGHUB_LITE_FORCE:-}" = "1" ]; then
        if [ "$_has_running" = true ]; then
            info "Force mode: stopping running processes..."
            pkill -x "$BINARY_NAME" 2>/dev/null || true
            sleep 1
            pkill -9 -x "$BINARY_NAME" 2>/dev/null || true
        fi
        return 0
    fi

    printf "\n"
    if [ "$_has_running" = true ]; then
        printf "${YELLOW}Stop running instances and replace with the new version? [y/N] ${NC}"
    else
        printf "${YELLOW}Replace the existing installation? [y/N] ${NC}"
    fi

    _answer=""
    if [ -t 0 ]; then
        read -r _answer
    elif [ -e /dev/tty ]; then
        read -r _answer < /dev/tty
    else
        printf "\n"
        info "Non-interactive mode: proceeding with replacement."
        _answer="y"
    fi

    case "$_answer" in
        [yY]|[yY][eE][sS])
            if [ "$_has_running" = true ]; then
                info "Stopping running processes..."
                pkill -x "$BINARY_NAME" 2>/dev/null || true
                sleep 1
                pkill -9 -x "$BINARY_NAME" 2>/dev/null || true
            fi
            ;;
        *)
            printf "\n"
            info "Installation cancelled."
            exit 0
            ;;
    esac
    printf "\n"
}

main() {
    TOTAL_STEPS=6
    printf "\n${BOLD}Installing ${BINARY_NAME}${NC}\n\n"

    # Step 1: Detect environment
    step 1 "$TOTAL_STEPS" "Detecting environment..."
    OS="$(detect_os)"
    ARCH="$(detect_arch)"
    REGION="$(detect_region)"
    info "OS: ${OS}, Arch: ${ARCH}, Region: ${REGION}"

    # Step 2: Check existing installation
    step 2 "$TOTAL_STEPS" "Checking for existing installation..."
    check_existing

    # Step 3: Resolve version
    step 3 "$TOTAL_STEPS" "Resolving version..."
    VERSION="${CSGHUB_LITE_VERSION:-}"
    if [ -z "$VERSION" ]; then
        VERSION="$(get_latest_version)" || true
        if [ -z "$VERSION" ]; then
            error "Could not determine latest version. Set CSGHUB_LITE_VERSION env var manually."
        fi
    fi
    info "Version: ${VERSION}"

    # Step 4: Download
    step 4 "$TOTAL_STEPS" "Downloading ${BINARY_NAME} ${VERSION}..."
    EXT="tar.gz"
    [ "$OS" = "windows" ] && EXT="zip"
    ARCHIVE_NAME="${BINARY_NAME}_${VERSION#v}_${OS}-${ARCH}.${EXT}"

    _github_url="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE_NAME}"
    _gitlab_url="${GITLAB_API}/${GITLAB_CSGHUB_ID}/packages/generic/${BINARY_NAME}/${VERSION#v}/${ARCHIVE_NAME}"

    TMPDIR="$(mktemp -d)"
    ARCHIVE_PATH="${TMPDIR}/${ARCHIVE_NAME}"
    if ! region_download "$ARCHIVE_PATH" "$_github_url" "$_gitlab_url"; then
        rm -rf "$TMPDIR"
        error "Failed to download ${BINARY_NAME} ${VERSION}."
    fi
    info "Download complete."

    # Step 5: Extract and install
    step 5 "$TOTAL_STEPS" "Installing..."
    case "$EXT" in
        tar.gz) tar xzf "$ARCHIVE_PATH" -C "$TMPDIR" ;;
        zip)    unzip -q "$ARCHIVE_PATH" -d "$TMPDIR" ;;
    esac

    BINARY_PATH="$(find "$TMPDIR" -name "$BINARY_NAME" -type f | head -1)"
    if [ -z "$BINARY_PATH" ]; then
        error "Binary not found in archive"
    fi
    chmod +x "$BINARY_PATH"

    if [ -z "$INSTALL_DIR" ]; then
        EXISTING_BIN="$(command -v "$BINARY_NAME" 2>/dev/null || true)"
        if [ -n "$EXISTING_BIN" ]; then
            INSTALL_DIR="$(dirname "$EXISTING_BIN")"
        else
            INSTALL_DIR="${INSTALL_DIR_DEFAULT}"
        fi
    fi

    TARGET="${INSTALL_DIR}/${BINARY_NAME}"
    if [ -w "$INSTALL_DIR" ]; then
        mv "$BINARY_PATH" "$TARGET"
    else
        info "Requires sudo to install to ${INSTALL_DIR}"
        sudo mv "$BINARY_PATH" "$TARGET"
    fi
    rm -rf "$TMPDIR"

    ACTIVE_BIN="$(command -v "$BINARY_NAME" 2>/dev/null || true)"
    if [ -n "$ACTIVE_BIN" ] && [ "$ACTIVE_BIN" != "$TARGET" ]; then
        warn "Current PATH resolves ${BINARY_NAME} to ${ACTIVE_BIN}, not ${TARGET}"
    fi

    # Step 6: Install llama-server
    step 6 "$TOTAL_STEPS" "Setting up inference engine..."
    install_llama_server

    # Done
    printf "\n${GREEN}${BOLD}✔ ${BINARY_NAME} ${VERSION} installed successfully!${NC}\n\n"

    printf "${BOLD}Quick start:${NC}\n"
    printf "  ${BINARY_NAME} serve                       # Start server with Web UI\n"
    printf "  ${BINARY_NAME} run Qwen/Qwen3-0.6B-GGUF    # Run a model\n"
    printf "  ${BINARY_NAME} ps                          # List running models\n"
    printf "  ${BINARY_NAME} login                       # Set CSGHub token\n"
    printf "  ${BINARY_NAME} --help                      # Show all commands\n"
    printf "\n"
    printf "${BOLD}Web UI:${NC}\n"
    printf "  Start the server and open ${CYAN}http://localhost:11435${NC} in your browser.\n"
    printf "  Dashboard, Marketplace, Library and Chat are all available.\n"
    printf "\n"

    printf "${BOLD}Want more?${NC}\n"
    printf "  Visit ${CYAN}https://opencsg.com${NC} for advanced features,\n"
    printf "  enterprise solutions, and the full CSGHub platform.\n"
    printf "\n"
}

main "$@"
