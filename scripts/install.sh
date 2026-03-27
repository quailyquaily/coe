#!/usr/bin/env bash
set -euo pipefail

REPO_SLUG="quailyquaily/coe"
PROJECT_NAME="coe"
DEFAULT_VERSION="latest"

BIN_DIR="${HOME}/.local/bin"
CONFIG_DIR="${HOME}/.config/coe"
SYSTEMD_DIR="${HOME}/.config/systemd/user"
UNIT_PATH="${SYSTEMD_DIR}/coe.service"
ENV_PATH="${CONFIG_DIR}/env"
GNOME_EXTENSIONS_DIR="${HOME}/.local/share/gnome-shell/extensions"
GNOME_FOCUS_HELPER_UUID="coe-focus-helper@mistermorph.com"
OLD_GNOME_FOCUS_HELPER_UUID="coe-focus-helper@quaily.com"
GNOME_FOCUS_HELPER_DST="${GNOME_EXTENSIONS_DIR}/${GNOME_FOCUS_HELPER_UUID}"
OLD_GNOME_FOCUS_HELPER_DST="${GNOME_EXTENSIONS_DIR}/${OLD_GNOME_FOCUS_HELPER_UUID}"

require_cmd() {
  if command -v "$1" >/dev/null 2>&1; then
    return 0
  fi
  echo "missing required command: $1" >&2
  exit 1
}

fetch_stdout() {
  local url="$1"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO- "$url"
    return
  fi
  echo "missing downloader: install curl or wget" >&2
  exit 1
}

download_file() {
  local url="$1"
  local output="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$output"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO "$output" "$url"
    return
  fi
  echo "missing downloader: install curl or wget" >&2
  exit 1
}

normalize_version() {
  local value="$1"
  if [[ -z "$value" || "$value" == "latest" ]]; then
    echo "latest"
    return
  fi
  if [[ "$value" == v* ]]; then
    echo "$value"
    return
  fi
  echo "v${value}"
}

resolve_latest_version() {
  fetch_stdout "https://api.github.com/repos/${REPO_SLUG}/releases/latest" | \
    sed -n 's/^[[:space:]]*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)
      echo "amd64"
      ;;
    aarch64|arm64)
      echo "arm64"
      ;;
    *)
      echo "unsupported architecture: $(uname -m)" >&2
      exit 1
      ;;
  esac
}

VERSION_INPUT="${1:-${COE_VERSION:-${DEFAULT_VERSION}}}"
VERSION="$(normalize_version "${VERSION_INPUT}")"
if [[ "${VERSION}" == "latest" ]]; then
  VERSION="$(resolve_latest_version)"
fi
if [[ -z "${VERSION}" ]]; then
  echo "failed to resolve release version" >&2
  exit 1
fi
ASSET_VERSION="${VERSION#v}"

ARCH="$(detect_arch)"
ARCHIVE_NAME="${PROJECT_NAME}_${ASSET_VERSION}_linux_${ARCH}.tar.gz"
ARCHIVE_URL="https://github.com/${REPO_SLUG}/releases/download/${VERSION}/${ARCHIVE_NAME}"

require_cmd tar
require_cmd install
require_cmd systemctl

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

ARCHIVE_PATH="${TMP_DIR}/${ARCHIVE_NAME}"
echo "downloading ${ARCHIVE_URL}"
download_file "${ARCHIVE_URL}" "${ARCHIVE_PATH}"

echo "extracting ${ARCHIVE_NAME}"
tar -xzf "${ARCHIVE_PATH}" -C "${TMP_DIR}"

BUNDLE_ROOT="${TMP_DIR}"
BIN_SRC="${BUNDLE_ROOT}/coe"
UNIT_SRC="${BUNDLE_ROOT}/packaging/systemd/coe.service"
GNOME_FOCUS_HELPER_SRC="${BUNDLE_ROOT}/packaging/gnome-shell-extension/${GNOME_FOCUS_HELPER_UUID}"

if [[ ! -f "${BIN_SRC}" ]]; then
  echo "release archive missing binary: ${BIN_SRC}" >&2
  exit 1
fi
if [[ ! -f "${UNIT_SRC}" ]]; then
  echo "release archive missing systemd unit: ${UNIT_SRC}" >&2
  exit 1
fi

mkdir -p "${BIN_DIR}" "${CONFIG_DIR}" "${SYSTEMD_DIR}" "${GNOME_EXTENSIONS_DIR}"

echo "installing coe ${VERSION} -> ${BIN_DIR}/coe"
install -m 0755 "${BIN_SRC}" "${BIN_DIR}/coe"

echo "ensuring default config exists"
"${BIN_DIR}/coe" config init >/dev/null || true

if [[ ! -f "${ENV_PATH}" ]]; then
  cat >"${ENV_PATH}" <<'EOF'
OPENAI_API_KEY=
EOF
  chmod 600 "${ENV_PATH}"
  echo "wrote ${ENV_PATH}"
fi

install -m 0644 "${UNIT_SRC}" "${UNIT_PATH}"

if [[ -d "${GNOME_FOCUS_HELPER_SRC}" ]]; then
  echo "installing GNOME focus helper -> ${GNOME_FOCUS_HELPER_DST}"
  if [[ -d "${OLD_GNOME_FOCUS_HELPER_DST}" ]]; then
    rm -rf "${OLD_GNOME_FOCUS_HELPER_DST}"
  fi
  rm -rf "${GNOME_FOCUS_HELPER_DST}"
  cp -r "${GNOME_FOCUS_HELPER_SRC}" "${GNOME_FOCUS_HELPER_DST}"

  if command -v gnome-extensions >/dev/null 2>&1; then
    gnome-extensions disable "${OLD_GNOME_FOCUS_HELPER_UUID}" >/dev/null 2>&1 || true
    gnome-extensions enable "${GNOME_FOCUS_HELPER_UUID}" || true
  fi
fi

systemctl --user import-environment \
  DISPLAY \
  WAYLAND_DISPLAY \
  XDG_CURRENT_DESKTOP \
  XDG_SESSION_TYPE \
  DBUS_SESSION_BUS_ADDRESS \
  XDG_RUNTIME_DIR || true

systemctl --user daemon-reload
systemctl --user enable --now coe.service
systemctl --user restart coe.service

echo
echo "Installed files:"
echo "- binary: ${BIN_DIR}/coe"
echo "- config: ${CONFIG_DIR}/config.yaml"
echo "- env: ${ENV_PATH}"
echo "- systemd unit: ${UNIT_PATH}"
echo "- GNOME extension: ${GNOME_FOCUS_HELPER_DST}"

echo
echo "Doctor report:"
"${BIN_DIR}/coe" doctor

echo
echo "Service check:"
if systemctl --user is-active --quiet coe.service; then
  echo "- coe.service is active"
else
  echo "- coe.service failed to start" >&2
  systemctl --user --no-pager --full status coe.service || true
  exit 1
fi

echo
echo "Coe ${VERSION} installed."
echo
echo "Next steps:"
echo "1. If you use cloud ASR or LLM providers, put the required API key(s) in ${ENV_PATH} or ${CONFIG_DIR}/config.yaml"
echo "2. Log out and log back in once so GNOME Shell and your user session both pick up the new extension and service cleanly"
echo "3. Check logs: journalctl --user -u coe.service -f"

missing_runtime=()
for runtime_bin in pw-record wl-copy; do
  if ! command -v "${runtime_bin}" >/dev/null 2>&1; then
    missing_runtime+=("${runtime_bin}")
  fi
done
if [[ ${#missing_runtime[@]} -gt 0 ]]; then
  echo
  echo "Runtime dependencies still missing: ${missing_runtime[*]}"
  echo "On Ubuntu, install them with: sudo apt install -y pipewire-bin wl-clipboard"
fi
