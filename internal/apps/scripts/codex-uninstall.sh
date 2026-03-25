#!/usr/bin/env bash
set -euo pipefail

emit_progress() {
  printf 'CSGHUB_PROGRESS|%s|%s\n' "$1" "$2"
}

log() {
  printf '%s\n' "$*"
}

if ! command -v npm >/dev/null 2>&1; then
  log "ERROR: npm is required to uninstall Codex."
  exit 1
fi

PACKAGE="@openai/codex"

emit_progress 5 preflight
emit_progress 35 removing_package
if npm ls -g --depth=0 "${PACKAGE}" >/dev/null 2>&1; then
  npm uninstall -g "${PACKAGE}"
else
  log "INFO: npm package ${PACKAGE} is not installed"
fi

emit_progress 80 verifying_uninstall
if command -v codex >/dev/null 2>&1; then
  log "ERROR: Codex binary is still available at $(command -v codex)"
  exit 1
fi

emit_progress 100 complete
log "INFO: Codex uninstallation complete"
