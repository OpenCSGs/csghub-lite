#!/usr/bin/env bash
set -euo pipefail

emit_progress() {
  local progress="$1"
  local phase="$2"
  printf '[progress] %s %s\n' "${progress}" "${phase}"
}

emit_progress 20 uninstalling_pi
if command -v npm >/dev/null 2>&1; then
  npm uninstall -g @mariozechner/pi-coding-agent || true
fi

emit_progress 100 complete
printf '%s\n' "INFO: Pi Coding Agent uninstall complete."
