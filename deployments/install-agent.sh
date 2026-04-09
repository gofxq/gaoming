#!/bin/sh
set -eu
umask 022

# ---------- defaults ----------
MASTER_API_URL_EXPLICIT="${MASTER_API_URL+1}"
INGEST_GATEWAY_GRPC_ADDR_EXPLICIT="${INGEST_GATEWAY_GRPC_ADDR+1}"
AGENT_TENANT_EXPLICIT="${AGENT_TENANT+1}"
AGENT_LOOP_INTERVAL_SEC_EXPLICIT="${AGENT_LOOP_INTERVAL_SEC+1}"

REPO="${REPO:-gofxq/gaoming}"
VERSION="${VERSION:-latest}"

SERVICE_NAME="${SERVICE_NAME:-gaoming-agent}"
SERVICE_USER="${SERVICE_USER:-gaoming-agent}"
SERVICE_GROUP="${SERVICE_GROUP:-gaoming-agent}"

MASTER_API_URL="${MASTER_API_URL:-https://gm-metric.gofxq.com/}"
INGEST_GATEWAY_GRPC_ADDR="${INGEST_GATEWAY_GRPC_ADDR:-gm-metric.gofxq.com:8091}"

AGENT_REGION="${AGENT_REGION:-local}"
AGENT_ENV="${AGENT_ENV:-prod}"
AGENT_ROLE="${AGENT_ROLE:-node}"
AGENT_TENANT="${AGENT_TENANT:-}"
AGENT_LOOP_INTERVAL_SEC="${AGENT_LOOP_INTERVAL_SEC:-5}"

INSTALL_DIR="${INSTALL_DIR:-}"

OS=""
ARCH=""
ASSET=""
TMP_DIR=""

# ---------- ui ----------
usage() {
  cat <<'EOF'
usage: install-agent.sh [options]

Options:
  --repo <owner/name>            GitHub repo, default: gofxq/gaoming
  --version <tag|latest>         Release tag to install, default: latest
  --install-dir <path>           Install dir, default: /opt/gaoming-agent or /usr/local/gaoming-agent
  --service-name <name>          Service name, default: gaoming-agent
  --service-user <name>          Linux service user, default: gaoming-agent
  --service-group <name>         Linux service group, default: gaoming-agent
  --master-url <url>             Default: https://gm-metric.gofxq.com/
  --ingest-grpc-addr <addr>      Default: gm-metric.gofxq.com:8091
  --tenant <code>                Default: empty, server generates tenant
  --loop-interval-sec <seconds>  Default: 5
  --region <name>                Default: local
  --env <name>                   Default: prod
  --role <name>                  Default: node
  --help, -h                     Show this help
EOF
}

log()  { printf '[+] %s\n' "$*"; }
warn() { printf '[!] %s\n' "$*" >&2; }
die()  { printf '[x] %s\n' "$*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

has_tty_prompt() {
  [ -r /dev/tty ] && [ -w /dev/tty ]
}

prompt_with_default() {
  label="$1"
  current="$2"
  display_default="$3"

  if ! has_tty_prompt; then
    printf '%s' "$current"
    return 0
  fi

  printf '%s [%s]: ' "$label" "$display_default" >/dev/tty
  if IFS= read -r input </dev/tty && [ -n "$input" ]; then
    printf '%s' "$input"
    return 0
  fi

  printf '%s' "$current"
}

prompt_install_inputs() {
  if ! has_tty_prompt; then
    return 0
  fi

  if [ -z "$MASTER_API_URL_EXPLICIT" ]; then
    MASTER_API_URL="$(prompt_with_default "master-url" "$MASTER_API_URL" "$MASTER_API_URL")"
  fi
  if [ -z "$INGEST_GATEWAY_GRPC_ADDR_EXPLICIT" ]; then
    INGEST_GATEWAY_GRPC_ADDR="$(prompt_with_default "ingest-grpc-addr" "$INGEST_GATEWAY_GRPC_ADDR" "$INGEST_GATEWAY_GRPC_ADDR")"
  fi
  if [ -z "$AGENT_TENANT_EXPLICIT" ]; then
    AGENT_TENANT="$(prompt_with_default "tenant" "$AGENT_TENANT" "${AGENT_TENANT:-<auto>}")"
  fi
  if [ -z "$AGENT_LOOP_INTERVAL_SEC_EXPLICIT" ]; then
    AGENT_LOOP_INTERVAL_SEC="$(prompt_with_default "loop-interval-sec" "$AGENT_LOOP_INTERVAL_SEC" "$AGENT_LOOP_INTERVAL_SEC")"
  fi
}

# ---------- validation ----------
validate_repo() {
  case "$1" in
    ''|/*|*/|*/*/*) die "repo must be in owner/name format" ;;
    */*) : ;;
    *) die "repo must be in owner/name format" ;;
  esac
}

validate_name() {
  case "$1" in
    ''|*[!A-Za-z0-9._-]*) die "$2 contains invalid characters: $1" ;;
    *) : ;;
  esac
}

validate_url() {
  case "$1" in
    http://*|https://*) : ;;
    *) die "$2 must start with http:// or https://: $1" ;;
  esac
}

validate_non_empty() {
  [ -n "$1" ] || die "$2 must not be empty"
}

validate_positive_int() {
  case "$1" in
    ''|*[!0-9]*) die "$2 must be a positive integer" ;;
    0) die "$2 must be greater than 0" ;;
    *) : ;;
  esac
}

validate_abs_path() {
  case "$1" in
    /*) : ;;
    *) die "$2 must be an absolute path: $1" ;;
  esac
}

validate_inputs() {
  validate_repo "$REPO"
  validate_name "$SERVICE_NAME" "service-name"
  validate_name "$SERVICE_USER" "service-user"
  validate_name "$SERVICE_GROUP" "service-group"
  validate_url "$MASTER_API_URL" "master-url"
  validate_non_empty "$INGEST_GATEWAY_GRPC_ADDR" "ingest-grpc-addr"
  validate_positive_int "$AGENT_LOOP_INTERVAL_SEC" "loop-interval-sec"
  validate_abs_path "$INSTALL_DIR" "install-dir"
}

# ---------- args ----------
parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --repo)                [ "$#" -ge 2 ] || die "missing value for --repo"; REPO="$2"; shift 2 ;;
      --version)             [ "$#" -ge 2 ] || die "missing value for --version"; VERSION="$2"; shift 2 ;;
      --install-dir)         [ "$#" -ge 2 ] || die "missing value for --install-dir"; INSTALL_DIR="$2"; shift 2 ;;
      --service-name)        [ "$#" -ge 2 ] || die "missing value for --service-name"; SERVICE_NAME="$2"; shift 2 ;;
      --service-user)        [ "$#" -ge 2 ] || die "missing value for --service-user"; SERVICE_USER="$2"; shift 2 ;;
      --service-group)       [ "$#" -ge 2 ] || die "missing value for --service-group"; SERVICE_GROUP="$2"; shift 2 ;;
      --master-url)          [ "$#" -ge 2 ] || die "missing value for --master-url"; MASTER_API_URL="$2"; MASTER_API_URL_EXPLICIT=1; shift 2 ;;
      --ingest-grpc-addr)    [ "$#" -ge 2 ] || die "missing value for --ingest-grpc-addr"; INGEST_GATEWAY_GRPC_ADDR="$2"; INGEST_GATEWAY_GRPC_ADDR_EXPLICIT=1; shift 2 ;;
      --tenant)              [ "$#" -ge 2 ] || die "missing value for --tenant"; AGENT_TENANT="$2"; AGENT_TENANT_EXPLICIT=1; shift 2 ;;
      --loop-interval-sec)   [ "$#" -ge 2 ] || die "missing value for --loop-interval-sec"; AGENT_LOOP_INTERVAL_SEC="$2"; AGENT_LOOP_INTERVAL_SEC_EXPLICIT=1; shift 2 ;;
      --region)              [ "$#" -ge 2 ] || die "missing value for --region"; AGENT_REGION="$2"; shift 2 ;;
      --env)                 [ "$#" -ge 2 ] || die "missing value for --env"; AGENT_ENV="$2"; shift 2 ;;
      --role)                [ "$#" -ge 2 ] || die "missing value for --role"; AGENT_ROLE="$2"; shift 2 ;;
      --help|-h)             usage; exit 0 ;;
      *)                     usage >&2; die "unknown argument: $1" ;;
    esac
  done
}

# ---------- platform ----------
detect_os() {
  case "$(uname -s | tr '[:upper:]' '[:lower:]')" in
    linux)  printf '%s\n' "linux" ;;
    darwin) printf '%s\n' "darwin" ;;
    *)      die "only Linux and Darwin are supported" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)   printf '%s\n' "amd64" ;;
    aarch64|arm64)  printf '%s\n' "arm64" ;;
    *)              die "unsupported architecture: $(uname -m)" ;;
  esac
}

prepare_platform() {
  OS="$(detect_os)"
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
  esac
}

require_root() {
  [ "$(id -u)" -eq 0 ] || die "this script must run as root"
}

# ---------- temp / download ----------
cleanup() {
  [ -n "${TMP_DIR:-}" ] && rm -rf "$TMP_DIR"
}

make_tmpdir() {
  if TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gaoming-agent.XXXXXX" 2>/dev/null)"; then
    :
  elif TMP_DIR="$(mktemp -d -t gaoming-agent 2>/dev/null)"; then
    :
  else
    die "failed to create temporary directory"
  fi
  trap cleanup EXIT INT TERM HUP
}

release_base_url() {
  case "$VERSION" in
    latest) printf '%s\n' "https://github.com/${REPO}/releases/latest/download" ;;
    *)      printf '%s\n' "https://github.com/${REPO}/releases/download/${VERSION}" ;;
  esac
}

download_assets() {
  base_url="$(release_base_url)"
  log "downloading ${ASSET}"
  curl -fsSL "${base_url}/${ASSET}" -o "${TMP_DIR}/${ASSET}"
  curl -fsSL "${base_url}/checksums.txt" -o "${TMP_DIR}/checksums.txt"
}

verify_checksum() {
  asset_path="${TMP_DIR}/${ASSET}"
  checksums_path="${TMP_DIR}/checksums.txt"
  filename="$(basename "$asset_path")"
  expected=""
  line="$(
    awk -v file="$filename" '
      {
        candidate = $2
        sub(/^\*/, "", candidate)
        sub(/^.*\//, "", candidate)
        if (candidate == file) {
          print
          exit
        }
      }
    ' "$checksums_path"
  )"

  [ -n "$line" ] || die "checksum not found for ${filename}"
  expected="$(printf '%s\n' "$line" | awk '{print $1}')"

  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$asset_path" | awk '{print $1}')"
    [ "$expected" = "$actual" ] || die "checksum verification failed"
    return
  fi

  if command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$asset_path" | awk '{print $1}')"
    [ "$expected" = "$actual" ] || die "checksum verification failed"
    return
  fi

  die "sha256sum or shasum is required for checksum verification"
}

# ---------- file rendering ----------
yaml_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

write_file() {
  path="$1"
  mode="$2"
  owner="${3:-}"
  group="${4:-}"

  dir="$(dirname "$path")"
  base="$(basename "$path")"
  mkdir -p "$dir"

  if tmp="$(mktemp "${dir}/.${base}.XXXXXX" 2>/dev/null)"; then
    :
  else
    tmp="${dir}/.${base}.$$"
    : >"$tmp"
  fi

  cat >"$tmp"
  chmod "$mode" "$tmp"

  if [ -n "$owner" ]; then
    if [ -n "$group" ]; then
      chown "$owner:$group" "$tmp"
    else
      chown "$owner" "$tmp"
    fi
  fi

  mv "$tmp" "$path"
}

read_config_value() {
  key="$1"
  path="$2"
  [ -f "$path" ] || return 1

  awk -F: -v key="$key" '
    $1 == key {
      value = substr($0, index($0, ":") + 1)
      sub(/^[[:space:]]+/, "", value)
      sub(/[[:space:]]+$/, "", value)
      gsub(/^"/, "", value)
      gsub(/"$/, "", value)
      print value
      exit
    }
  ' "$path"
}

wait_for_tenant_code() {
  config_path="${INSTALL_DIR}/agent-config.yaml"
  attempts=0

  while [ "$attempts" -lt 10 ]; do
    tenant_code="$(read_config_value "tenant_code" "$config_path" 2>/dev/null || true)"
    if [ -n "$tenant_code" ]; then
      printf '%s\n' "$tenant_code"
      return 0
    fi
    attempts=$((attempts + 1))
    sleep 1
  done

  return 1
}

build_dashboard_url() {
  tenant_code="$1"
  printf '%s/%s\n' "${MASTER_API_URL%/}" "$tenant_code"
}

build_hosts_api_url() {
  tenant_code="$1"
  printf '%s/master/api/v1/hosts?tenant=%s\n' "${MASTER_API_URL%/}" "$tenant_code"
}

fetch_install_tenant() {
  response="$(curl -fsSL -X POST "${MASTER_API_URL%/}/master/api/v1/install/tenant")" \
    || die "failed to allocate tenant from ${MASTER_API_URL%/}/master/api/v1/install/tenant"
  tenant_code="$(
    printf '%s' "$response" | tr -d '\n' | sed -n 's/.*"tenant_code"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p'
  )"
  [ -n "$tenant_code" ] || die "tenant allocation response did not include tenant_code"
  printf '%s\n' "$tenant_code"
}

ensure_install_tenant() {
  if [ -n "$AGENT_TENANT" ]; then
    return 0
  fi

  log "allocating tenant from master-api"
  AGENT_TENANT="$(fetch_install_tenant)"
}

render_config() {
  cat <<EOF
master_api_url: "$(yaml_escape "$MASTER_API_URL")"
ingest_gateway_grpc_addr: "$(yaml_escape "$INGEST_GATEWAY_GRPC_ADDR")"
region: "$(yaml_escape "$AGENT_REGION")"
env: "$(yaml_escape "$AGENT_ENV")"
role: "$(yaml_escape "$AGENT_ROLE")"
tenant_code: "$(yaml_escape "$AGENT_TENANT")"
loop_interval_sec: ${AGENT_LOOP_INTERVAL_SEC}
EOF
}

render_systemd_unit() {
  cat <<EOF
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
}

render_launchd_plist() {
  cat <<EOF
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
}

# ---------- install ----------
group_exists() {
  if command -v getent >/dev/null 2>&1; then
    getent group "$1" >/dev/null 2>&1
  else
    grep -q "^$1:" /etc/group
  fi
}

ensure_linux_identity() {
  if ! group_exists "$SERVICE_GROUP"; then
    if command -v groupadd >/dev/null 2>&1; then
      groupadd --system "$SERVICE_GROUP"
    elif command -v addgroup >/dev/null 2>&1; then
      addgroup --system "$SERVICE_GROUP" >/dev/null 2>&1 || addgroup -S "$SERVICE_GROUP"
    else
      die "unable to create group: $SERVICE_GROUP"
    fi
  fi

  if id "$SERVICE_USER" >/dev/null 2>&1; then
    return 0
  fi

  if command -v useradd >/dev/null 2>&1; then
    useradd --system --home-dir "$INSTALL_DIR" --shell /usr/sbin/nologin --gid "$SERVICE_GROUP" "$SERVICE_USER" 2>/dev/null \
      || useradd -r -d "$INSTALL_DIR" -s /sbin/nologin -g "$SERVICE_GROUP" "$SERVICE_USER"
    return 0
  fi

  if command -v adduser >/dev/null 2>&1; then
    if adduser --help 2>&1 | grep -q -- '--system'; then
      adduser --system --home "$INSTALL_DIR" --ingroup "$SERVICE_GROUP" "$SERVICE_USER"
    else
      adduser -S -H -h "$INSTALL_DIR" -G "$SERVICE_GROUP" "$SERVICE_USER"
    fi
    return 0
  fi

  die "unable to create user: $SERVICE_USER"
}

install_files() {
  mkdir -p "$INSTALL_DIR"
  tar -xzf "${TMP_DIR}/${ASSET}" -C "$TMP_DIR"
  [ -f "${TMP_DIR}/gaoming-agent" ] || die "gaoming-agent not found in archive"
  install -m 0755 "${TMP_DIR}/gaoming-agent" "${INSTALL_DIR}/gaoming-agent"
  render_config | write_file "${INSTALL_DIR}/agent-config.yaml" 0644
}

install_linux_service() {
  need_cmd systemctl
  ensure_linux_identity
  chown -R "${SERVICE_USER}:${SERVICE_GROUP}" "$INSTALL_DIR"
  render_systemd_unit | write_file "/etc/systemd/system/${SERVICE_NAME}.service" 0644 root root
  systemctl daemon-reload
  systemctl enable --now "$SERVICE_NAME"
}

install_darwin_service() {
  need_cmd launchctl
  plist="/Library/LaunchDaemons/com.gofxq.${SERVICE_NAME}.plist"
  chown -R root:wheel "$INSTALL_DIR"
  render_launchd_plist | write_file "$plist" 0644 root wheel
  launchctl bootout system "$plist" >/dev/null 2>&1 || true
  launchctl bootstrap system "$plist"
  launchctl enable "system/com.gofxq.${SERVICE_NAME}" >/dev/null 2>&1 || true
  launchctl kickstart -k "system/com.gofxq.${SERVICE_NAME}" >/dev/null 2>&1 || true
}

install_service() {
  case "$OS" in
    linux)  install_linux_service ;;
    darwin) install_darwin_service ;;
    *)      die "unsupported OS: $OS" ;;
  esac
}

print_summary() {
  tenant_code="$(wait_for_tenant_code || true)"

  log "installed ${SERVICE_NAME} to ${INSTALL_DIR}"
  log "config: ${INSTALL_DIR}/agent-config.yaml"
  log "ingest_grpc_addr: ${INGEST_GATEWAY_GRPC_ADDR}"
  if [ -n "$tenant_code" ]; then
    log "tenant_code: ${tenant_code}"
    log "dashboard: $(build_dashboard_url "$tenant_code")"
    log "hosts api: $(build_hosts_api_url "$tenant_code")"
    return
  fi

  log "tenant_code: ${AGENT_TENANT:-<auto>}"
  log "dashboard: $(build_dashboard_url "${AGENT_TENANT}")"
  log "hosts api: $(build_hosts_api_url "${AGENT_TENANT}")"
}

main() {
  parse_args "$@"
  prepare_platform
  prompt_install_inputs
  validate_inputs
  require_root

  need_cmd curl
  need_cmd tar
  need_cmd awk
  need_cmd sed
  need_cmd grep
  need_cmd install
  need_cmd mktemp

  ensure_install_tenant
  make_tmpdir
  download_assets
  verify_checksum
  install_files
  install_service
  print_summary
}

main "$@"
