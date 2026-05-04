#!/usr/bin/env bash
set -euo pipefail

emit_progress() {
  printf 'CSGHUB_PROGRESS|%s|%s\n' "$1" "$2"
}

log() {
  printf '%s\n' "$*"
}

RUNTIME_ROOT="${HOME}/.local/share/csgclaw"
CONFIG_ROOT="${HOME}/.csgclaw"
launchers=(
  "${HOME}/.local/bin/csgclaw"
  "${HOME}/bin/csgclaw"
)

stop_csgclaw_processes() {
  local binary="${HOME}/.local/bin/csgclaw"

  # Stop serve daemon if running
  if [[ -f "${binary}" ]] && command -v "${binary}" >/dev/null 2>&1; then
    local pid_file="${HOME}/.csghub-lite/apps/logs/csgclaw.pid"
    if [[ -f "${pid_file}" ]]; then
      "${binary}" stop --pid "${pid_file}" >/dev/null 2>&1 || true
    fi
  fi

  # Stop manager agent
  if [[ -f "${binary}" ]] && command -v "${binary}" >/dev/null 2>&1; then
    "${binary}" agent stop u-manager >/dev/null 2>&1 || true
  fi

  # Kill any remaining csgclaw serve processes
  if command -v pkill >/dev/null 2>&1; then
    pkill -f 'csgclaw( serve| _serve)' >/dev/null 2>&1 || true
  fi

  # Kill boxlite-shim processes spawned by csgclaw manager
  if command -v pkill >/dev/null 2>&1; then
    pkill -f 'boxlite-shim' >/dev/null 2>&1 || true
  fi

  # Wait briefly for processes to terminate
  sleep 1
}

emit_progress 5 preflight

emit_progress 20 stopping_services
stop_csgclaw_processes

emit_progress 35 removing_runtime
for launcher in "${launchers[@]}"; do
  rm -f "${launcher}"
done
rm -rf "${RUNTIME_ROOT}"
rm -rf "${CONFIG_ROOT}"
hash -r 2>/dev/null || true

emit_progress 80 verifying_uninstall
if command -v csgclaw >/dev/null 2>&1; then
  log "ERROR: CSGClaw binary is still available at $(command -v csgclaw)"
  exit 1
fi

emit_progress 100 complete
log "INFO: CSGClaw uninstallation complete"
