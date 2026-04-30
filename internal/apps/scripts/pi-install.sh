#!/usr/bin/env bash
set -euo pipefail

emit_progress() {
  local progress="$1"
  local phase="$2"
  printf '[progress] %s %s\n' "${progress}" "${phase}"
}

log() {
  printf '%s\n' "$*"
}

emit_progress 5 checking_node
if ! command -v npm >/dev/null 2>&1; then
  log "ERROR: npm (Node.js) is required to install Pi Coding Agent."
  log "Install Node.js first: https://nodejs.org/"
  exit 1
fi

registry="${NPM_CONFIG_REGISTRY:-https://registry.npmmirror.com}"
package="${CSGHUB_LITE_PI_PACKAGE:-@mariozechner/pi-coding-agent@latest}"

emit_progress 30 installing_pi
log "INFO: installing Pi Coding Agent package ${package}"
npm install -g --registry="${registry}" "${package}"

emit_progress 85 verifying_pi
if ! command -v pi >/dev/null 2>&1; then
  log "ERROR: Pi was installed but the pi command was not found on PATH."
  exit 1
fi

pi --version || true
emit_progress 100 complete
log "INFO: Pi Coding Agent installed successfully."
