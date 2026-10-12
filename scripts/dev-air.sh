#!/usr/bin/env sh
set -eu
GO="${GO:-go}"
AIR="${AIR:-air}"
YARN="${YARN:-yarn}"
SERVER_CONFIG="${SERVER_CONFIG:-config/server.yml}"
if ! command -v "$AIR" >/dev/null 2>&1; then
  if [ -x "$("$GO" env GOPATH)/bin/air" ]; then
    AIR="$("$GO" env GOPATH)/bin/air"
  else
    echo 'Air is required; install it before running make air.' >&2
    exit 127
  fi
fi
[ -f "$SERVER_CONFIG" ] || { echo 'Run make dev-init and configure native PostgreSQL/Redis first.' >&2; exit 1; }
mkdir -p .tmp/air/bin .tmp/air/config .tmp/local/logs
# Keep generated TOML valid; tool and config paths are passed via a shell script.
cat > .tmp/air/run-server.sh <<'RUN'
#!/usr/bin/env sh
exec ./.tmp/air/bin/server -config "$SERVER_CONFIG"
RUN
cat > .tmp/air/build-server.sh <<'BUILD'
#!/usr/bin/env sh
exec "$GO" build -o ./.tmp/air/bin/server ./services/server/cmd/server
BUILD
chmod +x .tmp/air/run-server.sh .tmp/air/build-server.sh
cat > .tmp/air/config/server.toml <<EOF_CONFIG
root = "."
tmp_dir = ".tmp/air/tmp/server"
[build]
cmd = "./.tmp/air/build-server.sh"
entrypoint = ["./.tmp/air/run-server.sh"]
include_ext = ["go", "yml", "yaml"]
exclude_dir = [".git", ".tmp", "web", "node_modules"]
delay = 1000
stop_on_error = true
send_interrupt = true
kill_delay = "500ms"
[log]
time = true
EOF_CONFIG
# The same supervisor owns the Air process group and frontend descendants.
export SERVER_CONFIG GO
DEV_BACKEND="$AIR" AIR_CONFIG=".tmp/air/config/server.toml" YARN="$YARN" node ./scripts/dev-local.mjs
