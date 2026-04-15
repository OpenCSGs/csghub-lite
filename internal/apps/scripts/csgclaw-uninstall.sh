#!/usr/bin/env bash
set -euo pipefail

emit_progress() {
  printf 'CSGHUB_PROGRESS|%s|%s\n' "$1" "$2"
}

log() {
  printf '%s\n' "$*"
}

RUNTIME_ROOT="${HOME}/.local/share/csgclaw"
launchers=(
  "${HOME}/.local/bin/csgclaw"
  "${HOME}/bin/csgclaw"
)

emit_progress 5 preflight

emit_progress 35 removing_runtime
for launcher in "${launchers[@]}"; do
  rm -f "${launcher}"
done
rm -rf "${RUNTIME_ROOT}"
hash -r 2>/dev/null || true

emit_progress 80 verifying_uninstall
if command -v csgclaw >/dev/null 2>&1; then
  log "ERROR: CSGClaw binary is still available at $(command -v csgclaw)"
  exit 1
fi

emit_progress 100 complete
log "INFO: CSGClaw uninstallation complete"
