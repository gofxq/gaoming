#!/usr/bin/env sh
set -eu

MASTER_URL="${MASTER_URL:-http://127.0.0.1:8080}"
INGEST_URL="${INGEST_URL:-http://127.0.0.1:8090}"

echo "waiting for master-api..."
i=0
until curl -fsS "${MASTER_URL}/healthz" >/dev/null 2>&1; do
  i=$((i + 1))
  if [ "$i" -ge 30 ]; then
    echo "master-api did not become healthy in time" >&2
    exit 1
  fi
  sleep 2
done

echo "waiting for ingest-gateway..."
i=0
until curl -fsS "${INGEST_URL}/healthz" >/dev/null 2>&1; do
  i=$((i + 1))
  if [ "$i" -ge 30 ]; then
    echo "ingest-gateway did not become healthy in time" >&2
    exit 1
  fi
  sleep 2
done

echo "checking ingest counters..."
curl -fsS "${INGEST_URL}/debug/counters"
echo
echo "backend smoke test passed"
