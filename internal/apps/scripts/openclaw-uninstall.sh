#!/usr/bin/env bash
set -euo pipefail

emit_progress() {
  printf 'CSGHUB_PROGRESS|%s|%s\n' "$1" "$2"
}

log() {
  printf '%s\n' "$*"
}

remove_stale_npm_temp_dirs() {
  local npm_root="$1"
  [[ -n "${npm_root}" && -d "${npm_root}" ]] || return 0

  local stale_dirs=()
  local path=""
  shopt -s nullglob
  for path in "${npm_root}/.openclaw-"*; do
    [[ -d "${path}" ]] || continue
    stale_dirs+=("${path}")
  done
  shopt -u nullglob

  for path in "${stale_dirs[@]}"; do
    rm -rf "${path}"
    log "INFO: removed stale npm temp dir ${path}"
  done
}

npm_uninstall_with_retry() {
  local npm_root=""
  npm_root="$(npm root -g 2>/dev/null || true)"

  remove_stale_npm_temp_dirs "${npm_root}"
  if npm uninstall -g "${PACKAGE}"; then
    return 0
  fi

  log "WARN: npm uninstall failed once, retrying after cleaning stale temp dirs"
  remove_stale_npm_temp_dirs "${npm_root}"
  npm uninstall -g "${PACKAGE}"
}

if ! command -v npm >/dev/null 2>&1; then
  log "ERROR: npm is required to uninstall OpenClaw."
  exit 1
fi

PACKAGE="openclaw"

emit_progress 5 preflight
emit_progress 35 removing_package
if npm ls -g --depth=0 "${PACKAGE}" >/dev/null 2>&1; then
  npm_uninstall_with_retry
else
  log "INFO: npm package ${PACKAGE} is not installed"
fi

emit_progress 65 cleaning_up
for launcher in "${HOME}/bin/openclaw" "${HOME}/.local/bin/openclaw"; do
  if [[ -f "${launcher}" ]]; then
    rm -f "${launcher}"
    log "INFO: removed launcher ${launcher}"
  fi
done

emit_progress 85 verifying_uninstall
if command -v openclaw >/dev/null 2>&1; then
  log "ERROR: OpenClaw binary is still available at $(command -v openclaw)"
  exit 1
fi

emit_progress 100 complete
log "INFO: OpenClaw uninstallation complete"
