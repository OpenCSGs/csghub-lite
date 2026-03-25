#!/usr/bin/env bash
set -euo pipefail

PACKAGE="openclaw@latest"
REQUIRED_NODE_VERSION="22.16.0"

emit_progress() {
  printf 'CSGHUB_PROGRESS|%s|%s\n' "$1" "$2"
}

log() {
  printf '%s\n' "$*"
}

version_ge() {
  python3 - "$1" "$2" <<'PY'
import sys
from itertools import zip_longest

def parse(value):
    value = value.lstrip("v").split("-", 1)[0]
    return [int(part) for part in value.split(".") if part.isdigit()]

left = parse(sys.argv[1])
right = parse(sys.argv[2])
for a, b in zip_longest(left, right, fillvalue=0):
    if a > b:
        sys.exit(0)
    if a < b:
        sys.exit(1)
sys.exit(0)
PY
}

node_version() {
  node -v 2>/dev/null || true
}

has_required_node() {
  local current
  current="$(node_version)"
  [[ -n "$current" ]] && version_ge "$current" "$REQUIRED_NODE_VERSION"
}

resolve_brew() {
  if command -v brew >/dev/null 2>&1; then
    command -v brew
    return 0
  fi
  if [[ -x "/opt/homebrew/bin/brew" ]]; then
    printf '%s\n' "/opt/homebrew/bin/brew"
    return 0
  fi
  if [[ -x "/usr/local/bin/brew" ]]; then
    printf '%s\n' "/usr/local/bin/brew"
    return 0
  fi
  return 1
}

load_nvm() {
  if [[ -s "${NVM_DIR:-$HOME/.nvm}/nvm.sh" ]]; then
    # shellcheck source=/dev/null
    . "${NVM_DIR:-$HOME/.nvm}/nvm.sh"
    return 0
  fi
  return 1
}

ensure_node() {
  emit_progress 15 ensuring_node
  if has_required_node; then
    log "INFO: using Node.js $(node_version)"
    return 0
  fi

  if load_nvm && command -v nvm >/dev/null 2>&1; then
    log "INFO: installing Node.js 22 with nvm"
    nvm install 22 >/dev/null
    nvm use 22 >/dev/null
    hash -r
  fi
  if has_required_node; then
    log "INFO: using Node.js $(node_version)"
    return 0
  fi

  local brew_bin=""
  brew_bin="$(resolve_brew || true)"
  if [[ -n "$brew_bin" ]]; then
    log "INFO: installing Node.js 22 with Homebrew"
    "$brew_bin" install node@22
    "$brew_bin" link node@22 --overwrite --force >/dev/null 2>&1 || true
    export PATH="$("$brew_bin" --prefix node@22)/bin:$PATH"
    hash -r
  fi
  if has_required_node; then
    log "INFO: using Node.js $(node_version)"
    return 0
  fi

  if [[ "$(uname -s)" == "Linux" ]]; then
    if command -v apt-get >/dev/null 2>&1; then
      log "INFO: installing Node.js 22 from NodeSource"
      if [[ "$(id -u)" -eq 0 ]]; then
        curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
        apt-get install -y nodejs
      elif command -v sudo >/dev/null 2>&1; then
        curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
        sudo apt-get install -y nodejs
      fi
    elif command -v dnf >/dev/null 2>&1; then
      log "INFO: installing Node.js 22 with dnf"
      if [[ "$(id -u)" -eq 0 ]]; then
        dnf install -y nodejs
      elif command -v sudo >/dev/null 2>&1; then
        sudo dnf install -y nodejs
      fi
    elif command -v yum >/dev/null 2>&1; then
      log "INFO: installing Node.js 22 with yum"
      if [[ "$(id -u)" -eq 0 ]]; then
        yum install -y nodejs
      elif command -v sudo >/dev/null 2>&1; then
        sudo yum install -y nodejs
      fi
    fi
    hash -r
  fi

  if ! has_required_node; then
    log "ERROR: OpenClaw requires Node.js >= ${REQUIRED_NODE_VERSION}."
    log "ERROR: install Node.js 22+ manually, or provide Homebrew/nvm/apt access, then retry."
    exit 1
  fi

  log "INFO: using Node.js $(node_version)"
}

ensure_npm() {
  if command -v npm >/dev/null 2>&1; then
    return 0
  fi
  log "ERROR: npm is required to install OpenClaw."
  exit 1
}

create_launcher() {
  local actual_bin=""
  local node_bin=""
  local launcher_dir=""
  local launcher_path=""

  actual_bin="$(command -v openclaw || true)"
  node_bin="$(dirname "$(command -v node)")"
  if [[ -z "${actual_bin}" || -z "${node_bin}" ]]; then
    return 0
  fi

  if [[ ":${PATH}:" == *":${HOME}/bin:"* ]]; then
    launcher_dir="${HOME}/bin"
  else
    launcher_dir="${HOME}/.local/bin"
  fi
  launcher_path="${launcher_dir}/openclaw"
  mkdir -p "${launcher_dir}"
  if [[ "${actual_bin}" == "${launcher_path}" ]]; then
    return 0
  fi

  cat > "${launcher_path}" <<EOF
#!/usr/bin/env bash
export PATH="${node_bin}:\$PATH"
exec "${actual_bin}" "\$@"
EOF
  chmod +x "${launcher_path}"
  hash -r
  log "INFO: created launcher: ${launcher_path}"
}

choose_registry() {
  local seen=""
  local registry
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

emit_progress 5 preflight
ensure_node
ensure_npm

registry="$(choose_registry || true)"
if [[ -z "${registry}" ]]; then
  log "ERROR: unable to reach a working npm registry for ${PACKAGE}"
  exit 1
fi

log "INFO: using npm registry ${registry}"
emit_progress 35 installing
npm install -g "${PACKAGE}" --registry "${registry}"
create_launcher

emit_progress 80 verifying
if command -v openclaw >/dev/null 2>&1; then
  log "INFO: installed binary: $(command -v openclaw)"
  openclaw --version || true
  log "INFO: run 'openclaw onboard --install-daemon' to finish interactive onboarding."
fi

emit_progress 100 complete
log "INFO: OpenClaw installation complete"
