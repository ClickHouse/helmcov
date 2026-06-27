#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:?version required}"
REPO="${2:?repository required}"
INSTALL_DIR="${3:?install dir required}"

mkdir -p "$INSTALL_DIR"

if [ "$VERSION" = "dev" ]; then
  go build -o "${INSTALL_DIR}/helmcov" ./cmd/helmcov
  exit 0
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
