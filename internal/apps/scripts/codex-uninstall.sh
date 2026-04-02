#!/usr/bin/env bash
set -euo pipefail

emit_progress() {
  printf 'CSGHUB_PROGRESS|%s|%s\n' "$1" "$2"
}

log() {
  printf '%s\n' "$*"
}

PACKAGE="@openai/codex"
RUNTIME_ROOT="${HOME}/.local/share/codex"
launchers=(
  "${HOME}/.local/bin/codex"
  "${HOME}/bin/codex"
)

emit_progress 5 preflight
emit_progress 35 removing_package
if command -v npm >/dev/null 2>&1; then
  if npm ls -g --depth=0 "${PACKAGE}" >/dev/null 2>&1; then
    npm uninstall -g "${PACKAGE}"
  else
    log "INFO: npm package ${PACKAGE} is not installed"
  fi
else
  log "INFO: npm not found, skipping legacy npm package removal"
fi

emit_progress 55 removing_runtime
for launcher in "${launchers[@]}"; do
  rm -f "${launcher}"
done
rm -rf "${RUNTIME_ROOT}"
hash -r 2>/dev/null || true

emit_progress 80 verifying_uninstall
if command -v codex >/dev/null 2>&1; then
  log "ERROR: Codex binary is still available at $(command -v codex)"
  exit 1
fi

emit_progress 100 complete
log "INFO: Codex uninstallation complete"
