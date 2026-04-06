#!/bin/sh
set -eu

REPO="${REPO:-gofxq/gaoming}"
VERSION="${VERSION:-latest}"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH_RAW="$(uname -m)"
SERVICE_NAME="${SERVICE_NAME:-gaoming-agent}"
SERVICE_USER="${SERVICE_USER:-gaoming-agent}"
SERVICE_GROUP="${SERVICE_GROUP:-gaoming-agent}"
MASTER_API_URL="${MASTER_API_URL:-https://gm-metric.gofxq.com/}"
INGEST_GATEWAY_URL="${INGEST_GATEWAY_URL:-https://gm-metric.gofxq.com/}"
AGENT_REGION="${AGENT_REGION:-local}"
AGENT_ENV="${AGENT_ENV:-prod}"
AGENT_ROLE="${AGENT_ROLE:-node}"
AGENT_TENANT="${AGENT_TENANT:-}"
AGENT_LOOP_INTERVAL_SEC="${AGENT_LOOP_INTERVAL_SEC:-5}"
INSTALL_DIR="${INSTALL_DIR:-}"

usage() {
  cat <<'EOF'
usage: install-agent.sh [options]

Options:
  --repo <owner/name>              GitHub repo, default: gofxq/gaoming
  --version <tag|latest>           Release tag to install, default: latest
  --install-dir <path>             Install dir, default: /opt/gaoming-agent or /usr/local/gaoming-agent
  --service-name <name>            Service name, default: gaoming-agent
  --service-user <name>            Linux service user, default: gaoming-agent
  --service-group <name>           Linux service group, default: gaoming-agent
  --master-url <url>               Default: https://gm-metric.gofxq.com/
  --ingest-url <url>               Default: https://gm-metric.gofxq.com/
  --tenant <code>                  Default: random generated tenant
  --loop-interval-sec <seconds>    Default: 5
  --region <name>                  Default: local
  --env <name>                     Default: prod
  --role <name>                    Default: node
  --help                           Show this help
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --repo) REPO="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --install-dir) INSTALL_DIR="$2"; shift 2 ;;
    --service-name) SERVICE_NAME="$2"; shift 2 ;;
    --service-user) SERVICE_USER="$2"; shift 2 ;;
    --service-group) SERVICE_GROUP="$2"; shift 2 ;;
    --master-url) MASTER_API_URL="$2"; shift 2 ;;
    --ingest-url) INGEST_GATEWAY_URL="$2"; shift 2 ;;
    --tenant) AGENT_TENANT="$2"; shift 2 ;;
    --loop-interval-sec) AGENT_LOOP_INTERVAL_SEC="$2"; shift 2 ;;
    --region) AGENT_REGION="$2"; shift 2 ;;
    --env) AGENT_ENV="$2"; shift 2 ;;
    --role) AGENT_ROLE="$2"; shift 2 ;;
    --help|-h) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 1 ;;
  esac
done

if [ "$(id -u)" -ne 0 ]; then
  echo "install-agent.sh must run as root" >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required" >&2
  exit 1
fi

if ! command -v tar >/dev/null 2>&1; then
  echo "tar is required" >&2
  exit 1
fi

detect_arch() {
  case "$ARCH_RAW" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) echo "unsupported architecture: $ARCH_RAW" >&2; exit 1 ;;
  esac
}

generate_tenant() {
  suffix="$(od -An -N6 -tx1 /dev/urandom 2>/dev/null | tr -d ' \n' || true)"
  if [ -z "$suffix" ]; then
    suffix="$(date +%s)"
  fi
  echo "tenant-${suffix}"
}

sha256_check() {
  asset="$1"
  checksums="$2"
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$(dirname "$asset")" && grep "  $(basename "$asset")\$" "$checksums" | sha256sum -c -)
    return 0
  fi
  if command -v shasum >/dev/null 2>&1; then
    expected="$(grep "  $(basename "$asset")\$" "$checksums" | awk '{print $1}')"
    actual="$(shasum -a 256 "$asset" | awk '{print $1}')"
    [ "$expected" = "$actual" ]
    return 0
  fi
  echo "warning: sha256sum/shasum not found, skipping checksum verification" >&2
}

ensure_linux_identity() {
  if getent group "$SERVICE_GROUP" >/dev/null 2>&1; then
    :
  elif command -v groupadd >/dev/null 2>&1; then
    groupadd --system "$SERVICE_GROUP"
  else
    addgroup --system "$SERVICE_GROUP"
  fi

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

ARCH="$(detect_arch)"
case "$OS" in
  linux)
    INSTALL_DIR="${INSTALL_DIR:-/opt/gaoming-agent}"
    ASSET="gaoming-agent_linux_${ARCH}.tar.gz"
    ;;
  darwin)
    INSTALL_DIR="${INSTALL_DIR:-/usr/local/gaoming-agent}"
    ASSET="gaoming-agent_darwin_${ARCH}.tar.gz"
    ;;
  *)
    echo "install-agent.sh supports Linux and Darwin only. Use install-agent.ps1 on Windows." >&2
    exit 1
    ;;
esac

if [ -z "$AGENT_TENANT" ]; then
  AGENT_TENANT="$(generate_tenant)"
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

if [ "$VERSION" = "latest" ]; then
  ASSET_URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
  CHECKSUM_URL="https://github.com/${REPO}/releases/latest/download/checksums.txt"
else
  ASSET_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
  CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"
fi

echo "downloading ${ASSET_URL}"
curl -fsSL "$ASSET_URL" -o "${TMP_DIR}/${ASSET}"
curl -fsSL "$CHECKSUM_URL" -o "${TMP_DIR}/checksums.txt"
sha256_check "${TMP_DIR}/${ASSET}" "${TMP_DIR}/checksums.txt"

mkdir -p "$INSTALL_DIR"
tar -xzf "${TMP_DIR}/${ASSET}" -C "${TMP_DIR}"
install -m 0755 "${TMP_DIR}/gaoming-agent" "${INSTALL_DIR}/gaoming-agent"

cat >"${INSTALL_DIR}/agent-config.yaml" <<EOF
master_api_url: "${MASTER_API_URL}"
ingest_gateway_url: "${INGEST_GATEWAY_URL}"
region: "${AGENT_REGION}"
env: "${AGENT_ENV}"
role: "${AGENT_ROLE}"
tenant_code: "${AGENT_TENANT}"
loop_interval_sec: ${AGENT_LOOP_INTERVAL_SEC}
EOF

case "$OS" in
  linux)
    if ! command -v systemctl >/dev/null 2>&1; then
      echo "systemctl not found" >&2
      exit 1
    fi
    ensure_linux_identity
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
    systemctl enable --now "$SERVICE_NAME"
    ;;
  darwin)
    PLIST="/Library/LaunchDaemons/com.gofxq.${SERVICE_NAME}.plist"
    chown -R root:wheel "$INSTALL_DIR"
    cat >"$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.gofxq.${SERVICE_NAME}</string>
  <key>ProgramArguments</key>
  <array>
    <string>${INSTALL_DIR}/gaoming-agent</string>
  </array>
  <key>WorkingDirectory</key>
  <string>${INSTALL_DIR}</string>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
</dict>
</plist>
EOF
    chmod 644 "$PLIST"
    launchctl bootout system "$PLIST" >/dev/null 2>&1 || true
    launchctl bootstrap system "$PLIST"
    launchctl enable "system/com.gofxq.${SERVICE_NAME}" >/dev/null 2>&1 || true
    launchctl kickstart -k "system/com.gofxq.${SERVICE_NAME}" >/dev/null 2>&1 || true
    ;;
esac

echo "installed ${SERVICE_NAME} to ${INSTALL_DIR}"
echo "config: ${INSTALL_DIR}/agent-config.yaml"
echo "tenant_code: ${AGENT_TENANT}"
