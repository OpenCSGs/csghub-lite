#!/bin/sh
# GoReleaser archives use OS_ARCH (underscore). GitLab generic packages + install.sh
# expect OS-ARCH (hyphen). Run from repo root after dist/ is populated.
set -eu

VERSION="${1:?usage: $0 <version_without_v>}"
ROOT="$(CDPATH='' cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/dist"

mv "csghub-lite_${VERSION}_darwin_arm64.tar.gz" "csghub-lite_${VERSION}_darwin-arm64.tar.gz"
mv "csghub-lite_${VERSION}_darwin_amd64.tar.gz" "csghub-lite_${VERSION}_darwin-amd64.tar.gz"
mv "csghub-lite_${VERSION}_linux_arm64.tar.gz" "csghub-lite_${VERSION}_linux-arm64.tar.gz"
mv "csghub-lite_${VERSION}_linux_amd64.tar.gz" "csghub-lite_${VERSION}_linux-amd64.tar.gz"
mv "csghub-lite_${VERSION}_windows_amd64.zip" "csghub-lite_${VERSION}_windows-amd64.zip"
