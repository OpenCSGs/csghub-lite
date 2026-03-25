#!/usr/bin/env bash
set -euo pipefail

emit_progress() {
  printf 'CSGHUB_PROGRESS|%s|%s\n' "$1" "$2"
}

log() {
  printf '%s\n' "$*"
}

if ! command -v npm >/dev/null 2>&1; then
  log "ERROR: npm is required to uninstall OpenClaw."
  exit 1
fi

PACKAGE="openclaw"

emit_progress 5 preflight
emit_progress 35 removing_package
if npm ls -g --depth=0 "${PACKAGE}" >/dev/null 2>&1; then
  npm uninstall -g "${PACKAGE}"
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
