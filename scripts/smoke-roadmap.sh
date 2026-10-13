#!/bin/sh
set -eu
umask 077
# Self-contained integration environment; no user configuration or existing
# volumes are read, and only this randomly named Compose project is removed.
task_repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
task_dir=$(mktemp -d)
task_admin_password=$(python3 -c 'import secrets; print(secrets.token_urlsafe(24))')
task_project="gaoming-roadmap-$(basename "$task_dir" | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9')"
compose() { docker compose -p "$task_project" -f "$task_dir/compose.yml" "$@"; }
cleanup() {
  task_exit=$?
  if [ "$task_exit" -ne 0 ]; then compose logs --tail=80 >&2 || true; fi
  compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -rf "$task_dir"
  exit "$task_exit"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM HUP
cat > "$task_dir/server.yml" <<'EOF'
http_addr: "0.0.0.0:8080"
grpc_addr: "0.0.0.0:8091"
probe_interval_sec: 1
redis_namespace: gaoming-smoke
postgres_dsn: "postgres://gaoming:smoke-only-password@postgres:5432/gaoming?sslmode=disable"
redis_addr: "redis:6379"
tenant_code: "default"
session_cookie_secure: false
EOF
cat > "$task_dir/agent.yml" <<'EOF'
master_api_url: "http://server:8080"
ingest_gateway_grpc_addr: "server:8091"
tenant_code: "default"
region: "smoke"
loop_interval_sec: 1
EOF
# Container UID 10001 must read these smoke-only configuration files.
# The containing temporary directory remains private.
chmod 644 "$task_dir/server.yml" "$task_dir/agent.yml"
cat > "$task_dir/compose.yml" <<EOF
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: gaoming
      POSTGRES_PASSWORD: smoke-only-password
      POSTGRES_DB: gaoming
    ports: ["127.0.0.1::5432"]
    volumes: ["database:/var/lib/postgresql/data", "$task_repo/deployments/sql/init.sql:/docker-entrypoint-initdb.d/001-init.sql:ro"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -h 127.0.0.1 -U gaoming -d gaoming"]
      interval: 1s
      timeout: 3s
      retries: 30
  redis:
    image: redis:7-alpine
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 1s
      timeout: 3s
      retries: 30
  server:
    environment:
      GAOMING_ADMIN_PASSWORD: "$task_admin_password"
    build:
      context: "$task_repo"
      dockerfile: deployments/docker/Dockerfile
      args: {SERVICE_PATH: ./server}
    volumes: ["$task_dir/server.yml:/app/config/server.yml:ro"]
    ports: ["127.0.0.1::8080"]
    depends_on:
      postgres: {condition: service_healthy}
      redis: {condition: service_healthy}
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://127.0.0.1:8080/master/healthz >/dev/null"]
      interval: 1s
      timeout: 3s
      retries: 30
  agent:
    build:
      context: "$task_repo"
      dockerfile: deployments/docker/Dockerfile
      target: agent
      args: {SERVICE_PATH: ./agent}
    volumes: ["$task_dir/agent.yml:/app/agent-config.yaml:ro"]
    depends_on:
      server: {condition: service_healthy}
volumes:
  database: {}
EOF
compose config >/dev/null
compose up --build -d --wait --wait-timeout 90 postgres redis server agent
task_master_port=$(compose port server 8080 | sed 's/.*://')
cd "$task_repo"
umask 077
# Private login bodies keep the startup password out of arguments and logs.
printf '%s' "$task_admin_password" | python3 -c 'import json, sys; json.dump({"tenant_code": "default", "username": "admin", "password": sys.stdin.read()}, open(sys.argv[1], "w"))' "$task_dir/login.json"
task_master_origin="http://127.0.0.1:$task_master_port"
curl -fsS -c "$task_dir/session.cookies" -H 'Content-Type: application/json' -H "Origin: $task_master_origin" --data-binary "@$task_dir/login.json" "$task_master_origin/master/api/v1/auth/login" > "$task_dir/session.json"
rm -f "$task_dir/login.json" "$task_dir/session.json"
# Production UI, SPA deep links and API share the same port.
curl -fsS "$task_master_origin/" > "$task_dir/index.html"
curl -fsS "$task_master_origin/default/login" > "$task_dir/login.html"
cmp "$task_dir/index.html" "$task_dir/login.html"
python3 - "$task_dir/index.html" > "$task_dir/assets.txt" <<'PYASSET'
import re, sys
for asset in sorted(set(re.findall(r'(?:src|href)="(/assets/[^" ]+)"', open(sys.argv[1]).read()))):
    print(asset)
PYASSET
[ -s "$task_dir/assets.txt" ] || { echo 'built Web assets missing from HTML' >&2; exit 1; }
while IFS= read -r task_asset; do curl -fsS "$task_master_origin$task_asset" > /dev/null; done < "$task_dir/assets.txt"
[ "$(curl -s -o /dev/null -w '%{http_code}' "$task_master_origin/assets/missing.js")" = 404 ]
[ "$(curl -s -o /dev/null -w '%{http_code}' "$task_master_origin/.env")" = 404 ]
task_anon_status=$(curl -s -o /dev/null -w '%{http_code}' "$task_master_origin/master/api/v1/hosts")
[ "$task_anon_status" = 401 ] || { echo 'anonymous host read was not rejected' >&2; exit 1; }
MASTER_URL="http://127.0.0.1:$task_master_port/master" INGEST_URL="http://127.0.0.1:$task_master_port/ingest" make smoke
MASTER_URL="$task_master_origin" INGEST_URL="http://127.0.0.1:$task_master_port" TENANT=default SESSION_COOKIE_FILE="$task_dir/session.cookies" make smoke-agent
echo 'roadmap infrastructure smoke passed; product stages require their own acceptance'
