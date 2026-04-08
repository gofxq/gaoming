#!/usr/bin/env sh
set -eu

MASTER_URL="${MASTER_URL:-http://127.0.0.1:8080}"
INGEST_URL="${INGEST_URL:-http://127.0.0.1:8090}"
TENANT="${TENANT:-}"
EXPECT_METRICS="${EXPECT_METRICS:-1}"
RETRIES="${RETRIES:-30}"
SLEEP_SEC="${SLEEP_SEC:-2}"

normalize_master_base() {
  base="${1%/}"
  case "$base" in
    */master/api/v1) printf '%s\n' "${base%/api/v1}" ;;
    */master) printf '%s\n' "$base" ;;
    *) printf '%s/master\n' "$base" ;;
  esac
}

normalize_ingest_base() {
  base="${1%/}"
  case "$base" in
    */ingest/api/v1) printf '%s\n' "${base%/api/v1}" ;;
    */ingest) printf '%s\n' "$base" ;;
    *) printf '%s/ingest\n' "$base" ;;
  esac
}

metric_batches_from_body() {
  printf '%s' "$1" | tr -d '\n' | sed -n 's/.*"metric_batches"[[:space:]]*:[[:space:]]*\([0-9][0-9]*\).*/\1/p'
}

MASTER_BASE="$(normalize_master_base "$MASTER_URL")"
INGEST_BASE="$(normalize_ingest_base "$INGEST_URL")"
HOSTS_URL="${MASTER_BASE}/api/v1/hosts"
if [ -n "$TENANT" ]; then
  HOSTS_URL="${HOSTS_URL}?tenant=${TENANT}"
fi
COUNTERS_URL="${INGEST_BASE}/debug/counters"

baseline_metric_batches=0
if [ "$EXPECT_METRICS" = "1" ]; then
  echo "capturing baseline ingest counters..."
  counters_body="$(curl -fsS "$COUNTERS_URL")" || {
    echo "failed to read ingest counters from ${COUNTERS_URL}" >&2
    exit 1
  }
  baseline_metric_batches="$(metric_batches_from_body "$counters_body")"
  [ -n "$baseline_metric_batches" ] || {
    echo "failed to parse metric_batches from: $counters_body" >&2
    exit 1
  }
fi

echo "waiting for host agent registration..."
i=0
until hosts_body="$(curl -fsS "$HOSTS_URL")" && printf '%s' "$hosts_body" | grep -q '"host_uid"'; do
  i=$((i + 1))
  if [ "$i" -ge "$RETRIES" ]; then
    echo "no host agent registered in time" >&2
    curl -fsS "$HOSTS_URL" || true
    exit 1
  fi
  sleep "$SLEEP_SEC"
done

echo "checking host list..."
printf '%s\n' "$hosts_body"
echo

if [ "$EXPECT_METRICS" = "1" ]; then
  echo "waiting for ingest metric_batches to grow..."
  i=0
  while :; do
    counters_body="$(curl -fsS "$COUNTERS_URL")" || {
      echo "failed to read ingest counters from ${COUNTERS_URL}" >&2
      exit 1
    }
    current_metric_batches="$(metric_batches_from_body "$counters_body")"
    [ -n "$current_metric_batches" ] || {
      echo "failed to parse metric_batches from: $counters_body" >&2
      exit 1
    }
    if [ "$current_metric_batches" -gt "$baseline_metric_batches" ]; then
      echo "ingest counters:"
      printf '%s\n' "$counters_body"
      break
    fi

    i=$((i + 1))
    if [ "$i" -ge "$RETRIES" ]; then
      echo "metric_batches did not grow in time" >&2
      printf '%s\n' "$counters_body" >&2
      exit 1
    fi
    sleep "$SLEEP_SEC"
  done
fi

echo "host-agent smoke test passed"
