#!/bin/sh
set -eu

DIST_DIR="${1:-dist}"

if [ ! -d "$DIST_DIR" ]; then
    printf '%s\n' "dist directory not found: $DIST_DIR" >&2
    exit 1
fi

cd "$DIST_DIR"

if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 csghub-lite_* > checksums.txt
elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum csghub-lite_* > checksums.txt
else
    printf '%s\n' "shasum or sha256sum is required to generate checksums" >&2
    exit 1
fi
