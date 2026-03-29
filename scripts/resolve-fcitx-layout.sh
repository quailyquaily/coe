#!/usr/bin/env bash
set -euo pipefail

INSTALL_PREFIX_OVERRIDE=""

print_usage() {
  cat <<'EOF'
usage: ./scripts/resolve-fcitx-layout.sh [--install-prefix <path>]

Outputs shell assignments:
  FCITX_LAYOUT_SOURCE
  FCITX_PREFIX
  FCITX_LIBDIR
  FCITX_DATADIR
  FCITX_REL_LIBDIR
  FCITX_REL_DATADIR
  FCITX_MODULE_DIR
  FCITX_ADDON_DIR
EOF
}

emit_var() {
  printf '%s=%q\n' "$1" "$2"
}

relative_to_prefix() {
  local prefix="$1"
  local path="$2"

  if [[ "${path}" == "${prefix}" ]]; then
    echo "."
    return
  fi
  if [[ "${path}" == "${prefix}/"* ]]; then
    echo "${path#${prefix}/}"
    return
  fi

  echo "path ${path} is not under prefix ${prefix}" >&2
  exit 1
}

resolve_existing_module_dir() {
  local candidate
  for candidate in \
    /usr/lib/fcitx5 \
    /usr/lib64/fcitx5 \
    /usr/lib/*/fcitx5 \
    /usr/local/lib/fcitx5 \
    /usr/local/lib64/fcitx5 \
    /usr/local/lib/*/fcitx5
  do
    if [[ -d "${candidate}" ]]; then
      echo "${candidate}"
      return 0
    fi
  done

  return 1
}

resolve_from_pkg_config() {
  if ! command -v pkg-config >/dev/null 2>&1; then
    return 1
  fi
  if ! pkg-config --exists Fcitx5Core; then
    return 1
  fi

  FCITX_LAYOUT_SOURCE="pkg-config"
  FCITX_PREFIX="$(pkg-config --variable=prefix Fcitx5Core)"
  FCITX_LIBDIR="$(pkg-config --variable=libdir Fcitx5Core)"
  FCITX_DATADIR="$(pkg-config --variable=datadir Fcitx5Core)"
  if [[ -z "${FCITX_DATADIR}" ]]; then
    FCITX_DATADIR="${FCITX_PREFIX}/share"
  fi

  FCITX_REL_LIBDIR="$(relative_to_prefix "${FCITX_PREFIX}" "${FCITX_LIBDIR}")"
  FCITX_REL_DATADIR="$(relative_to_prefix "${FCITX_PREFIX}" "${FCITX_DATADIR}")"
  return 0
}

resolve_from_filesystem() {
  local module_dir
  module_dir="$(resolve_existing_module_dir)" || return 1

  FCITX_LAYOUT_SOURCE="filesystem"
  FCITX_LIBDIR="${module_dir%/fcitx5}"
  case "${module_dir}" in
    /usr/local/*)
      FCITX_PREFIX="/usr/local"
      ;;
    /usr/*)
      FCITX_PREFIX="/usr"
      ;;
    *)
      FCITX_PREFIX="$(dirname "$(dirname "${FCITX_LIBDIR}")")"
      ;;
  esac
  FCITX_DATADIR="${FCITX_PREFIX}/share"
  FCITX_REL_LIBDIR="$(relative_to_prefix "${FCITX_PREFIX}" "${FCITX_LIBDIR}")"
  FCITX_REL_DATADIR="$(relative_to_prefix "${FCITX_PREFIX}" "${FCITX_DATADIR}")"
  return 0
}

apply_install_prefix_override() {
  local override="$1"
  if [[ -z "${override}" ]]; then
    return
  fi

  FCITX_PREFIX="${override}"
  if [[ "${FCITX_REL_LIBDIR}" == "." ]]; then
    FCITX_LIBDIR="${FCITX_PREFIX}"
  else
    FCITX_LIBDIR="${FCITX_PREFIX}/${FCITX_REL_LIBDIR}"
  fi
  if [[ "${FCITX_REL_DATADIR}" == "." ]]; then
    FCITX_DATADIR="${FCITX_PREFIX}"
  else
    FCITX_DATADIR="${FCITX_PREFIX}/${FCITX_REL_DATADIR}"
  fi
}

while (($# > 0)); do
  case "$1" in
    --install-prefix)
      if [[ $# -lt 2 ]]; then
        echo "--install-prefix requires a path" >&2
        exit 1
      fi
      INSTALL_PREFIX_OVERRIDE="$2"
      shift 2
      ;;
    -h|--help)
      print_usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      print_usage >&2
      exit 1
      ;;
  esac
done

if ! resolve_from_pkg_config && ! resolve_from_filesystem; then
  echo "failed to resolve local Fcitx5 layout" >&2
  exit 1
fi

apply_install_prefix_override "${INSTALL_PREFIX_OVERRIDE}"

FCITX_MODULE_DIR="${FCITX_LIBDIR}/fcitx5"
FCITX_ADDON_DIR="${FCITX_DATADIR}/fcitx5/addon"

emit_var FCITX_LAYOUT_SOURCE "${FCITX_LAYOUT_SOURCE}"
emit_var FCITX_PREFIX "${FCITX_PREFIX}"
emit_var FCITX_LIBDIR "${FCITX_LIBDIR}"
emit_var FCITX_DATADIR "${FCITX_DATADIR}"
emit_var FCITX_REL_LIBDIR "${FCITX_REL_LIBDIR}"
emit_var FCITX_REL_DATADIR "${FCITX_REL_DATADIR}"
emit_var FCITX_MODULE_DIR "${FCITX_MODULE_DIR}"
emit_var FCITX_ADDON_DIR "${FCITX_ADDON_DIR}"
