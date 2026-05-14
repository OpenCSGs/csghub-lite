#!/usr/bin/env bash
set -euo pipefail

binary_name="${CSGHUB_LITE_BINARY_NAME:-csghub-lite}"
install_url="${CSGHUB_LITE_INSTALL_URL:-https://hub.opencsg.com/csghub-lite/install.sh}"
install_dir="${INSTALL_DIR:-/root/.csghub-lite/bin}"
llama_install_dir="${CSGHUB_LITE_LLAMA_SERVER_INSTALL_DIR:-${install_dir}}"
install_policy="${CSGHUB_LITE_INSTALL_POLICY:-if-missing}"
require_llama_server="${CSGHUB_LITE_REQUIRE_LLAMA_SERVER:-1}"

mkdir -p "${install_dir}" "${llama_install_dir}"
export PATH="${install_dir}:${llama_install_dir}:${PATH}"

has_csghub_lite() {
    command -v "${binary_name}" >/dev/null 2>&1
}

has_llama_server() {
    if [ -n "${CSGHUB_LITE_LLAMA_SERVER:-}" ] && [ -x "${CSGHUB_LITE_LLAMA_SERVER}" ]; then
        return 0
    fi
    command -v llama-server >/dev/null 2>&1
}

needs_llama_server() {
    [ "${CSGHUB_LITE_AUTO_INSTALL_LLAMA_SERVER:-1}" != "0" ] && [ "${require_llama_server}" != "0" ]
}

installed_version_matches() {
    local requested="${CSGHUB_LITE_VERSION:-}"
    local requested_without_v="${requested#v}"
    local current=""

    [ -n "${requested}" ] || return 1
    current="$("${binary_name}" --version 2>/dev/null || true)"
    [ -n "${current}" ] || return 1

    case "${current}" in
        *"${requested}"*|*"${requested_without_v}"*) return 0 ;;
        *) return 1 ;;
    esac
}

needs_install() {
    if [ "${CSGHUB_LITE_INSTALL_ALWAYS:-0}" = "1" ]; then
        return 0
    fi

    case "${install_policy}" in
        always)
            return 0
            ;;
        if-missing)
            if ! has_csghub_lite || (needs_llama_server && ! has_llama_server); then
                return 0
            fi
            return 1
            ;;
        if-version-mismatch)
            if ! has_csghub_lite || (needs_llama_server && ! has_llama_server); then
                return 0
            fi
            if [ -n "${CSGHUB_LITE_VERSION:-}" ] && ! installed_version_matches; then
                return 0
            fi
            return 1
            ;;
        *)
            echo "Unsupported CSGHUB_LITE_INSTALL_POLICY=${install_policy}" >&2
            echo "Expected one of: if-missing, if-version-mismatch, always" >&2
            exit 64
            ;;
    esac
}

install_csghub_lite() {
    local tmp_script="/tmp/csghub-lite-install.sh"

    echo "Installing csghub-lite${CSGHUB_LITE_VERSION:+ ${CSGHUB_LITE_VERSION}} (policy: ${install_policy})..."
    curl -fsSL "${install_url}" -o "${tmp_script}"
    CSGHUB_LITE_FORCE="${CSGHUB_LITE_FORCE:-1}" \
        CSGHUB_LITE_AUTO_INSTALL_LLAMA_SERVER="${CSGHUB_LITE_AUTO_INSTALL_LLAMA_SERVER:-1}" \
        INSTALL_DIR="${install_dir}" \
        CSGHUB_LITE_LLAMA_SERVER_INSTALL_DIR="${llama_install_dir}" \
        sh "${tmp_script}"
    rm -f "${tmp_script}"
    hash -r

    # The public installer starts a background service for desktop installs.
    # Containers run the requested command in the foreground instead.
    "${binary_name}" stop-service >/dev/null 2>&1 || true
}

if needs_install; then
    install_csghub_lite
else
    echo "csghub-lite runtime already installed; set CSGHUB_LITE_INSTALL_ALWAYS=1 to reinstall on startup."
fi

if ! has_csghub_lite; then
    echo "csghub-lite was not found after installation. Check CSGHUB_LITE_INSTALL_URL, CSGHUB_LITE_VERSION, and network access." >&2
    exit 127
fi

if needs_llama_server && ! has_llama_server; then
    echo "llama-server was not found after installation. Local inference will not work." >&2
    echo "Set CSGHUB_LITE_REQUIRE_LLAMA_SERVER=0 to run without a local inference engine." >&2
    exit 127
fi

if [ "$#" -eq 0 ]; then
    set -- serve --listen 0.0.0.0:11435
fi

case "$1" in
    serve|run|chat|pull|list|show|ps|stop|stop-service|restart|restart-service|restart-server|reload|rm|login|search|config|upgrade|apps|launch|--help|--version|-*)
        exec "${binary_name}" "$@"
        ;;
    csghub-lite)
        exec "$@"
        ;;
    *)
        exec "$@"
        ;;
esac
