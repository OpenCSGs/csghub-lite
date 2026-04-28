#!/usr/bin/env bash
set -euo pipefail

emit_progress() {
  printf 'CSGHUB_PROGRESS|%s|%s\n' "$1" "$2"
}

log() {
  printf '%s\n' "$*"
}

PACKAGE="openclaw"

remove_stale_npm_temp_dirs() {
  local npm_root="$1"
  [[ -n "${npm_root}" && -d "${npm_root}" ]] || return 0

  local path=""
  shopt -s nullglob
  for path in "${npm_root}/.openclaw-"*; do
    [[ -d "${path}" ]] || continue
    rm -rf "${path}"
    log "INFO: removed stale npm temp dir ${path}"
  done
  shopt -u nullglob
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
  npm uninstall -g "${PACKAGE}" || return 1
}

stop_openclaw_processes() {
  if command -v pkill >/dev/null 2>&1; then
    pkill -f 'openclaw( |$)' >/dev/null 2>&1 || true
    pkill -f 'openclaw-onboard( |$)' >/dev/null 2>&1 || true
  fi
}

force_remove_openclaw_files() {
  local npm_root=""
  npm_root="$(npm root -g 2>/dev/null || true)"
  if [[ -n "${npm_root}" && -d "${npm_root}" ]]; then
    rm -rf "${npm_root}/${PACKAGE}" "${npm_root}/.openclaw-"* 2>/dev/null || true
  fi

  local npm_prefix=""
  npm_prefix="$(npm prefix -g 2>/dev/null || true)"
  local candidates=(
    "${HOME}/bin/openclaw"
    "${HOME}/.local/bin/openclaw"
    "/opt/homebrew/bin/openclaw"
    "/usr/local/bin/openclaw"
  )
  if [[ -n "${npm_prefix}" ]]; then
    candidates+=("${npm_prefix}/bin/openclaw")
  fi

  local resolved=""
  resolved="$(command -v openclaw 2>/dev/null || true)"
  if [[ -n "${resolved}" ]]; then
    candidates+=("${resolved}")
  fi

  local path=""
  for path in "${candidates[@]}"; do
    [[ -n "${path}" && -e "${path}" ]] || continue
    rm -f "${path}" 2>/dev/null || true
    log "INFO: removed ${path}"
  done
}

emit_progress 5 preflight
stop_openclaw_processes

emit_progress 35 removing_package
if command -v npm >/dev/null 2>&1; then
  if npm ls -g --depth=0 "${PACKAGE}" >/dev/null 2>&1; then
    if ! npm_uninstall_with_retry; then
      log "WARN: npm uninstall failed; continuing with forced file cleanup"
    fi
  else
    log "INFO: npm package ${PACKAGE} is not installed"
  fi
else
  log "WARN: npm is not available; continuing with forced file cleanup"
fi

emit_progress 65 cleaning_up
force_remove_openclaw_files

emit_progress 85 verifying_uninstall
if command -v openclaw >/dev/null 2>&1; then
  log "WARN: OpenClaw binary is still available at $(command -v openclaw)"
fi

emit_progress 100 complete
log "INFO: OpenClaw forced uninstallation complete"
