#!/bin/sh
# Upload csghub-lite release assets to GitHub and GitLab
# Usage: scripts/push.sh [--gitlab-token TOKEN] [--tag TAG]
set -eu

BINARY_NAME="csghub-lite"
GITHUB_REPO="OpenCSGs/csghub-lite"
GITLAB_HOST="https://git-devops.opencsg.com"
GITLAB_REMOTE_URL="https://git-devops.opencsg.com/opensource/csghub-lite.git"
GITLAB_PROJECT_ID="392"
GITLAB_PROJECT_PATH="opensource/csghub-lite"

info() { printf "\033[0;32m[INFO]\033[0m %s\n" "$1"; }
warn() { printf "\033[1;33m[WARN]\033[0m %s\n" "$1"; }
die()  { printf "\033[0;31m[ERROR]\033[0m %s\n" "$1" >&2; exit 1; }

usage() {
    cat <<'EOF'
Usage: scripts/push.sh [options]

Upload csghub-lite release assets to GitHub and GitLab.

Options:
  --gitlab-token TOKEN   GitLab personal access token (or set GITLAB_TOKEN env)
  --tag TAG              Release tag (default: auto-detect from git)
  --skip-github          Skip GitHub upload
  --skip-gitlab          Skip GitLab upload
  --skip-build           Skip make package (reuse existing dist/)
  -h, --help             Show this help

Environment variables:
  GITLAB_TOKEN           GitLab PAT with api scope
EOF
}

GITLAB_TOKEN="${GITLAB_TOKEN:-}"
TAG=""
SKIP_GITHUB=0
SKIP_GITLAB=0
SKIP_BUILD=0

while [ $# -gt 0 ]; do
    case "$1" in
        --gitlab-token) GITLAB_TOKEN="$2"; shift 2 ;;
        --tag)          TAG="$2"; shift 2 ;;
        --skip-github)  SKIP_GITHUB=1; shift ;;
        --skip-gitlab)  SKIP_GITLAB=1; shift ;;
        --skip-build)   SKIP_BUILD=1; shift ;;
        -h|--help)      usage; exit 0 ;;
        *)              die "Unknown option: $1" ;;
    esac
done

if [ -z "$TAG" ]; then
    TAG="$(git describe --tags --exact-match 2>/dev/null || true)"
fi
if [ -z "$TAG" ]; then
    die "Current commit is not tagged. Use --tag TAG or: git tag vX.Y.Z"
fi

VERSION="${TAG#v}"
info "Release tag: ${TAG} (version: ${VERSION})"

# ---- Build & package ----
if [ "$SKIP_BUILD" -eq 0 ]; then
    info "Building and packaging..."
    make package
fi

# ---- Collect asset files ----
ASSETS=""
for platform in darwin-arm64 darwin-amd64 linux-amd64 linux-arm64; do
    f="dist/${BINARY_NAME}_${VERSION}_${platform}.tar.gz"
    [ -f "$f" ] || die "Asset not found: $f"
    ASSETS="$ASSETS $f"
done
f="dist/${BINARY_NAME}_${VERSION}_windows-amd64.zip"
[ -f "$f" ] || die "Asset not found: $f"
ASSETS="$ASSETS $f"

info "Assets:${ASSETS}"

# ---- GitHub upload ----
if [ "$SKIP_GITHUB" -eq 0 ]; then
    info ""
    info "==> Uploading to GitHub..."
    command -v gh >/dev/null 2>&1 || die "gh CLI not found. Install: https://cli.github.com/"
    if gh release view "$TAG" >/dev/null 2>&1; then
        info "Uploading assets to existing GitHub release ${TAG}"
        gh release upload "$TAG" $ASSETS --clobber
    else
        info "Creating GitHub release ${TAG}"
        gh release create "$TAG" $ASSETS --title "$TAG" --generate-notes
    fi
    info "GitHub: https://github.com/${GITHUB_REPO}/releases/tag/${TAG}"
fi

# ---- GitLab upload ----
if [ "$SKIP_GITLAB" -eq 0 ]; then
    [ -n "$GITLAB_TOKEN" ] || die "GITLAB_TOKEN required. Create at: ${GITLAB_HOST}/-/user_settings/personal_access_tokens"

    info ""
    info "==> Uploading to GitLab..."

    # Ensure gitlab remote exists and push code + tag
    if ! git remote | grep -q "^gitlab$"; then
        git remote add gitlab "$GITLAB_REMOTE_URL"
        info "Added git remote: gitlab -> ${GITLAB_REMOTE_URL}"
    fi
    info "Pushing code and tag to GitLab..."
    git push gitlab HEAD:main 2>/dev/null || warn "Push code failed (may already exist)"
    git push gitlab "$TAG" 2>/dev/null    || warn "Push tag failed (may already exist)"

    # Ensure release exists
    if ! curl -fsSL -H "PRIVATE-TOKEN: ${GITLAB_TOKEN}" \
        "${GITLAB_HOST}/api/v4/projects/${GITLAB_PROJECT_ID}/releases/${TAG}" >/dev/null 2>&1; then
        info "Creating GitLab release ${TAG}..."
        curl -fsSL -X POST \
            -H "PRIVATE-TOKEN: ${GITLAB_TOKEN}" \
            -H "Content-Type: application/json" \
            -d "{\"tag_name\":\"${TAG}\",\"name\":\"${TAG}\"}" \
            "${GITLAB_HOST}/api/v4/projects/${GITLAB_PROJECT_ID}/releases" >/dev/null 2>&1 || \
            warn "Failed to create release (may already exist)"
    fi

    # Get existing release links to avoid duplicates
    EXISTING="$(curl -fsSL -H "PRIVATE-TOKEN: ${GITLAB_TOKEN}" \
        "${GITLAB_HOST}/api/v4/projects/${GITLAB_PROJECT_ID}/releases/${TAG}/assets/links?per_page=100" 2>/dev/null || echo "[]")"

    for asset_file in $ASSETS; do
        filename="$(basename "$asset_file")"
        info "Uploading ${filename}..."

        # Upload to generic package registry
        curl -fSL --progress-bar \
            -H "PRIVATE-TOKEN: ${GITLAB_TOKEN}" \
            --upload-file "$asset_file" \
            "${GITLAB_HOST}/api/v4/projects/${GITLAB_PROJECT_ID}/packages/generic/${BINARY_NAME}/${VERSION}/${filename}" >/dev/null

        # Create release link if not exists
        if ! printf "%s" "$EXISTING" | grep -q "\"${filename}\""; then
            PKG_URL="${GITLAB_HOST}/api/v4/projects/${GITLAB_PROJECT_ID}/packages/generic/${BINARY_NAME}/${VERSION}/${filename}"
            curl -fsSL -X POST \
                -H "PRIVATE-TOKEN: ${GITLAB_TOKEN}" \
                -H "Content-Type: application/json" \
                -d "{\"name\":\"${filename}\",\"url\":\"${PKG_URL}\",\"link_type\":\"package\"}" \
                "${GITLAB_HOST}/api/v4/projects/${GITLAB_PROJECT_ID}/releases/${TAG}/assets/links" >/dev/null 2>&1
        fi
        info "${filename} done"
    done
    info "GitLab: ${GITLAB_HOST}/${GITLAB_PROJECT_PATH}/-/releases/${TAG}"
fi

info ""
info "==> Release ${TAG} published successfully!"
