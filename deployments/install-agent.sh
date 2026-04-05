#!/bin/sh
set -eu

REPO="${REPO:-gofxq/gaoming}"
VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/opt/gaoming-agent}"
SERVICE_NAME="${SERVICE_NAME:-gaoming-agent}"
SERVICE_USER="${SERVICE_USER:-gaoming-agent}"
SERVICE_GROUP="${SERVICE_GROUP:-gaoming-agent}"
MASTER_API_URL="${MASTER_API_URL:-}"
INGEST_GATEWAY_URL="${INGEST_GATEWAY_URL:-}"
AGENT_REGION="${AGENT_REGION:-}"
AGENT_ENV="${AGENT_ENV:-}"
AGENT_ROLE="${AGENT_ROLE:-}"
AGENT_TENANT="${AGENT_TENANT:-}"
AGENT_LOOP_INTERVAL_SEC="${AGENT_LOOP_INTERVAL_SEC:-}"

usage() {
  cat <<'EOF'
usage: install-agent.sh [options]

Options:
  --repo <owner/name>              GitHub repo, default: gofxq/gaoming
  --version <tag|latest>           Release tag to install, default: latest
  --install-dir <path>             Install dir, default: /opt/gaoming-agent
  --service-name <name>            systemd service name, default: gaoming-agent
  --service-user <name>            service user, default: gaoming-agent
  --service-group <name>           service group, default: gaoming-agent
  --master-url <url>               Agent MASTER_API_URL
  --ingest-url <url>               Agent INGEST_GATEWAY_URL
  --region <name>                  Agent region
  --env <name>                     Agent env
  --role <name>                    Agent role
  --tenant <code>                  Agent tenant code
  --loop-interval-sec <seconds>    Agent loop interval
  --help                           Show this help
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --repo)
      REPO="$2"
      shift 2
      ;;
    --version)
      VERSION="$2"
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
    --master-url)
      MASTER_API_URL="$2"
      shift 2
      ;;
    --ingest-url)
      INGEST_GATEWAY_URL="$2"
      shift 2
      ;;
    --region)
      AGENT_REGION="$2"
      shift 2
      ;;
    --env)
      AGENT_ENV="$2"
      shift 2
      ;;
    --role)
      AGENT_ROLE="$2"
      shift 2
      ;;
    --tenant)
      AGENT_TENANT="$2"
      shift 2
      ;;
    --loop-interval-sec)
      AGENT_LOOP_INTERVAL_SEC="$2"
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
  echo "install-agent.sh must run as root" >&2
  exit 1
fi

if ! command -v systemctl >/dev/null 2>&1; then
  echo "systemctl not found; this installer currently supports systemd hosts only" >&2
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

read_config_value() {
  key="$1"
  file="$2"
  if [ ! -f "$file" ]; then
    return 0
  fi
  sed -n "s/^${key}:[[:space:]]*//p" "$file" | head -n1 | tr -d '"' | tr -d "'"
}

ensure_group() {
  if getent group "$SERVICE_GROUP" >/dev/null 2>&1; then
    return 0
  fi
  if command -v groupadd >/dev/null 2>&1; then
    groupadd --system "$SERVICE_GROUP"
    return 0
  fi
  if command -v addgroup >/dev/null 2>&1; then
    addgroup --system "$SERVICE_GROUP"
    return 0
  fi
  echo "unable to create group $SERVICE_GROUP" >&2
  exit 1
}

ensure_user() {
  if id "$SERVICE_USER" >/dev/null 2>&1; then
    return 0
  fi
  if command -v useradd >/dev/null 2>&1; then
    useradd --system --home-dir "$INSTALL_DIR" --shell /usr/sbin/nologin --gid "$SERVICE_GROUP" "$SERVICE_USER"
    return 0
  fi
  if command -v adduser >/dev/null 2>&1; then
    if adduser --help 2>&1 | grep -q -- '--system'; then
      adduser --system --home "$INSTALL_DIR" --ingroup "$SERVICE_GROUP" "$SERVICE_USER"
      return 0
    fi
    adduser -S -H -h "$INSTALL_DIR" -G "$SERVICE_GROUP" "$SERVICE_USER"
    return 0
  fi
  echo "unable to create user $SERVICE_USER" >&2
  exit 1
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

ARCH="$(detect_arch)"
ASSET="gaoming-agent_linux_${ARCH}.tar.gz"
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

CONFIG_FILE="${INSTALL_DIR}/agent-config.yaml"

if [ -z "$MASTER_API_URL" ]; then
  MASTER_API_URL="$(read_config_value master_api_url "$CONFIG_FILE")"
fi
if [ -z "$INGEST_GATEWAY_URL" ]; then
  INGEST_GATEWAY_URL="$(read_config_value ingest_gateway_url "$CONFIG_FILE")"
fi
if [ -z "$AGENT_REGION" ]; then
  AGENT_REGION="$(read_config_value region "$CONFIG_FILE")"
fi
if [ -z "$AGENT_ENV" ]; then
  AGENT_ENV="$(read_config_value env "$CONFIG_FILE")"
fi
if [ -z "$AGENT_ROLE" ]; then
  AGENT_ROLE="$(read_config_value role "$CONFIG_FILE")"
fi
if [ -z "$AGENT_TENANT" ]; then
  AGENT_TENANT="$(read_config_value tenant_code "$CONFIG_FILE")"
fi
if [ -z "$AGENT_LOOP_INTERVAL_SEC" ]; then
  AGENT_LOOP_INTERVAL_SEC="$(read_config_value loop_interval_sec "$CONFIG_FILE")"
fi

MASTER_API_URL="${MASTER_API_URL:-http://127.0.0.1:8080}"
INGEST_GATEWAY_URL="${INGEST_GATEWAY_URL:-http://127.0.0.1:8090}"
AGENT_REGION="${AGENT_REGION:-local}"
AGENT_ENV="${AGENT_ENV:-dev}"
AGENT_ROLE="${AGENT_ROLE:-node}"
AGENT_LOOP_INTERVAL_SEC="${AGENT_LOOP_INTERVAL_SEC:-1}"

cat >"$CONFIG_FILE" <<EOF
master_api_url: "${MASTER_API_URL}"
ingest_gateway_url: "${INGEST_GATEWAY_URL}"
region: "${AGENT_REGION}"
env: "${AGENT_ENV}"
role: "${AGENT_ROLE}"
tenant_code: "${AGENT_TENANT}"
loop_interval_sec: ${AGENT_LOOP_INTERVAL_SEC}
EOF

ensure_group
ensure_user
chown -R "$SERVICE_USER:$SERVICE_GROUP" "$INSTALL_DIR"

SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
cat >"$SERVICE_FILE" <<EOF
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

echo "installed ${SERVICE_NAME} to ${INSTALL_DIR}"
echo "config: ${CONFIG_FILE}"
echo "status: systemctl status ${SERVICE_NAME}"
