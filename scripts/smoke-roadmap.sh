#!/bin/sh
set -eu
# Self-contained integration environment; no user configuration or existing
# volumes are read, and only this randomly named Compose project is removed.
task_repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
task_dir=$(mktemp -d)
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
cat > "$task_dir/master.yml" <<'EOF'
http_addr: "0.0.0.0:8080"
postgres_dsn: "postgres://gaoming:smoke-only-password@postgres:5432/gaoming?sslmode=disable"
redis_addr: "redis:6379"
tenant_code: "default"
session_cookie_secure: false
EOF
cat > "$task_dir/ingest.yml" <<'EOF'
http_addr: "0.0.0.0:8090"
grpc_addr: "0.0.0.0:8091"
postgres_dsn: "postgres://gaoming:smoke-only-password@postgres:5432/gaoming?sslmode=disable"
redis_addr: "redis:6379"
tenant_code: "default"
EOF
cat > "$task_dir/agent.yml" <<'EOF'
master_api_url: "http://master:8080"
ingest_gateway_grpc_addr: "ingest:8091"
tenant_code: "default"
region: "smoke"
loop_interval_sec: 1
EOF
cat > "$task_dir/compose.yml" <<EOF
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: gaoming
      POSTGRES_PASSWORD: smoke-only-password
      POSTGRES_DB: gaoming
    ports: ["0.0.0.0::5432"]
    volumes: ["database:/var/lib/postgresql/data"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U gaoming -d gaoming"]
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
  migrate:
    build:
      context: "$task_repo"
      dockerfile: deployments/docker/Dockerfile
      args: {SERVICE_PATH: ./services/master-api/cmd/admin}
    entrypoint: ["/app/admin", "migrate"]
    volumes: ["$task_dir/master.yml:/app/config/master-api.yml:ro"]
    depends_on:
      postgres: {condition: service_healthy}
  master:
    build:
      context: "$task_repo"
      dockerfile: deployments/docker/Dockerfile
      args: {SERVICE_PATH: ./services/master-api/cmd/server}
    volumes: ["$task_dir/master.yml:/app/config/master-api.yml:ro"]
    ports: ["0.0.0.0::8080"]
    depends_on:
      migrate: {condition: service_completed_successfully}
      redis: {condition: service_healthy}
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://127.0.0.1:8080/master/healthz >/dev/null"]
      interval: 1s
      timeout: 3s
      retries: 30
  ingest:
    build:
      context: "$task_repo"
      dockerfile: deployments/docker/Dockerfile
      args: {SERVICE_PATH: ./services/ingest-gateway/cmd/server}
    volumes: ["$task_dir/ingest.yml:/app/config/ingest-gateway.yml:ro"]
    ports: ["0.0.0.0::8090"]
    depends_on:
      migrate: {condition: service_completed_successfully}
      redis: {condition: service_healthy}
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://127.0.0.1:8090/ingest/healthz >/dev/null"]
      interval: 1s
      timeout: 3s
      retries: 30
  agent:
    build:
      context: "$task_repo"
      dockerfile: deployments/docker/Dockerfile
      args: {SERVICE_PATH: ./agent/daemon/cmd/agent}
    volumes: ["$task_dir/agent.yml:/app/agent-config.yaml:ro"]
    depends_on:
      master: {condition: service_healthy}
      ingest: {condition: service_healthy}
volumes:
  database: {}
EOF
compose config >/dev/null
compose up --build -d --wait --wait-timeout 90 postgres redis master ingest agent
task_pg_port=$(compose port postgres 5432 | sed 's/.*://')
task_master_port=$(compose port master 8080 | sed 's/.*://')
task_ingest_port=$(compose port ingest 8090 | sed 's/.*://')
cd "$task_repo"
umask 077
# The test administrator is created via stdin. Password and session never
# become process arguments, logged output or persistent repository files.
python3 - "$task_dir/login.json" <<'PY'
import json, secrets, sys
with open(sys.argv[1], 'w') as out:
    json.dump({'tenant_code': 'default', 'username': 'smoke-admin', 'password': secrets.token_urlsafe(24)}, out)
PY
python3 - "$task_dir/login.json" <<'PY' | compose exec -T master /app/admin -tenant default -username smoke-admin -display-name 'Smoke Administrator' -password-stdin create-admin
import json, sys
print(json.load(open(sys.argv[1]))['password'])
PY
task_master_origin="http://127.0.0.1:$task_master_port"
curl -fsS -c "$task_dir/session.cookies" -H 'Content-Type: application/json' -H "Origin: $task_master_origin" --data-binary "@$task_dir/login.json" "$task_master_origin/master/api/v1/auth/login" > "$task_dir/session.json"
rm -f "$task_dir/login.json" "$task_dir/session.json"
task_anon_status=$(curl -s -o /dev/null -w '%{http_code}' "$task_master_origin/master/api/v1/hosts")
[ "$task_anon_status" = 401 ] || { echo 'anonymous host read was not rejected' >&2; exit 1; }
TEST_POSTGRES_DSN="postgres://gaoming:smoke-only-password@127.0.0.1:$task_pg_port/gaoming?sslmode=disable" make migration-test
MASTER_URL="http://127.0.0.1:$task_master_port/master" INGEST_URL="http://127.0.0.1:$task_ingest_port/ingest" make smoke
MASTER_URL="$task_master_origin" INGEST_URL="http://127.0.0.1:$task_ingest_port" TENANT=default SESSION_COOKIE_FILE="$task_dir/session.cookies" make smoke-agent
echo 'roadmap infrastructure smoke passed; product stages require their own acceptance'
