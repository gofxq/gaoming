#!/bin/sh
set -eu

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
SERVICE_NAME="${SERVICE_NAME:-gaoming-agent}"
INSTALL_DIR="${INSTALL_DIR:-}"

if [ "$(id -u)" -ne 0 ]; then
  echo "uninstall-agent.sh must run as root" >&2
  exit 1
fi

case "$OS" in
  linux)
    INSTALL_DIR="${INSTALL_DIR:-/opt/gaoming-agent}"
    if command -v systemctl >/dev/null 2>&1; then
      systemctl disable --now "$SERVICE_NAME" >/dev/null 2>&1 || true
      rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
      systemctl daemon-reload >/dev/null 2>&1 || true
    fi
    ;;
  darwin)
    INSTALL_DIR="${INSTALL_DIR:-/usr/local/gaoming-agent}"
    PLIST="/Library/LaunchDaemons/com.gofxq.${SERVICE_NAME}.plist"
    launchctl bootout system "$PLIST" >/dev/null 2>&1 || true
    rm -f "$PLIST"
    ;;
  *)
    echo "uninstall-agent.sh supports Linux and Darwin only. Use uninstall-agent.ps1 on Windows." >&2
    exit 1
    ;;
esac

rm -rf "$INSTALL_DIR"
echo "uninstalled ${SERVICE_NAME}"
