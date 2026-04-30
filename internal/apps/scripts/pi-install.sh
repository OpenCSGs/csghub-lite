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

install_fd_fallback() {
  local path="${launcher_dir}/fd"
  cat > "${path}" <<'PY'
#!/usr/bin/env python3
import os
import sys

if "--version" in sys.argv[1:]:
    print("fd fallback for csghub-lite")
    raise SystemExit(0)

args = [arg for arg in sys.argv[1:] if not arg.startswith("-")]
pattern = args[0] if args else ""
roots = args[1:] if len(args) > 1 else ["."]
for root in roots:
    for current, dirs, files in os.walk(root):
        dirs[:] = [d for d in dirs if d not in {".git", "node_modules", ".pi", ".csghub-lite"}]
        for name in dirs + files:
            if not pattern or pattern.lower() in name.lower():
                print(os.path.join(current, name))
PY
  chmod +x "${path}"
}

install_rg_fallback() {
  local path="${launcher_dir}/rg"
  cat > "${path}" <<'PY'
#!/usr/bin/env python3
import os
import re
import sys

args = sys.argv[1:]
if "--version" in args:
    print("ripgrep fallback for csghub-lite")
    raise SystemExit(0)

ignore_case = "-i" in args or "--ignore-case" in args
line_numbers = "-n" in args or "--line-number" in args
clean = []
skip_next = False
for arg in args:
    if skip_next:
        skip_next = False
        continue
    if arg in {"-e", "--regexp", "-g", "--glob", "--type", "-t"}:
        skip_next = arg not in {"-e", "--regexp"}
        if arg in {"-e", "--regexp"}:
            continue
    if arg.startswith("-"):
        continue
    clean.append(arg)

if not clean:
    raise SystemExit(1)

pattern = clean[0]
roots = clean[1:] or ["."]
flags = re.IGNORECASE if ignore_case else 0
try:
    regex = re.compile(pattern, flags)
except re.error:
    regex = re.compile(re.escape(pattern), flags)

for root in roots:
    if os.path.isfile(root):
        candidates = [(os.path.dirname(root) or ".", [], [os.path.basename(root)])]
    else:
        candidates = os.walk(root)
    for current, dirs, files in candidates:
        dirs[:] = [d for d in dirs if d not in {".git", "node_modules", ".pi", ".csghub-lite"}]
        for name in files:
            path = os.path.join(current, name)
            try:
                with open(path, "r", encoding="utf-8", errors="ignore") as handle:
                    for number, line in enumerate(handle, 1):
                        if regex.search(line):
                            line = line.rstrip("\n")
                            if line_numbers:
                                print(f"{path}:{number}:{line}")
                            else:
                                print(f"{path}:{line}")
            except OSError:
                pass
PY
  chmod +x "${path}"
}

ensure_pi_search_tools() {
  emit_progress 70 ensuring_search_tools

  if command -v fd >/dev/null 2>&1; then
    log "INFO: using existing fd: $(command -v fd)"
  elif command -v fdfind >/dev/null 2>&1; then
    cat > "${launcher_dir}/fd" <<EOF
#!/usr/bin/env bash
exec "$(command -v fdfind)" "\$@"
EOF
    chmod +x "${launcher_dir}/fd"
    log "INFO: linked fd to existing fdfind"
  else
    install_fd_fallback
    log "INFO: installed fd fallback launcher: ${launcher_dir}/fd"
  fi

  if command -v rg >/dev/null 2>&1; then
    log "INFO: using existing rg: $(command -v rg)"
  else
    install_rg_fallback
    log "INFO: installed rg fallback launcher: ${launcher_dir}/rg"
  fi

  hash -r
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
ensure_pi_search_tools

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
