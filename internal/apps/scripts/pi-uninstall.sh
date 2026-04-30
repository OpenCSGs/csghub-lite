#!/usr/bin/env bash
set -euo pipefail

emit_progress() {
  local progress="$1"
  local phase="$2"
  printf 'CSGHUB_PROGRESS|%s|%s\n' "${progress}" "${phase}"
}

emit_progress 20 uninstalling_pi
install_root="${CSGHUB_LITE_PI_INSTALL_ROOT:-${HOME}/.local/share/pi-coding-agent}"
launcher_path="${HOME}/.local/bin/pi"

rm -rf "${install_root}"
rm -f "${launcher_path}"

emit_progress 100 complete
printf '%s\n' "INFO: Pi Coding Agent uninstall complete."
