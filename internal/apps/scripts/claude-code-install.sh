#!/usr/bin/env bash
set -euo pipefail

TARGET="${1:-latest}"
PACKAGE="@anthropic-ai/claude-code"
GCS_BUCKET="https://storage.googleapis.com/claude-code-dist-86c565f3-f756-42ad-8dfa-d59b1c096819/claude-code-releases"
DOWNLOAD_DIR="${HOME}/.claude/downloads"
DOWNLOADER=""

emit_progress() {
  printf 'CSGHUB_PROGRESS|%s|%s\n' "$1" "$2"
}

log() {
  printf '%s\n' "$*"
}

choose_registry() {
  local seen=""
  local registry=""
  local registries=()
  if [[ -n "${NPM_CONFIG_REGISTRY:-}" ]]; then
    registries+=("${NPM_CONFIG_REGISTRY}")
  fi
  registries+=("https://registry.npmmirror.com" "https://registry.npmjs.org")

  for registry in "${registries[@]}"; do
    [[ "$seen" == *"|$registry|"* ]] && continue
    seen="${seen}|${registry}|"
    printf 'INFO: checking npm registry %s\n' "${registry}" >&2
    if npm view "${PACKAGE}" version --registry "${registry}" >/dev/null 2>&1; then
      printf '%s\n' "${registry}"
      return 0
    fi
  done
  return 1
}

install_via_npm() {
  local registry=""
  local package_spec="${PACKAGE}"

  if ! command -v npm >/dev/null 2>&1; then
    log "INFO: npm not found; falling back to native Claude Code installer"
    return 1
  fi

  emit_progress 10 preflight
  registry="$(choose_registry || true)"
  if [[ -z "${registry}" ]]; then
    log "WARN: unable to reach a working npm registry for ${PACKAGE}; falling back to native installer"
    return 1
  fi

  if [[ "${TARGET}" != "latest" ]]; then
    package_spec="${PACKAGE}@${TARGET}"
  fi

  log "INFO: using npm registry ${registry}"
  emit_progress 25 resolving_latest
  emit_progress 55 installing
  npm install -g "${package_spec}" --registry "${registry}"

  emit_progress 90 verifying
  if ! command -v claude >/dev/null 2>&1; then
    log "ERROR: Claude Code command was not found on PATH after npm installation"
    exit 1
  fi

  claude --version || true
  emit_progress 100 complete
  log "INFO: Claude Code installation complete via npm"
  return 0
}

select_downloader() {
  if command -v curl >/dev/null 2>&1; then
    DOWNLOADER="curl"
    return 0
  fi
  if command -v wget >/dev/null 2>&1; then
    DOWNLOADER="wget"
    return 0
  fi
  log "ERROR: either curl or wget is required"
  exit 1
}

download_text() {
  local url="$1"
  if [[ "$DOWNLOADER" == "curl" ]]; then
    curl --connect-timeout 15 --max-time 60 --retry 3 --retry-delay 2 -fsSL "$url"
  else
    wget --tries=3 --timeout=20 -q -O - "$url"
  fi
}

download_file() {
  local url="$1"
  local output="$2"
  if [[ "$DOWNLOADER" == "curl" ]]; then
    curl --connect-timeout 15 --max-time 900 --retry 3 --retry-delay 2 -fsSL -o "$output" "$url"
  else
    wget --tries=3 --timeout=20 -q -O "$output" "$url"
  fi
}

get_checksum_from_manifest() {
  local json="$1"
  local platform="$2"
  json=$(echo "$json" | tr -d '\n\r\t' | sed 's/ \+/ /g')
  if [[ $json =~ \"$platform\"[^}]*\"checksum\"[[:space:]]*:[[:space:]]*\"([a-f0-9]{64})\" ]]; then
    echo "${BASH_REMATCH[1]}"
    return 0
  fi
  return 1
}

install_via_native_binary() {
  local os=""
  local arch=""
  local platform=""
  local version=""
  local manifest_json=""
  local checksum=""
  local binary_path=""
  local actual=""

  select_downloader

  emit_progress 10 detecting_platform
  case "$(uname -s)" in
    Darwin) os="darwin" ;;
    Linux) os="linux" ;;
    MINGW*|MSYS*|CYGWIN*)
      log "ERROR: use the PowerShell installer on Windows"
      exit 1
      ;;
    *)
      log "ERROR: unsupported operating system $(uname -s)"
      exit 1
      ;;
  esac

  case "$(uname -m)" in
    x86_64|amd64) arch="x64" ;;
    arm64|aarch64) arch="arm64" ;;
    *)
      log "ERROR: unsupported architecture $(uname -m)"
      exit 1
      ;;
  esac

  if [[ "$os" == "darwin" && "$arch" == "x64" ]]; then
    if [[ "$(sysctl -n sysctl.proc_translated 2>/dev/null || true)" == "1" ]]; then
      arch="arm64"
    fi
  fi

  if [[ "$os" == "linux" ]]; then
    if [[ -f /lib/libc.musl-x86_64.so.1 || -f /lib/libc.musl-aarch64.so.1 ]] || ldd /bin/ls 2>&1 | grep -q musl; then
      platform="linux-${arch}-musl"
    else
      platform="linux-${arch}"
    fi
  else
    platform="${os}-${arch}"
  fi

  mkdir -p "$DOWNLOAD_DIR"

  emit_progress 25 resolving_latest
  version="$(download_text "$GCS_BUCKET/latest")"
  manifest_json="$(download_text "$GCS_BUCKET/$version/manifest.json")"
  checksum="$(get_checksum_from_manifest "$manifest_json" "$platform" || true)"
  if [[ -z "$checksum" ]]; then
    log "ERROR: platform ${platform} not found in manifest"
    exit 1
  fi

  binary_path="$DOWNLOAD_DIR/claude-$version-$platform"
  emit_progress 55 downloading_binary
  download_file "$GCS_BUCKET/$version/$platform/claude" "$binary_path"

  emit_progress 75 verifying_checksum
  if [[ "$os" == "darwin" ]]; then
    actual="$(shasum -a 256 "$binary_path" | cut -d' ' -f1)"
  else
    actual="$(sha256sum "$binary_path" | cut -d' ' -f1)"
  fi

  if [[ "$actual" != "$checksum" ]]; then
    rm -f "$binary_path"
    log "ERROR: checksum verification failed"
    exit 1
  fi

  chmod +x "$binary_path"

  emit_progress 90 running_installer
  log "INFO: running Claude Code installer target ${TARGET}"
  "$binary_path" install "$TARGET"
  rm -f "$binary_path"

  emit_progress 100 complete
  if command -v claude >/dev/null 2>&1; then
    claude --version || true
  fi
  log "INFO: Claude Code installation complete"
}

if install_via_npm; then
  exit 0
fi

install_via_native_binary
