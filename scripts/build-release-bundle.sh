#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

require_cmd() {
  if command -v "$1" >/dev/null 2>&1; then
    return 0
  fi
  echo "missing required command: $1" >&2
  exit 1
}

detect_arch() {
  case "${1:-$(uname -m)}" in
    x86_64|amd64)
      echo "amd64"
      ;;
    aarch64|arm64)
      echo "arm64"
      ;;
    *)
      echo "unsupported architecture: ${1:-$(uname -m)}" >&2
      exit 1
      ;;
  esac
}

VERSION="${1:-}"
ARCH="$(detect_arch "${2:-}")"
OUTPUT_DIR="${3:-${ROOT_DIR}/dist/release}"

if [[ -z "${VERSION}" ]]; then
  echo "usage: ./scripts/build-release-bundle.sh <version> [arch] [output-dir]" >&2
  exit 1
fi

require_cmd go
require_cmd tar
require_cmd git
require_cmd cmake
require_cmd pkg-config

COMMIT="$(git -C "${ROOT_DIR}" rev-parse HEAD)"
BUILD_DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
ASSET_VERSION="${VERSION#v}"
ARCHIVE_NAME="coe_${ASSET_VERSION}_linux_${ARCH}.tar.gz"

BUNDLE_ROOT="${OUTPUT_DIR}/bundle-${ARCH}"
FCITX_BUILD_DIR="${OUTPUT_DIR}/fcitx-build-${ARCH}"
ARCHIVE_PATH="${OUTPUT_DIR}/${ARCHIVE_NAME}"

rm -rf "${BUNDLE_ROOT}" "${FCITX_BUILD_DIR}" "${ARCHIVE_PATH}"
mkdir -p "${BUNDLE_ROOT}"

go build \
  -trimpath \
  -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE} -X main.builtBy=release-bundle" \
  -o "${BUNDLE_ROOT}/coe" \
  "${ROOT_DIR}/cmd/coe"

mkdir -p "${BUNDLE_ROOT}/scripts" \
         "${BUNDLE_ROOT}/docs" \
         "${BUNDLE_ROOT}/packaging/systemd" \
         "${BUNDLE_ROOT}/packaging/gnome-shell-extension"

cp "${ROOT_DIR}/README.md" "${BUNDLE_ROOT}/README.md"
cp "${ROOT_DIR}/config.example.yaml" "${BUNDLE_ROOT}/config.example.yaml"
cp "${ROOT_DIR}/docs/install.md" "${BUNDLE_ROOT}/docs/install.md"
cp "${ROOT_DIR}/scripts/install.sh" "${BUNDLE_ROOT}/scripts/install.sh"
cp "${ROOT_DIR}/packaging/systemd/coe.service" "${BUNDLE_ROOT}/packaging/systemd/coe.service"
cp -r "${ROOT_DIR}/packaging/gnome-shell-extension/coe-focus-helper@mistermorph.com" \
  "${BUNDLE_ROOT}/packaging/gnome-shell-extension/"

BUILD_DIR="${FCITX_BUILD_DIR}" INSTALL_SCOPE=system \
  "${ROOT_DIR}/scripts/build-fcitx-module.sh" --system
DESTDIR="${BUNDLE_ROOT}/packaging/fcitx5/runtime" cmake --install "${FCITX_BUILD_DIR}"

chmod 0755 "${BUNDLE_ROOT}/scripts/install.sh"

tar -czf "${ARCHIVE_PATH}" -C "${BUNDLE_ROOT}" .

echo "release bundle created: ${ARCHIVE_PATH}"
