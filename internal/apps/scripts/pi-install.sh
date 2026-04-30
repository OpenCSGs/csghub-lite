#!/usr/bin/env bash
set -euo pipefail

emit_progress() {
  local progress="$1"
  local phase="$2"
  printf 'CSGHUB_PROGRESS|%s|%s\n' "${progress}" "${phase}"
}

log() {
  printf '%s\n' "$*"
}

emit_progress 5 checking_node
if ! command -v npm >/dev/null 2>&1; then
  log "ERROR: npm (Node.js) is required to install Pi Coding Agent."
  log "Install Node.js first: https://nodejs.org/"
  exit 1
fi

registry="${NPM_CONFIG_REGISTRY:-https://registry.npmmirror.com}"
package="${CSGHUB_LITE_PI_PACKAGE:-@mariozechner/pi-coding-agent@latest}"
install_root="${CSGHUB_LITE_PI_INSTALL_ROOT:-${HOME}/.local/share/pi-coding-agent}"
launcher_dir="${HOME}/.local/bin"
launcher_path="${launcher_dir}/pi"
actual_bin="${install_root}/bin/pi"

shell_profile_file() {
  local home_dir="${HOME:-}"
  if [[ -z "$home_dir" ]]; then
    return 1
  fi
  case "$(basename "${SHELL:-}")" in
    zsh)  printf '%s\n' "${home_dir}/.zprofile" ;;
    bash) printf '%s\n' "${home_dir}/.bash_profile" ;;
    *)    printf '%s\n' "${home_dir}/.profile" ;;
  esac
}

ensure_local_bin_on_path() {
  local profile=""
  local line='case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) export PATH="$HOME/.local/bin:$PATH" ;; esac'

  export PATH="${launcher_dir}:${PATH}"

  profile="$(shell_profile_file || true)"
  if [[ -z "$profile" ]]; then
    return 0
  fi
  mkdir -p "$(dirname "$profile")"
  [[ -f "$profile" ]] || : > "$profile"
  if ! grep -F "$line" "$profile" >/dev/null 2>&1; then
    printf '\n%s\n' "$line" >> "$profile"
  fi
}

emit_progress 30 installing_pi
log "INFO: installing Pi Coding Agent package ${package} to ${install_root}"
rm -rf "${install_root}"
mkdir -p "${install_root}" "${launcher_dir}"
npm install -g --prefix="${install_root}" --registry="${registry}" "${package}"

if [[ ! -f "${actual_bin}" ]]; then
  log "ERROR: Pi was installed but npm did not create ${actual_bin}."
  exit 1
fi

ln -sfn "${actual_bin}" "${launcher_path}"
ensure_local_bin_on_path
hash -r

emit_progress 85 verifying_pi
if ! command -v pi >/dev/null 2>&1 && [[ ! -x "${launcher_path}" ]]; then
  log "ERROR: Pi was installed but the pi command was not found on PATH."
  log "INFO: launcher was written to ${launcher_path}; add ${launcher_dir} to PATH and retry."
  exit 1
fi

"${launcher_path}" --version || true
emit_progress 100 complete
log "INFO: Pi Coding Agent installed successfully."
log "INFO: updated launcher ${launcher_path}"
