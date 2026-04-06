#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

master_addr="${MASTER_API_HTTP_ADDR:-:8080}"
ingest_addr="${INGEST_GATEWAY_HTTP_ADDR:-:8090}"
master_port="${master_addr##*:}"
ingest_port="${ingest_addr##*:}"
master_health_url="http://127.0.0.1:${master_port}/master/healthz"
ingest_health_url="http://127.0.0.1:${ingest_port}/ingest/healthz"

kill_go_run() {
  pkill -f "$1" || true
}

require_port_free() {
  local port="$1"
  local output

  output="$(lsof -nP -iTCP:${port} -sTCP:LISTEN 2>/dev/null || true)"
  if [ -n "$output" ]; then
    echo "port ${port} is already in use:"
    echo "$output"
    exit 1
  fi
}

wait_for_http() {
  local name="$1"
  local url="$2"
  local attempts="${3:-40}"
  local sleep_sec="${4:-0.5}"
  local i

  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      echo "${name} is ready: ${url}"
      return 0
    fi
    sleep "$sleep_sec"
  done

  echo "timed out waiting for ${name}: ${url}"
  exit 1
}

# 先杀掉当前脚本会拉起的相关 go run 进程
kill_go_run "services/master-api/cmd/server"
kill_go_run "services/ingest-gateway/cmd/server"
kill_go_run "services/core-worker/cmd/worker"
kill_go_run "services/probe-worker/cmd/worker"
kill_go_run "agent/daemon/cmd/agent"

sleep 1

require_port_free "$master_port"
require_port_free "$ingest_port"

go run ./services/master-api/cmd/server &
go run ./services/ingest-gateway/cmd/server &

wait_for_http "master-api" "$master_health_url"
wait_for_http "ingest-gateway" "$ingest_health_url"

go run ./services/core-worker/cmd/worker &
go run ./services/probe-worker/cmd/worker &
go run ./agent/daemon/cmd/agent &

wait
