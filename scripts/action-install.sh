#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:?version required}"
REPO="${2:?repository required}"
INSTALL_DIR="${3:?install dir required}"

mkdir -p "$INSTALL_DIR"

build_from_dir() {
  local src_dir="$1"
  if [ ! -f "${src_dir}/cmd/helmcov/main.go" ]; then
    echo "helmcov source not found at ${src_dir}/cmd/helmcov" >&2
    return 1
  fi
  (cd "${src_dir}" && go build -o "${INSTALL_DIR}/helmcov" ./cmd/helmcov)
  chmod +x "${INSTALL_DIR}/helmcov"
}

binary_supports_markdown() {
  [ -x "${INSTALL_DIR}/helmcov" ] && \
    "${INSTALL_DIR}/helmcov" --help 2>&1 | grep -q 'markdown-file'
}

if [ "$VERSION" = "dev" ]; then
  build_from_dir "${GITHUB_WORKSPACE}"
  exit 0
fi

resolve_latest_version() {
  local tag
  tag="$(
    curl -fsSL \
      -H "Accept: application/vnd.github+json" \
      "https://api.github.com/repos/${REPO}/releases/latest" \
      | python3 -c 'import json,sys; print(json.load(sys.stdin)["tag_name"])'
  )" || true
  if [ -z "${tag:-}" ]; then
    echo "failed to resolve latest release for ${REPO}" >&2
    return 1
  fi
  printf '%s' "$tag"
}

if [ "$VERSION" = "latest" ]; then
  VERSION="$(resolve_latest_version)"
  echo "resolved latest helmcov release: ${VERSION}" >&2
fi

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  *)
    echo "unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

case "$OS" in
  linux | darwin) ;;
  mingw* | msys* | cygwin*)
    OS="windows"
    ;;
  *)
    echo "unsupported OS: $OS" >&2
    exit 1
    ;;
esac

VERSION_NUM="${VERSION#v}"
ASSET="helmcov_${VERSION_NUM}_${OS}_${ARCH}"
if [ "$OS" = "windows" ]; then
  ARCHIVE="${INSTALL_DIR}/${ASSET}.zip"
  curl -fsSL "https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}.zip" -o "$ARCHIVE"
  unzip -o "$ARCHIVE" -d "$INSTALL_DIR"
else
  ARCHIVE="${INSTALL_DIR}/${ASSET}.tar.gz"
  curl -fsSL "https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}.tar.gz" -o "$ARCHIVE"
  tar -xzf "$ARCHIVE" -C "$INSTALL_DIR"
fi

chmod +x "${INSTALL_DIR}/helmcov"

if binary_supports_markdown; then
  exit 0
fi

ACTION_ROOT="${GITHUB_ACTION_PATH:-}"
if [ -n "$ACTION_ROOT" ] && [ -f "${ACTION_ROOT}/internal/reporters/markdown.go" ]; then
  echo "helmcov ${VERSION} lacks markdown support; building from action source" >&2
  build_from_dir "$ACTION_ROOT"
  exit 0
fi

echo "helmcov ${VERSION} does not support markdown output and no action source fallback is available" >&2
exit 1
