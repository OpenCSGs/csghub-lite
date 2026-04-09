#!/bin/sh
set -eu

REPO_ROOT="$(CDPATH='' cd "$(dirname "$0")/.." && pwd)"
LLAMA_CPP_REPO="ggml-org/llama.cpp"
MODE="sync"
TAG="${LLAMA_CPP_CONVERTER_TAG:-}"

TARGET_SCRIPT="${REPO_ROOT}/internal/convert/data/convert_hf_to_gguf.py"
BUNDLED_GO="${REPO_ROOT}/internal/convert/bundled_converter.go"
README_FILE="${REPO_ROOT}/internal/convert/data/README.md"

info() { printf "\033[0;32m[INFO]\033[0m %s\n" "$1"; }
die()  { printf "\033[0;31m[ERROR]\033[0m %s\n" "$1" >&2; exit 1; }

usage() {
    cat <<'EOF'
Usage: scripts/sync-llama-converter.sh [options]

Check or sync the bundled llama.cpp convert_hf_to_gguf.py copy.

Options:
  --check               Fail if the bundled converter is older than upstream
  --tag TAG             Sync/check against a specific llama.cpp release tag
  -h, --help            Show this help

Environment variables:
  LLAMA_CPP_CONVERTER_TAG   Optional llama.cpp release tag to use when --tag is omitted

Notes:
  For GitHub access in this environment, run `source ~/.myshrc` before this script.
EOF
}

need_tool() {
    command -v "$1" >/dev/null 2>&1 || die "$1 not found on PATH"
}

extract_current_tag() {
    sed -n 's/^const BundledConverterLLamacppRef = "\(.*\)"$/\1/p' "${BUNDLED_GO}"
}

extract_current_revision() {
    sed -n 's/^const bundledConverterRevision = \([0-9][0-9]*\)$/\1/p' "${BUNDLED_GO}"
}

resolve_tag() {
    if [ -n "${TAG}" ]; then
        printf "%s\n" "${TAG}"
        return
    fi
    gh release view --repo "${LLAMA_CPP_REPO}" --json tagName --jq '.tagName'
}

while [ $# -gt 0 ]; do
    case "$1" in
        --check) MODE="check"; shift ;;
        --tag)   TAG="$2"; shift 2 ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            die "Unknown option: $1"
            ;;
    esac
done

[ -f "${TARGET_SCRIPT}" ] || die "converter script not found: ${TARGET_SCRIPT}"
[ -f "${BUNDLED_GO}" ] || die "bundled metadata file not found: ${BUNDLED_GO}"
[ -f "${README_FILE}" ] || die "converter README not found: ${README_FILE}"

need_tool curl
need_tool gh

UPSTREAM_TAG="$(resolve_tag)"
[ -n "${UPSTREAM_TAG}" ] || die "failed to resolve llama.cpp release tag"

RAW_URL="https://raw.githubusercontent.com/ggml-org/llama.cpp/${UPSTREAM_TAG}/convert_hf_to_gguf.py"
TMP_SCRIPT="$(mktemp "${TMPDIR:-/tmp}/llama-converter.XXXXXX")"
trap 'rm -f "${TMP_SCRIPT}"' EXIT INT TERM

info "Fetching ${RAW_URL}"
curl -fsSL -o "${TMP_SCRIPT}" "${RAW_URL}"

CURRENT_TAG="$(extract_current_tag)"
CURRENT_REVISION="$(extract_current_revision)"
[ -n "${CURRENT_TAG}" ] || die "failed to parse current bundled converter tag"
[ -n "${CURRENT_REVISION}" ] || die "failed to parse current bundled converter revision"

SAME_SCRIPT=1
if ! cmp -s "${TMP_SCRIPT}" "${TARGET_SCRIPT}"; then
    SAME_SCRIPT=0
fi

if [ "${MODE}" = "check" ]; then
    if [ "${SAME_SCRIPT}" -eq 1 ] && [ "${CURRENT_TAG}" = "${UPSTREAM_TAG}" ]; then
        info "Bundled converter already matches ${UPSTREAM_TAG}"
        exit 0
    fi
    die "Bundled converter is stale (repo: ${CURRENT_TAG}, upstream: ${UPSTREAM_TAG}). Run ./scripts/sync-llama-converter.sh --tag ${UPSTREAM_TAG}, commit the result, retag, and rerun release."
fi

NEW_REVISION="${CURRENT_REVISION}"
if [ "${SAME_SCRIPT}" -eq 0 ]; then
    cp "${TMP_SCRIPT}" "${TARGET_SCRIPT}"
    NEW_REVISION=$((CURRENT_REVISION + 1))
    info "Updated bundled converter script content"
else
    info "Bundled converter script content already matches ${UPSTREAM_TAG}"
fi

need_tool python3
python3 - "${BUNDLED_GO}" "${README_FILE}" "${UPSTREAM_TAG}" "${NEW_REVISION}" <<'PY'
from pathlib import Path
import re
import sys

go_path = Path(sys.argv[1])
readme_path = Path(sys.argv[2])
tag = sys.argv[3]
revision = sys.argv[4]

go_text = go_path.read_text(encoding="utf-8")
go_text, n1 = re.subn(
    r"const bundledConverterRevision = \d+",
    f"const bundledConverterRevision = {revision}",
    go_text,
    count=1,
)
go_text, n2 = re.subn(
    r'const BundledConverterLLamacppRef = "[^"]+"',
    f'const BundledConverterLLamacppRef = "{tag}"',
    go_text,
    count=1,
)
if n1 != 1 or n2 != 1:
    raise SystemExit("failed to patch bundled_converter.go")
go_path.write_text(go_text, encoding="utf-8")

readme_text = readme_path.read_text(encoding="utf-8")
readme_text, n3 = re.subn(
    r"\| Upstream tag \| `[^`]+` \(see `BundledConverterLLamacppRef` in `bundled_converter\.go`\) \|",
    f"| Upstream tag | `{tag}` (see `BundledConverterLLamacppRef` in `bundled_converter.go`) |",
    readme_text,
    count=1,
)
readme_text, n4 = re.subn(
    r"\./scripts/sync-llama-converter\.sh --tag [^\s`]+",
    f"./scripts/sync-llama-converter.sh --tag {tag}",
    readme_text,
    count=1,
)
if n3 != 1 or n4 != 1:
    raise SystemExit("failed to patch internal/convert/data/README.md")
readme_path.write_text(readme_text, encoding="utf-8")
PY

info "Synced bundled converter to ${UPSTREAM_TAG} (revision ${NEW_REVISION})"
