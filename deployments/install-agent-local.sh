#!/bin/sh
set -eu

BIN_PATH="${BIN_PATH:-$(pwd)/gaoming-agent}"
CONFIG_PATH="${CONFIG_PATH:-$(pwd)/agent-config.yaml}"
INSTALL_DIR="${INSTALL_DIR:-/opt/gaoming-agent}"
SERVICE_NAME="${SERVICE_NAME:-gaoming-agent}"
SERVICE_USER="${SERVICE_USER:-gaoming-agent}"
SERVICE_GROUP="${SERVICE_GROUP:-gaoming-agent}"

usage() {
  cat <<'EOF'
usage: install-agent-local.sh [options]

Options:
  --bin <path>                     Agent binary path, default: ./gaoming-agent
  --config <path>                  Config file, default: ./agent-config.yaml
  --install-dir <path>             Install dir, default: /opt/gaoming-agent
  --service-name <name>            systemd service name, default: gaoming-agent
  --service-user <name>            service user, default: gaoming-agent
  --service-group <name>           service group, default: gaoming-agent
  --help                           Show this help
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --bin)
      BIN_PATH="$2"
      shift 2
      ;;
    --config)
      CONFIG_PATH="$2"
      shift 2
      ;;
    --install-dir)
      INSTALL_DIR="$2"
      shift 2
      ;;
    --service-name)
      SERVICE_NAME="$2"
      shift 2
      ;;
    --service-user)
      SERVICE_USER="$2"
      shift 2
      ;;
    --service-group)
      SERVICE_GROUP="$2"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [ "$(id -u)" -ne 0 ]; then
  if ! command -v sudo >/dev/null 2>&1; then
    echo "sudo is required to install the service" >&2
    exit 1
  fi
  exec sudo env BIN_PATH="$BIN_PATH" CONFIG_PATH="$CONFIG_PATH" \
    INSTALL_DIR="$INSTALL_DIR" SERVICE_NAME="$SERVICE_NAME" \
    SERVICE_USER="$SERVICE_USER" SERVICE_GROUP="$SERVICE_GROUP" \
    sh "$0" "$@"
fi

if ! command -v systemctl >/dev/null 2>&1; then
  echo "systemctl not found; this installer currently supports systemd hosts only" >&2
  exit 1
fi

if [ ! -f "$BIN_PATH" ]; then
  echo "binary not found: ${BIN_PATH}" >&2
  exit 1
fi

if [ ! -x "$BIN_PATH" ]; then
  echo "binary is not executable: ${BIN_PATH}" >&2
  exit 1
fi

if [ ! -f "$CONFIG_PATH" ]; then
  echo "config file not found: ${CONFIG_PATH}" >&2
  exit 1
fi

ensure_group() {
  if getent group "$SERVICE_GROUP" >/dev/null 2>&1; then
    return 0
  fi
  if command -v groupadd >/dev/null 2>&1; then
    groupadd --system "$SERVICE_GROUP"
    return 0
  fi
  addgroup --system "$SERVICE_GROUP"
}

ensure_user() {
  if id "$SERVICE_USER" >/dev/null 2>&1; then
    return 0
  fi
  if command -v useradd >/dev/null 2>&1; then
    useradd --system --home-dir "$INSTALL_DIR" --shell /usr/sbin/nologin --gid "$SERVICE_GROUP" "$SERVICE_USER"
    return 0
  fi
  if adduser --help 2>&1 | grep -q -- '--system'; then
    adduser --system --home "$INSTALL_DIR" --ingroup "$SERVICE_GROUP" "$SERVICE_USER"
    return 0
  fi
  adduser -S -H -h "$INSTALL_DIR" -G "$SERVICE_GROUP" "$SERVICE_USER"
}

ensure_group
ensure_user

mkdir -p "$INSTALL_DIR"
install -m 0755 "$BIN_PATH" "${INSTALL_DIR}/gaoming-agent"
install -m 0644 "$CONFIG_PATH" "${INSTALL_DIR}/agent-config.yaml"
chown -R "$SERVICE_USER:$SERVICE_GROUP" "$INSTALL_DIR"

cat >"/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=Gaoming Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_GROUP}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/gaoming-agent
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "$SERVICE_NAME" >/dev/null 2>&1 || true
systemctl restart "$SERVICE_NAME" >/dev/null 2>&1 || systemctl start "$SERVICE_NAME"

echo "installed ${SERVICE_NAME}"
echo "source binary: ${BIN_PATH}"
echo "installed binary: ${INSTALL_DIR}/gaoming-agent"
echo "config: ${INSTALL_DIR}/agent-config.yaml"
