#!/bin/sh
set -eu
umask 022

# ---------- defaults ----------
WEB_BASE_URL_EXPLICIT="${WEB_BASE_URL+1}"
MASTER_API_URL_EXPLICIT="${MASTER_API_URL+1}"
INGEST_GATEWAY_GRPC_ADDR_EXPLICIT="${INGEST_GATEWAY_GRPC_ADDR+1}"
AGENT_TENANT_EXPLICIT="${AGENT_TENANT+1}"
AGENT_LOOP_INTERVAL_SEC_EXPLICIT="${AGENT_LOOP_INTERVAL_SEC+1}"
AGENT_REGION_EXPLICIT="${AGENT_REGION+1}"
AGENT_ENV_EXPLICIT="${AGENT_ENV+1}"
AGENT_ROLE_EXPLICIT="${AGENT_ROLE+1}"

REPO="${REPO:-gofxq/gaoming}"
VERSION="${VERSION:-latest}"

SERVICE_NAME="${SERVICE_NAME:-gaoming-agent}"
SERVICE_USER="${SERVICE_USER:-gaoming-agent}"
SERVICE_GROUP="${SERVICE_GROUP:-gaoming-agent}"

WEB_BASE_URL="${WEB_BASE_URL:-${MASTER_API_URL:-https://gm-metric.gofxq.com/}}"
MASTER_API_URL="${MASTER_API_URL:-}"
INGEST_GATEWAY_GRPC_ADDR="${INGEST_GATEWAY_GRPC_ADDR:-gm-rpc.gofxq.com:443}"

AGENT_REGION="${AGENT_REGION:-local}"
AGENT_ENV="${AGENT_ENV:-prod}"
AGENT_ROLE="${AGENT_ROLE:-node}"
AGENT_TENANT="${AGENT_TENANT:-}"
AGENT_LOOP_INTERVAL_SEC="${AGENT_LOOP_INTERVAL_SEC:-5}"

INSTALL_DIR="${INSTALL_DIR:-}"
INSTALL_MODE="${INSTALL_MODE:-auto}"
INTERACTIVE="${INTERACTIVE:-0}"

if [ -n "$MASTER_API_URL_EXPLICIT" ] && [ -z "$WEB_BASE_URL_EXPLICIT" ]; then
  WEB_BASE_URL_EXPLICIT=1
fi

OS=""
ARCH=""
ASSET=""
TMP_DIR=""
UPGRADE_IN_PROGRESS=0

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
  --web-url <url>                Default: https://gm-metric.gofxq.com/
  --ingest-grpc-addr <addr>      Default: gm-rpc.gofxq.com:443
  --tenant <code>                Default: empty, installer requests one from master or generates one locally
  --loop-interval-sec <seconds>  Default: 5
  --region <name>                Default: local
  --env <name>                   Default: prod
  --role <name>                  Default: node
  --update-binary                Keep current config and update binary only
  --reinstall                    Replace binary and rewrite config
  --interactive                  Prompt for config and install mode (default: no prompts)
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
  [ "$INTERACTIVE" = 1 ] || return 0
  if ! has_tty_prompt; then
    return 0
  fi

  if [ -z "$WEB_BASE_URL_EXPLICIT" ]; then
    WEB_BASE_URL="$(prompt_with_default "web-url" "$WEB_BASE_URL" "$WEB_BASE_URL")"
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

prompt_install_mode() {
  config_path="$1"

  if [ "$INTERACTIVE" != 1 ] || ! has_tty_prompt; then
    INSTALL_MODE="update-binary"
    return 0
  fi

  while :; do
    printf 'existing config found: %s\n' "$config_path" >/dev/tty
    printf 'choose install mode: [1] update binary only (default), [2] reinstall: ' >/dev/tty
    if ! IFS= read -r input </dev/tty; then
      INSTALL_MODE="update-binary"
      return 0
    fi

    case "$input" in
      ''|1|u|U|update|update-binary)
        INSTALL_MODE="update-binary"
        return 0
        ;;
      2|r|R|reinstall)
        INSTALL_MODE="reinstall"
        return 0
        ;;
    esac
  done
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

load_existing_config_defaults() {
  config_path="${INSTALL_DIR}/agent-config.yaml"
  [ -f "$config_path" ] || return 0

  existing_master_api_url="$(read_config_value "master_api_url" "$config_path" 2>/dev/null || true)"
  if [ -z "$WEB_BASE_URL_EXPLICIT" ] && [ -z "$MASTER_API_URL_EXPLICIT" ] && [ -n "$existing_master_api_url" ]; then
    WEB_BASE_URL="$existing_master_api_url"
    MASTER_API_URL="$existing_master_api_url"
  fi

  if [ -z "$INGEST_GATEWAY_GRPC_ADDR_EXPLICIT" ]; then
    existing_ingest_addr="$(read_config_value "ingest_gateway_grpc_addr" "$config_path" 2>/dev/null || true)"
    if [ -n "$existing_ingest_addr" ]; then
      INGEST_GATEWAY_GRPC_ADDR="$existing_ingest_addr"
    fi
  fi

  if [ -z "$AGENT_TENANT_EXPLICIT" ] && [ -z "$AGENT_TENANT" ]; then
    existing_tenant_code="$(read_config_value "tenant_code" "$config_path" 2>/dev/null || true)"
    if [ -n "$existing_tenant_code" ]; then
      AGENT_TENANT="$existing_tenant_code"
    fi
  fi

  if [ -z "$AGENT_LOOP_INTERVAL_SEC_EXPLICIT" ]; then
    existing_loop_interval="$(read_config_value "loop_interval_sec" "$config_path" 2>/dev/null || true)"
    if [ -n "$existing_loop_interval" ]; then
      AGENT_LOOP_INTERVAL_SEC="$existing_loop_interval"
    fi
  fi

  if [ -z "$AGENT_REGION_EXPLICIT" ]; then
    existing_region="$(read_config_value "region" "$config_path" 2>/dev/null || true)"
    if [ -n "$existing_region" ]; then
      AGENT_REGION="$existing_region"
    fi
  fi

  if [ -z "$AGENT_ENV_EXPLICIT" ]; then
    existing_env="$(read_config_value "env" "$config_path" 2>/dev/null || true)"
    if [ -n "$existing_env" ]; then
      AGENT_ENV="$existing_env"
    fi
  fi

  if [ -z "$AGENT_ROLE_EXPLICIT" ]; then
    existing_role="$(read_config_value "role" "$config_path" 2>/dev/null || true)"
    if [ -n "$existing_role" ]; then
      AGENT_ROLE="$existing_role"
    fi
  fi
}

choose_install_mode() {
  config_path="${INSTALL_DIR}/agent-config.yaml"

  case "$INSTALL_MODE" in
    auto|update-binary|reinstall) ;;
    *)
      die "install mode must be one of: auto, update-binary, reinstall"
      ;;
  esac

  if [ "$INSTALL_MODE" = "update-binary" ] && [ ! -f "$config_path" ]; then
    die "cannot update binary only; installed config not found: ${config_path}"
  fi

  if [ "$INSTALL_MODE" = "reinstall" ] || [ "$INSTALL_MODE" = "update-binary" ]; then
    return 0
  fi

  if [ ! -f "$config_path" ]; then
    INSTALL_MODE="reinstall"
    return 0
  fi

  prompt_install_mode "$config_path"
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
    /|/opt|/usr|/usr/local|/tmp|/etc|/var|/home|*//*|*/../*|*/..|*/./*|*/.|*/|*[!A-Za-z0-9/._-]*)
      die "$2 must be a dedicated directory using letters, numbers, /, ., _, or -" ;;
    /*) : ;;
    *) die "$2 must be an absolute path: $1" ;;
  esac
}

validate_common_inputs() {
  validate_repo "$REPO"
  validate_name "$SERVICE_NAME" "service-name"
  validate_name "$SERVICE_USER" "service-user"
  validate_name "$SERVICE_GROUP" "service-group"
  validate_abs_path "$INSTALL_DIR" "install-dir"
  check_path="$INSTALL_DIR"
  while [ "$check_path" != / ]; do
    [ ! -L "$check_path" ] || die "install-dir must not contain symbolic links: $check_path"
    check_path="$(dirname "$check_path")"
  done
  for check_name in gaoming-agent .gaoming-agent.new .gaoming-agent.rollback agent-config.yaml logs; do
    [ ! -L "${INSTALL_DIR}/${check_name}" ] || die "installed files must not be symbolic links: $check_name"
  done
}

validate_config_inputs() {
  validate_url "$WEB_BASE_URL" "web-url"
  validate_url "$MASTER_API_URL" "master-api-url"
  validate_non_empty "$INGEST_GATEWAY_GRPC_ADDR" "ingest-grpc-addr"
  validate_positive_int "$AGENT_LOOP_INTERVAL_SEC" "loop-interval-sec"
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
      --web-url|--master-url) [ "$#" -ge 2 ] || die "missing value for --web-url"; WEB_BASE_URL="$2"; MASTER_API_URL="$2"; WEB_BASE_URL_EXPLICIT=1; MASTER_API_URL_EXPLICIT=1; shift 2 ;;
      --ingest-grpc-addr)    [ "$#" -ge 2 ] || die "missing value for --ingest-grpc-addr"; INGEST_GATEWAY_GRPC_ADDR="$2"; INGEST_GATEWAY_GRPC_ADDR_EXPLICIT=1; shift 2 ;;
      --tenant)              [ "$#" -ge 2 ] || die "missing value for --tenant"; AGENT_TENANT="$2"; AGENT_TENANT_EXPLICIT=1; shift 2 ;;
      --loop-interval-sec)   [ "$#" -ge 2 ] || die "missing value for --loop-interval-sec"; AGENT_LOOP_INTERVAL_SEC="$2"; AGENT_LOOP_INTERVAL_SEC_EXPLICIT=1; shift 2 ;;
      --region)              [ "$#" -ge 2 ] || die "missing value for --region"; AGENT_REGION="$2"; AGENT_REGION_EXPLICIT=1; shift 2 ;;
      --env)                 [ "$#" -ge 2 ] || die "missing value for --env"; AGENT_ENV="$2"; AGENT_ENV_EXPLICIT=1; shift 2 ;;
      --role)                [ "$#" -ge 2 ] || die "missing value for --role"; AGENT_ROLE="$2"; AGENT_ROLE_EXPLICIT=1; shift 2 ;;
      --update-binary)       INSTALL_MODE="update-binary"; shift ;;
      --reinstall)           INSTALL_MODE="reinstall"; shift ;;
      --interactive)         INTERACTIVE=1; shift ;;
      --no-prompt)           INTERACTIVE=0; shift ;;
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
  status="${1:-0}"
  if [ "$status" -ne 0 ] && [ "$UPGRADE_IN_PROGRESS" = 1 ]; then
    warn "upgrade failed; restoring previous binary and config"
    cp -p "${TMP_DIR}/previous/gaoming-agent" "${INSTALL_DIR}/.gaoming-agent.rollback" && mv -f "${INSTALL_DIR}/.gaoming-agent.rollback" "${INSTALL_DIR}/gaoming-agent" || warn "failed to restore previous binary"
    cp -p "${TMP_DIR}/previous/agent-config.yaml" "${INSTALL_DIR}/agent-config.yaml" || warn "failed to restore previous config"
    chmod 0755 "$INSTALL_DIR"
    case "$OS" in
      linux)
        if [ -f "${TMP_DIR}/previous/service.definition" ]; then
          cp -p "${TMP_DIR}/previous/service.definition" "/etc/systemd/system/${SERVICE_NAME}.service" || warn "failed to restore previous service definition"
          systemctl daemon-reload || warn "failed to reload previous service definition"
        fi
        systemctl restart "$SERVICE_NAME" || warn "failed to restart previous service"
        ;;
      darwin)
        rollback_plist="/Library/LaunchDaemons/com.gofxq.${SERVICE_NAME}.plist"
        launchctl bootout system "$rollback_plist" >/dev/null 2>&1 || true
        if [ -f "${TMP_DIR}/previous/service.definition" ]; then
          cp -p "${TMP_DIR}/previous/service.definition" "$rollback_plist" || warn "failed to restore previous service definition"
        fi
        launchctl bootstrap system "$rollback_plist" || warn "failed to restore previous service"
        ;;
    esac
  fi
  if [ -n "${TMP_DIR:-}" ]; then rm -rf "$TMP_DIR"; fi
}

make_tmpdir() {
  if TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gaoming-agent.XXXXXX" 2>/dev/null)"; then
    :
  elif TMP_DIR="$(mktemp -d -t gaoming-agent 2>/dev/null)"; then
    :
  else
    die "failed to create temporary directory"
  fi
  trap 'cleanup "$?"' EXIT
  trap 'exit 1' INT TERM HUP
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
  curl -fsSL --connect-timeout 15 --max-time 300 --retry 3 "${base_url}/${ASSET}" -o "${TMP_DIR}/${ASSET}"
  curl -fsSL --connect-timeout 15 --max-time 60 --retry 3 "${base_url}/checksums.txt" -o "${TMP_DIR}/checksums.txt"
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
  printf '%s' "$1" | awk 'BEGIN { ORS="" } { if (NR > 1) printf "\\n"; gsub(/\\/, "\\\\"); gsub(/"/, "\\\""); gsub(/\r/, "\\r"); gsub(/\t/, "\\t"); printf "%s", $0 }'
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

normalize_master_api_base() {
  base="${1%/}"
  case "$base" in
    */master/api/v1) printf '%s\n' "$base" ;;
    */master) printf '%s/api/v1\n' "$base" ;;
    *) printf '%s/master/api/v1\n' "$base" ;;
  esac
}

extract_json_string() {
  key="$1"
  awk -v key="$key" '
    BEGIN {
      pattern = "\"" key "\"[[:space:]]*:[[:space:]]*\""
    }
    match($0, pattern "[^\"]*") {
      value = substr($0, RSTART, RLENGTH)
      sub("^.*:[[:space:]]*\"", "", value)
      print value
      exit
    }
  '
}

generate_local_tenant_code() {
  if command -v od >/dev/null 2>&1; then
    suffix="$(od -An -N6 -tx1 /dev/urandom 2>/dev/null | tr -d ' \n')"
  else
    suffix="$(date +%s)"
  fi
  printf 'tenant-local-%s\n' "$suffix"
}

request_tenant_code_from_master() {
  api_base="$(normalize_master_api_base "$MASTER_API_URL")"
  body="$(curl -fsS --connect-timeout 5 --max-time 15 -X POST "${api_base}/install/tenant" 2>/dev/null || true)"
  [ -n "$body" ] || return 1

  tenant_code="$(printf '%s\n' "$body" | extract_json_string "tenant_code")"
  [ -n "$tenant_code" ] || return 1
  printf '%s\n' "$tenant_code"
}

resolve_agent_tenant() {
  if [ -n "$AGENT_TENANT" ]; then
    return 0
  fi

  tenant_code="$(request_tenant_code_from_master || true)"
  if [ -n "$tenant_code" ]; then
    AGENT_TENANT="$tenant_code"
    log "allocated tenant_code from master-api: ${AGENT_TENANT}"
    return 0
  fi

  AGENT_TENANT="$(generate_local_tenant_code)"
  warn "failed to allocate tenant_code from master-api; using local tenant_code: ${AGENT_TENANT}"
}

build_dashboard_url() {
  base_url="$1"
  tenant_code="$2"
  printf '%s/%s\n' "${base_url%/}" "$tenant_code"
}

build_hosts_api_url() {
  base_url="$1"
  tenant_code="$2"
  printf '%s/master/api/v1/hosts?tenant=%s\n' "${base_url%/}" "$tenant_code"
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

extract_binary() {
  tar -xzf "${TMP_DIR}/${ASSET}" -C "$TMP_DIR" gaoming-agent
  [ -f "${TMP_DIR}/gaoming-agent" ] && [ ! -L "${TMP_DIR}/gaoming-agent" ] || die "gaoming-agent not found in archive or is a symbolic link"
}

install_files() {
  mkdir -p "$INSTALL_DIR"
  install -m 0755 "${TMP_DIR}/gaoming-agent" "${INSTALL_DIR}/.gaoming-agent.new"
  mv -f "${INSTALL_DIR}/.gaoming-agent.new" "${INSTALL_DIR}/gaoming-agent"
  if [ "$INSTALL_MODE" = "reinstall" ]; then
    render_config | write_file "${INSTALL_DIR}/agent-config.yaml" 0600
    return 0
  fi

  [ -f "${INSTALL_DIR}/agent-config.yaml" ] || die "installed config not found: ${INSTALL_DIR}/agent-config.yaml"
}

install_linux_service() {
  need_cmd systemctl
  ensure_linux_identity
  chown root:"${SERVICE_GROUP}" "$INSTALL_DIR" "$INSTALL_DIR/gaoming-agent"
  chmod 0750 "$INSTALL_DIR"
  chown "${SERVICE_USER}:${SERVICE_GROUP}" "${INSTALL_DIR}/agent-config.yaml"
  chmod 0600 "${INSTALL_DIR}/agent-config.yaml"
  mkdir -p "${INSTALL_DIR}/logs"
  chown -R "${SERVICE_USER}:${SERVICE_GROUP}" "${INSTALL_DIR}/logs"
  render_systemd_unit | write_file "/etc/systemd/system/${SERVICE_NAME}.service" 0644 root root
  systemctl daemon-reload
  systemctl enable "$SERVICE_NAME"
  systemctl restart "$SERVICE_NAME"
  systemctl is-active --quiet "$SERVICE_NAME" || die "service failed to start: $SERVICE_NAME"
}

install_darwin_service() {
  need_cmd launchctl
  plist="/Library/LaunchDaemons/com.gofxq.${SERVICE_NAME}.plist"
  chown -R root:wheel "$INSTALL_DIR"
  render_launchd_plist | write_file "$plist" 0644 root wheel
  launchctl bootout system "$plist" >/dev/null 2>&1 || true
  launchctl bootstrap system "$plist"
  chmod 0600 "${INSTALL_DIR}/agent-config.yaml"
  launchctl enable "system/com.gofxq.${SERVICE_NAME}"
  launchctl kickstart -k "system/com.gofxq.${SERVICE_NAME}"
}

install_service() {
  case "$OS" in
    linux)  install_linux_service ;;
    darwin) install_darwin_service ;;
    *)      die "unsupported OS: $OS" ;;
  esac
}

print_summary() {
  config_path="${INSTALL_DIR}/agent-config.yaml"
  summary_web_base_url="$(read_config_value "master_api_url" "$config_path" 2>/dev/null || true)"
  summary_ingest_addr="$(read_config_value "ingest_gateway_grpc_addr" "$config_path" 2>/dev/null || true)"
  tenant_code="$(wait_for_tenant_code || true)"

  if [ -z "$summary_web_base_url" ]; then
    summary_web_base_url="$WEB_BASE_URL"
  fi

  if [ -z "$summary_ingest_addr" ]; then
    summary_ingest_addr="$INGEST_GATEWAY_GRPC_ADDR"
  fi

  log "installed ${SERVICE_NAME} to ${INSTALL_DIR}"
  log "install mode: ${INSTALL_MODE}"
  log "config: ${config_path}"
  log "ingest_grpc_addr: ${summary_ingest_addr}"
  if [ -n "$tenant_code" ]; then
    log "tenant_code: ${tenant_code}"
    log "dashboard: $(build_dashboard_url "$summary_web_base_url" "$tenant_code")"
    log "hosts api: $(build_hosts_api_url "$summary_web_base_url" "$tenant_code")"
    return
  fi

  if [ -n "$AGENT_TENANT" ]; then
    log "tenant_code: ${AGENT_TENANT}"
    log "dashboard: $(build_dashboard_url "$summary_web_base_url" "${AGENT_TENANT}")"
    log "hosts api: $(build_hosts_api_url "$summary_web_base_url" "${AGENT_TENANT}")"
    return
  fi

  log "tenant_code: <missing>"
  log "tenant_code allocation did not complete; update agent-config.yaml before starting the agent"
}

main() {
  parse_args "$@"
  prepare_platform
  require_root
  need_cmd curl
  need_cmd tar
  need_cmd awk
  need_cmd sed
  need_cmd grep
  need_cmd install
  need_cmd mktemp
  case "$OS" in
    linux) need_cmd systemctl; [ -d /run/systemd/system ] || die "Linux requires a running systemd service manager" ;;
    darwin) need_cmd launchctl ;;
  esac
  validate_common_inputs
  load_existing_config_defaults
  choose_install_mode
  if [ "$INSTALL_MODE" = "reinstall" ]; then
    prompt_install_inputs
    if [ -z "$MASTER_API_URL" ]; then
      MASTER_API_URL="$WEB_BASE_URL"
    fi
  fi
  validate_common_inputs
  if [ "$INSTALL_MODE" = "reinstall" ]; then
    validate_config_inputs
    resolve_agent_tenant
  fi
  make_tmpdir
  download_assets
  verify_checksum
  extract_binary
  # Older installations allowed the service user to write the install directory.
  # Lock it before staging files, then recheck entries under that locked parent.
  mkdir -p "$INSTALL_DIR"
  chown root "$INSTALL_DIR"
  chmod 0755 "$INSTALL_DIR"
  validate_common_inputs
  if [ -f "${INSTALL_DIR}/gaoming-agent" ] && [ -f "${INSTALL_DIR}/agent-config.yaml" ]; then
    mkdir "${TMP_DIR}/previous"
    cp -p "${INSTALL_DIR}/gaoming-agent" "${TMP_DIR}/previous/gaoming-agent"
    cp -p "${INSTALL_DIR}/agent-config.yaml" "${TMP_DIR}/previous/agent-config.yaml"
    case "$OS" in
      linux) previous_definition="/etc/systemd/system/${SERVICE_NAME}.service" ;;
      darwin) previous_definition="/Library/LaunchDaemons/com.gofxq.${SERVICE_NAME}.plist" ;;
    esac
    if [ -f "$previous_definition" ]; then cp -p "$previous_definition" "${TMP_DIR}/previous/service.definition"; fi
    UPGRADE_IN_PROGRESS=1
  fi
  case "$OS" in
    linux) if systemctl cat "$SERVICE_NAME" >/dev/null 2>&1; then systemctl stop "$SERVICE_NAME"; fi ;;
    darwin) if launchctl print "system/com.gofxq.${SERVICE_NAME}" >/dev/null 2>&1; then launchctl bootout system "/Library/LaunchDaemons/com.gofxq.${SERVICE_NAME}.plist"; fi ;;
  esac
  install_files
  install_service
  UPGRADE_IN_PROGRESS=0
  print_summary
}

main "$@"
