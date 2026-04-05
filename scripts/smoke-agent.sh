#!/usr/bin/env sh
set -eu

MASTER_URL="${MASTER_URL:-http://127.0.0.1:8080}"

echo "waiting for host agent registration..."
i=0
until curl -fsS "${MASTER_URL}/api/v1/hosts" | grep -q '"host_uid"'; do
  i=$((i + 1))
  if [ "$i" -ge 30 ]; then
    echo "no host agent registered in time" >&2
    curl -fsS "${MASTER_URL}/api/v1/hosts" || true
    exit 1
  fi
  sleep 2
done

echo "checking host list..."
curl -fsS "${MASTER_URL}/api/v1/hosts"
echo
echo "host-agent smoke test passed"
