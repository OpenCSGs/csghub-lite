#!/usr/bin/env bash
set -euo pipefail

emit_progress() {
  printf 'CSGHUB_PROGRESS|%s|%s\n' "$1" "$2"
}

log() {
  printf '%s\n' "$*"
}

if ! command -v npm >/dev/null 2>&1; then
  log "ERROR: npm is required to install Codex."
  exit 1
fi

PACKAGE="@openai/codex@latest"
registries=()
if [[ -n "${NPM_CONFIG_REGISTRY:-}" ]]; then
  registries+=("${NPM_CONFIG_REGISTRY}")
fi
registries+=("https://registry.npmmirror.com" "https://registry.npmjs.org")

choose_registry() {
  local seen=""
  local registry
  for registry in "${registries[@]}"; do
    [[ "$seen" == *"|$registry|"* ]] && continue
    seen="${seen}|${registry}|"
    printf 'INFO: checking npm registry %s\n' "${registry}" >&2
    if npm view "${PACKAGE}" version --registry "${registry}" >/dev/null 2>&1; then
      printf '%s\n' "${registry}"
      return 0
    fi
  done
  return 1
}

emit_progress 5 preflight
registry="$(choose_registry || true)"
if [[ -z "${registry}" ]]; then
  log "ERROR: unable to reach a working npm registry for ${PACKAGE}"
  exit 1
fi

log "INFO: using npm registry ${registry}"
emit_progress 30 installing
npm install -g "${PACKAGE}" --registry "${registry}"

emit_progress 80 verifying
if command -v codex >/dev/null 2>&1; then
  log "INFO: installed binary: $(command -v codex)"
  codex --version || true
fi

emit_progress 100 complete
log "INFO: Codex installation complete"
