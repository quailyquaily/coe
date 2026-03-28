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
FCITX_LOG_PATH="/tmp/coe-fcitx-$(id -u).log"

INSTALL_MODE_OVERRIDE=""
LOCAL_BUNDLE_PATH=""
BUNDLE_ROOT=""
POSITIONAL_ARGS=()

print_usage() {
  cat <<'EOF'
usage: ./scripts/install.sh [--fcitx|--gnome] [--bundle <path>] [version]

examples:
  ./scripts/install.sh
  ./scripts/install.sh --gnome
  ./scripts/install.sh v0.0.5
  ./scripts/install.sh --bundle ./dist/release/coe_0.0.5_linux_amd64.tar.gz
  ./scripts/install.sh --bundle ./dist/release/bundle-amd64 --fcitx
EOF
}

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

resolve_local_path() {
  local path="$1"
  if [[ -d "${path}" ]]; then
    (
      cd "${path}"
      pwd -P
    )
    return
  fi

  local dir
  local base
  dir="$(dirname "${path}")"
  base="$(basename "${path}")"
  (
    cd "${dir}"
    printf '%s/%s\n' "$(pwd -P)" "${base}"
  )
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

detect_install_mode() {
  if [[ -n "${INSTALL_MODE_OVERRIDE}" ]]; then
    echo "${INSTALL_MODE_OVERRIDE}"
    return
  fi

  if command -v fcitx5 >/dev/null 2>&1 && [[ -d /usr/share/fcitx5 ]]; then
    echo "fcitx"
    return
  fi

  echo "gnome"
}

prepare_local_bundle_root() {
  local tmp_dir="$1"
  local source_path
  local local_extract_dir

  source_path="$(resolve_local_path "${LOCAL_BUNDLE_PATH}")"
  if [[ ! -e "${source_path}" ]]; then
    echo "local bundle path does not exist: ${source_path}" >&2
    exit 1
  fi

  case "${source_path}" in
    *.tar.gz|*.tgz)
      local_extract_dir="${tmp_dir}/local-bundle"
      mkdir -p "${local_extract_dir}"
      echo "extracting local bundle ${source_path}"
      tar -xzf "${source_path}" -C "${local_extract_dir}"
      BUNDLE_ROOT="${local_extract_dir}"
      return
      ;;
  esac

  if [[ -d "${source_path}" ]]; then
    echo "using local bundle directory ${source_path}"
    BUNDLE_ROOT="${source_path}"
    return
  fi

  echo "unsupported local bundle path: ${source_path}" >&2
  echo "Use either an extracted release bundle directory or a .tar.gz archive from build-release-bundle.sh." >&2
  exit 1
}

prepare_remote_bundle_root() {
  local tmp_dir="$1"
  if [[ -n "${LOCAL_BUNDLE_PATH}" ]]; then
    prepare_local_bundle_root "${tmp_dir}"
    return
  fi

  local version_input
  local version
  local asset_version
  local arch
  local archive_name
  local archive_url
  local archive_path

  version_input="${POSITIONAL_ARGS[0]:-${COE_VERSION:-${DEFAULT_VERSION}}}"
  version="$(normalize_version "${version_input}")"
  if [[ "${version}" == "latest" ]]; then
    version="$(resolve_latest_version)"
  fi
  if [[ -z "${version}" ]]; then
    echo "failed to resolve release version" >&2
    exit 1
  fi
  VERSION="${version}"
  asset_version="${VERSION#v}"
  arch="$(detect_arch)"

  archive_name="${PROJECT_NAME}_${asset_version}_linux_${arch}.tar.gz"
  archive_url="https://github.com/${REPO_SLUG}/releases/download/${VERSION}/${archive_name}"
  archive_path="${tmp_dir}/${archive_name}"

  echo "downloading ${archive_url}"
  download_file "${archive_url}" "${archive_path}"

  echo "extracting ${archive_name}"
  tar -xzf "${archive_path}" -C "${tmp_dir}"
  BUNDLE_ROOT="${tmp_dir}"
}

run_as_root_install() {
  if [[ "${EUID}" -eq 0 ]]; then
    install "$@"
    return
  fi

  if command -v sudo >/dev/null 2>&1; then
    sudo install "$@"
    return
  fi

  echo "installing the Fcitx5 module requires root privileges." >&2
  echo "Re-run with a user that can use sudo, or use ./scripts/install.sh --gnome" >&2
  exit 1
}

install_gnome_assets() {
  local helper_src="$1"

  mkdir -p "${GNOME_EXTENSIONS_DIR}"
  if [[ -d "${helper_src}" ]]; then
    echo "installing GNOME focus helper -> ${GNOME_FOCUS_HELPER_DST}"
    if [[ -d "${OLD_GNOME_FOCUS_HELPER_DST}" ]]; then
      rm -rf "${OLD_GNOME_FOCUS_HELPER_DST}"
    fi
    rm -rf "${GNOME_FOCUS_HELPER_DST}"
    cp -r "${helper_src}" "${GNOME_FOCUS_HELPER_DST}"

    if command -v gnome-extensions >/dev/null 2>&1; then
      gnome-extensions disable "${OLD_GNOME_FOCUS_HELPER_UUID}" >/dev/null 2>&1 || true
      gnome-extensions enable "${GNOME_FOCUS_HELPER_UUID}" || true
    fi
  fi
}

install_fcitx_assets() {
  local runtime_root="$1"

  if [[ ! -d "${runtime_root}/usr" ]]; then
    echo "release archive missing staged Fcitx5 runtime assets: ${runtime_root}/usr" >&2
    exit 1
  fi

  local module_src
  module_src="$(find "${runtime_root}/usr/lib" -path '*/fcitx5/libcoefcitx.so' | head -n 1)"
  local addon_src="${runtime_root}/usr/share/fcitx5/addon/coe.conf"

  if [[ -z "${module_src}" || ! -f "${addon_src}" ]]; then
    echo "release archive is missing the Fcitx5 module payload" >&2
    exit 1
  fi

  local module_dst="/${module_src#${runtime_root}/}"
  local addon_dst="/${addon_src#${runtime_root}/}"

  echo "installing Fcitx5 module -> ${module_dst}"
  run_as_root_install -D -m 0755 "${module_src}" "${module_dst}"
  run_as_root_install -D -m 0644 "${addon_src}" "${addon_dst}"

  rm -f "${FCITX_LOG_PATH}"
}

restart_fcitx_if_running() {
  if ! command -v fcitx5 >/dev/null 2>&1; then
    echo "- fcitx5 binary not found; restart it manually after install"
    return
  fi
  if ! pgrep -x fcitx5 >/dev/null 2>&1; then
    echo "- fcitx5 is not running; it will pick up the module next time it starts"
    return
  fi

  echo "restarting fcitx5..."
  if fcitx5 -rd >/dev/null 2>&1; then
    echo "- fcitx5 restart requested"
    sleep 1
  else
    echo "- failed to restart fcitx5 automatically; restart it manually or log out and back in"
  fi

  if [[ -f "${FCITX_LOG_PATH}" ]] && grep -q "init ok" "${FCITX_LOG_PATH}"; then
    echo "- fcitx module reported init ok"
  else
    echo "- fcitx module load not confirmed yet; check ${FCITX_LOG_PATH} after restart"
  fi
}

while (($# > 0)); do
  case "$1" in
    --gnome)
      INSTALL_MODE_OVERRIDE="gnome"
      shift
      ;;
    --fcitx)
      INSTALL_MODE_OVERRIDE="fcitx"
      shift
      ;;
    --bundle)
      if [[ $# -lt 2 ]]; then
        echo "--bundle requires a path" >&2
        exit 1
      fi
      LOCAL_BUNDLE_PATH="$2"
      shift 2
      ;;
    --bundle=*)
      LOCAL_BUNDLE_PATH="${1#--bundle=}"
      shift
      ;;
    -h|--help)
      print_usage
      exit 0
      ;;
    *)
      POSITIONAL_ARGS+=("$1")
      shift
      ;;
  esac
done

if [[ ${#POSITIONAL_ARGS[@]} -gt 1 ]]; then
  print_usage >&2
  exit 1
fi

if [[ -n "${LOCAL_BUNDLE_PATH}" && ${#POSITIONAL_ARGS[@]} -gt 0 ]]; then
  echo "version argument cannot be used with --bundle" >&2
  exit 1
fi

VERSION="local-build"
INSTALL_MODE="$(detect_install_mode)"

require_cmd tar
require_cmd install
require_cmd systemctl

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

prepare_remote_bundle_root "${TMP_DIR}"
BIN_SRC="${BUNDLE_ROOT}/coe"
UNIT_SRC="${BUNDLE_ROOT}/packaging/systemd/coe.service"
GNOME_FOCUS_HELPER_SRC="${BUNDLE_ROOT}/packaging/gnome-shell-extension/${GNOME_FOCUS_HELPER_UUID}"
FCITX_RUNTIME_ROOT="${BUNDLE_ROOT}/packaging/fcitx5/runtime"

if [[ ! -f "${BIN_SRC}" ]]; then
  echo "release archive missing binary: ${BIN_SRC}" >&2
  exit 1
fi
if [[ ! -f "${UNIT_SRC}" ]]; then
  echo "release archive missing systemd unit: ${UNIT_SRC}" >&2
  exit 1
fi

mkdir -p "${BIN_DIR}" "${CONFIG_DIR}" "${SYSTEMD_DIR}"

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

if [[ "${INSTALL_MODE}" == "fcitx" ]]; then
  install_fcitx_assets "${FCITX_RUNTIME_ROOT}"
  "${BIN_DIR}/coe" config set runtime.mode fcitx >/dev/null
else
  install_gnome_assets "${GNOME_FOCUS_HELPER_SRC}"
  "${BIN_DIR}/coe" config set runtime.mode desktop >/dev/null
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
if [[ "${INSTALL_MODE}" == "fcitx" ]]; then
  echo "- fcitx addon config: /usr/share/fcitx5/addon/coe.conf"
  echo "- fcitx module: installed into /usr/lib/*/fcitx5/libcoefcitx.so"
else
  echo "- GNOME extension: ${GNOME_FOCUS_HELPER_DST}"
fi

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

if [[ "${INSTALL_MODE}" == "fcitx" ]]; then
  echo
  echo "Fcitx check:"
  restart_fcitx_if_running
fi

echo
echo "Coe ${VERSION} installed."
echo "- runtime mode: ${INSTALL_MODE}"
echo
echo "Next steps:"
echo "1. If you use cloud ASR or LLM providers, put the required API key(s) in ${ENV_PATH} or ${CONFIG_DIR}/config.yaml"
if [[ "${INSTALL_MODE}" == "fcitx" ]]; then
  echo "2. Open any app with an active Fcitx input context and press the configured hotkey"
  echo "3. If the module still does not load, check ${FCITX_LOG_PATH}"
else
  echo "2. Log out and log back in once so GNOME Shell and your user session both pick up the new extension and service cleanly"
  echo "3. Check logs: journalctl --user -u coe.service -f"
fi

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
