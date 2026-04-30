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
fd_path="${HOME}/.local/bin/fd"
rg_path="${HOME}/.local/bin/rg"

remove_generated_launcher() {
  local path="$1"
  if [[ -f "$path" ]] && grep -F "csghub-lite" "$path" >/dev/null 2>&1; then
    rm -f "$path"
  fi
}

rm -rf "${install_root}"
rm -f "${launcher_path}"
remove_generated_launcher "${fd_path}"
remove_generated_launcher "${rg_path}"

emit_progress 100 complete
printf '%s\n' "INFO: Pi Coding Agent uninstall complete."
