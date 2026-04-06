#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
SQL_FILE="${SQL_FILE:-${SCRIPT_DIR}/sql/upgrade-host-status-current-metrics.sql}"
DOCKER_COMPOSE_BIN="${DOCKER_COMPOSE_BIN:-docker compose}"
POSTGRES_SERVICE="${POSTGRES_SERVICE:-postgres}"
POSTGRES_USER="${POSTGRES_USER:-gaoming}"
POSTGRES_DB="${POSTGRES_DB:-gaoming}"

usage() {
  cat <<EOF
usage: update-db.sh [options]

Options:
  --sql-file <path>              SQL file to apply, default: ${SQL_FILE}
  --postgres-service <name>      Docker Compose postgres service, default: ${POSTGRES_SERVICE}
  --postgres-user <name>         Postgres user, default: ${POSTGRES_USER}
  --postgres-db <name>           Postgres database, default: ${POSTGRES_DB}
  --help                         Show this help
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --sql-file)
      SQL_FILE="$2"
      shift 2
      ;;
    --postgres-service)
      POSTGRES_SERVICE="$2"
      shift 2
      ;;
    --postgres-user)
      POSTGRES_USER="$2"
      shift 2
      ;;
    --postgres-db)
      POSTGRES_DB="$2"
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

if [ ! -f "$SQL_FILE" ]; then
  echo "sql file not found: $SQL_FILE" >&2
  exit 1
fi

echo "applying ${SQL_FILE} to ${POSTGRES_SERVICE}/${POSTGRES_DB}"
sh -c "${DOCKER_COMPOSE_BIN} exec -T ${POSTGRES_SERVICE} psql -v ON_ERROR_STOP=1 -U ${POSTGRES_USER} -d ${POSTGRES_DB} -f -" < "$SQL_FILE"
echo "database update complete"
