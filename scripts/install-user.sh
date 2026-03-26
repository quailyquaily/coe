#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/.." && pwd)

BIN_DIR="${HOME}/.local/bin"
CONFIG_DIR="${HOME}/.config/coe"
SYSTEMD_DIR="${HOME}/.config/systemd/user"
UNIT_PATH="${SYSTEMD_DIR}/coe.service"
ENV_PATH="${CONFIG_DIR}/env"

mkdir -p "${BIN_DIR}" "${CONFIG_DIR}" "${SYSTEMD_DIR}"

echo "building coe -> ${BIN_DIR}/coe"
(cd "${REPO_ROOT}" && go build -o "${BIN_DIR}/coe" ./cmd/coe)

echo "ensuring default config exists"
"${BIN_DIR}/coe" config init >/dev/null || true

if [[ ! -f "${ENV_PATH}" ]]; then
  cat >"${ENV_PATH}" <<'EOF'
OPENAI_API_KEY=
EOF
  chmod 600 "${ENV_PATH}"
  echo "wrote ${ENV_PATH}"
fi

install -m 0644 "${REPO_ROOT}/packaging/systemd/coe.service" "${UNIT_PATH}"

systemctl --user import-environment \
  DISPLAY \
  WAYLAND_DISPLAY \
  XDG_CURRENT_DESKTOP \
  XDG_SESSION_TYPE \
  DBUS_SESSION_BUS_ADDRESS \
  XDG_RUNTIME_DIR || true

systemctl --user daemon-reload
systemctl --user enable --now coe.service

echo
echo "Coe user service installed."
echo
echo "Next steps:"
echo "1. Put your OpenAI key in ${ENV_PATH}"
echo "2. Restart the service: systemctl --user restart coe.service"
echo "3. Check logs: journalctl --user -u coe.service -f"
echo "4. Add a GNOME custom shortcut that runs: ${BIN_DIR}/coe trigger toggle"
