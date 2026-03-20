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
        curl -fSL --connect-timeout 15 --retry 3 --retry-delay 5 -o "$2" "$1"
    elif command -v wget >/dev/null 2>&1; then
        wget --timeout=15 --tries=3 -O "$2" "$1"
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

    # Build ordered list of candidate asset names (best match first)
    _candidates=""
    case "$OS" in
        darwin)
            case "$ARCH" in
                amd64) _candidates="llama-${_llama_tag}-bin-macos-x64.tar.gz" ;;
                arm64) _candidates="llama-${_llama_tag}-bin-macos-arm64.tar.gz" ;;
            esac ;;
        linux)
            case "$ARCH" in
                amd64) _arch_token="x64" ;;
                arm64) _arch_token="arm64" ;;
            esac
            if [ -n "${_arch_token:-}" ]; then
                if command -v nvidia-smi >/dev/null 2>&1; then
                    info "NVIDIA GPU detected, trying CUDA build first."
                    _candidates="llama-${_llama_tag}-bin-ubuntu-cuda-12.4-${_arch_token}.tar.gz"
                    _candidates="${_candidates} llama-${_llama_tag}-bin-ubuntu-vulkan-${_arch_token}.tar.gz"
                    _candidates="${_candidates} llama-${_llama_tag}-bin-ubuntu-${_arch_token}.tar.gz"
                else
                    _candidates="llama-${_llama_tag}-bin-ubuntu-${_arch_token}.tar.gz"
                fi
            fi ;;
    esac
    if [ -z "$_candidates" ]; then
        warn "No compatible llama.cpp asset for ${OS}/${ARCH}."
        warn "Install manually from: https://github.com/${LLAMA_CPP_REPO}/releases"
        return
    fi

    _tmpdir="$(mktemp -d)"
    _downloaded=false
    _llama_asset=""
    for _candidate in $_candidates; do
        _github_dl="https://github.com/${LLAMA_CPP_REPO}/releases/download/${_llama_tag}/${_candidate}"
        _gitlab_dl="${GITLAB_API}/${GITLAB_LLAMA_ID}/packages/generic/llama-cpp/${_llama_tag}/${_candidate}"
        _archive="${_tmpdir}/${_candidate}"
        if region_download "$_archive" "$_github_dl" "$_gitlab_dl"; then
            _llama_asset="$_candidate"
            _downloaded=true
            break
        fi
        warn "Asset ${_candidate} not available, trying next option..."
    done
    if [ "$_downloaded" = false ]; then
        warn "Failed to download llama.cpp."
        warn "Install manually from: https://github.com/${LLAMA_CPP_REPO}/releases"
        rm -rf "$_tmpdir"
        return
    fi
    info "Downloaded ${_llama_asset}"

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

install_python_deps() {
    PYTHON_PKGS="torch safetensors gguf transformers"

    # Find Python 3
    _python=""
    for _name in python3.13 python3.12 python3.11 python3.10 python3 python; do
        if command -v "$_name" >/dev/null 2>&1; then
            _ver="$("$_name" -c 'import sys; print(sys.version_info.major)' 2>/dev/null || echo "0")"
            if [ "$_ver" = "3" ]; then
                _python="$_name"
                break
            fi
        fi
    done

    if [ -z "$_python" ]; then
        warn "Python 3 not found. It is required to convert SafeTensors models to GGUF."
        printf "\n${YELLOW}Install Python 3? [y/N] ${NC}"
        _answer=""
        if [ -t 0 ]; then
            read -r _answer
        elif [ -e /dev/tty ]; then
            read -r _answer < /dev/tty
        else
            _answer="n"
        fi
        case "$_answer" in
            [yY]|[yY][eE][sS])
                OS="$(detect_os)"
                case "$OS" in
                    darwin)
                        if command -v brew >/dev/null 2>&1; then
                            info "Installing Python 3 via Homebrew..."
                            brew install python3
                        else
                            warn "Homebrew not found. Install Python from https://www.python.org/downloads/"
                            return
                        fi ;;
                    linux)
                        if command -v apt-get >/dev/null 2>&1; then
                            info "Installing Python 3 via apt..."
                            sudo apt-get update && sudo apt-get install -y python3 python3-pip python3-venv
                        elif command -v dnf >/dev/null 2>&1; then
                            info "Installing Python 3 via dnf..."
                            sudo dnf install -y python3 python3-pip
                        elif command -v yum >/dev/null 2>&1; then
                            info "Installing Python 3 via yum..."
                            sudo yum install -y python3 python3-pip
                        else
                            warn "No supported package manager found. Install Python from https://www.python.org/downloads/"
                            return
                        fi ;;
                esac
                # Re-detect python after install
                for _name in python3.13 python3.12 python3.11 python3.10 python3 python; do
                    if command -v "$_name" >/dev/null 2>&1; then
                        _ver="$("$_name" -c 'import sys; print(sys.version_info.major)' 2>/dev/null || echo "0")"
                        if [ "$_ver" = "3" ]; then
                            _python="$_name"
                            break
                        fi
                    fi
                done
                if [ -z "$_python" ]; then
                    warn "Python 3 installation failed. Install manually from https://www.python.org/downloads/"
                    return
                fi
                info "Python 3 installed: $(${_python} --version 2>&1)"
                ;;
            *)
                warn "Skipping Python setup. SafeTensors auto-conversion will not be available."
                return ;;
        esac
    else
        info "Python 3 found: $(${_python} --version 2>&1)"
    fi

    # Check which packages are missing
    _missing=""
    for _pkg in $PYTHON_PKGS; do
        if ! "$_python" -c "import ${_pkg}" 2>/dev/null; then
            _missing="${_missing} ${_pkg}"
        fi
    done
    _missing="$(echo "$_missing" | sed 's/^ *//')"

    if [ -z "$_missing" ]; then
        info "Python dependencies already installed (torch, safetensors, gguf, transformers)."
        return
    fi

    warn "Missing Python packages: ${_missing}"
    printf "${YELLOW}Install them now? (CPU-only torch, ~300MB) [y/N] ${NC}"
    _answer=""
    if [ -t 0 ]; then
        read -r _answer
    elif [ -e /dev/tty ]; then
        read -r _answer < /dev/tty
    else
        _answer="n"
    fi
    case "$_answer" in
        [yY]|[yY][eE][sS])
            info "Installing: ${_missing}..."
            _pip_args=""
            case " ${_missing} " in
                *" torch "*)
                    _pip_args="--extra-index-url https://download.pytorch.org/whl/cpu" ;;
            esac
            if "$_python" -m pip install $_pip_args $_missing; then
                info "Python dependencies installed successfully."
            else
                warn "pip install failed. Try manually: ${_python} -m pip install ${_pip_args} ${_missing}"
            fi ;;
        *)
            warn "Skipping. Install later with: ${_python} -m pip install ${_missing}" ;;
    esac
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
    TOTAL_STEPS=7
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

    # Step 7: Python & GGUF conversion deps
    step 7 "$TOTAL_STEPS" "Checking Python environment for model conversion..."
    install_python_deps

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
