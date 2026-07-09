#!/usr/bin/env sh
set -eu

GO="${GO:-go}"
AIR="${AIR:-air}"
YARN="${YARN:-yarn}"

LOG_DIR=".tmp/air/logs"
BIN_DIR=".tmp/air/bin"
CONFIG_DIR=".tmp/air/config"
mkdir -p "$LOG_DIR" "$BIN_DIR" "$CONFIG_DIR"

if ! command -v "$AIR" >/dev/null 2>&1; then
	if [ -x "$("$GO" env GOPATH)/bin/air" ]; then
		AIR="$("$GO" env GOPATH)/bin/air"
	else
		echo "air is required. Install it with: go install github.com/air-verse/air@latest" >&2
		exit 127
	fi
fi

require_file() {
	if [ ! -f "$1" ]; then
		echo "missing local config: $1" >&2
		echo "config/ is intentionally git-ignored; create local config files before running make air." >&2
		exit 1
	fi
}

require_file "config/master-api.yml"
require_file "config/ingest-gateway.yml"
require_file "config/core-worker.yml"
require_file "config/probe-worker.yml"
require_file "config/agent-1.yml"
require_file "config/agent-2.yml"
require_file "config/agent-3.yml"

pids=""

cleanup() {
	trap - INT TERM EXIT
	for pid in $pids; do
		kill "$pid" >/dev/null 2>&1 || true
	done
	wait >/dev/null 2>&1 || true
}

trap cleanup INT TERM EXIT

write_air_config() {
	name="$1"
	pkg="$2"
	config="$CONFIG_DIR/$name.toml"

	cat >"$config" <<EOF
root = "."
tmp_dir = ".tmp/air/tmp/$name"

[build]
cmd = "$GO build -o ./$BIN_DIR/$name $pkg"
bin = "./$BIN_DIR/$name"
include_ext = ["go", "yml", "yaml"]
exclude_dir = [".git", ".tmp", "web", "node_modules"]
delay = 1000
stop_on_error = true
send_interrupt = true
kill_delay = "500ms"

[log]
time = true
EOF

	echo "$config"
}

start_air() {
	name="$1"
	pkg="$2"
	log="$LOG_DIR/$name.log"
	config="$(write_air_config "$name" "$pkg")"

	echo "starting $name with air, log: $log"
	"$AIR" -c "$config" >"$log" 2>&1 &
	pids="$pids $!"
}

start_agent() {
	name="$1"
	config_path="$2"
	log="$LOG_DIR/$name.log"
	air_config="$(write_air_config "$name" "./agent/daemon/cmd/agent")"

	echo "starting $name with air, config: $config_path, log: $log"
	GAOMING_AGENT_CONFIG="$config_path" "$AIR" -c "$air_config" >"$log" 2>&1 &
	pids="$pids $!"
}

start_air "master-api" "./services/master-api/cmd/server"
start_air "ingest-gateway" "./services/ingest-gateway/cmd/server"
start_air "core-worker" "./services/core-worker/cmd/worker"
start_air "probe-worker" "./services/probe-worker/cmd/worker"

start_agent "agent-1" "config/agent-1.yml"
start_agent "agent-2" "config/agent-2.yml"
start_agent "agent-3" "config/agent-3.yml"

echo "starting web with yarn dev, log: $LOG_DIR/web.log"
(
	cd web
	VITE_PROXY_TARGET=http://localhost:8080 "$YARN" dev
) >"$LOG_DIR/web.log" 2>&1 &
pids="$pids $!"

echo "all processes started. Frontend: http://127.0.0.1:5173/default"
echo "streaming logs; press Ctrl-C to stop everything."
tail -n 40 -F "$LOG_DIR"/*.log &
pids="$pids $!"

wait
