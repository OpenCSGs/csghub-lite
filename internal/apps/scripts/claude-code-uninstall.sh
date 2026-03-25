#!/usr/bin/env bash
set -euo pipefail

emit_progress() {
  printf 'CSGHUB_PROGRESS|%s|%s\n' "$1" "$2"
}

log() {
  printf '%s\n' "$*"
}

native_bin="${HOME}/.local/bin/claude"
native_share="${HOME}/.local/share/claude"
npm_package="@anthropic-ai/claude-code"

emit_progress 5 preflight

emit_progress 35 removing_binary
if [[ -e "${native_bin}" || -L "${native_bin}" ]]; then
  rm -f "${native_bin}"
  log "INFO: removed ${native_bin}"
else
  log "INFO: native Claude binary not found at ${native_bin}"
fi

emit_progress 65 removing_files
if [[ -d "${native_share}" ]]; then
  rm -rf "${native_share}"
  log "INFO: removed ${native_share}"
else
  log "INFO: native Claude runtime not found at ${native_share}"
fi

if command -v npm >/dev/null 2>&1; then
  if npm ls -g --depth=0 "${npm_package}" >/dev/null 2>&1; then
    emit_progress 80 removing_package
    npm uninstall -g "${npm_package}"
  fi
fi

emit_progress 90 verifying_uninstall
if command -v claude >/dev/null 2>&1; then
  log "ERROR: Claude Code binary is still available at $(command -v claude)"
  exit 1
fi

emit_progress 100 complete
log "INFO: Claude Code uninstallation complete"
