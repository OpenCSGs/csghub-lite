#!/usr/bin/env bash
# Rebuild and upload Ubuntu 22.04 CUDA x64 + arm64 tarballs.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
TAG="${1:-}"

chmod +x "${ROOT}"/*.sh
"${ROOT}/rebuild-upload-arm64.sh" ${TAG:+"${TAG}"}
"${ROOT}/rebuild-upload-x64.sh" ${TAG:+"${TAG}"}
