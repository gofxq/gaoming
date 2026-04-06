# 先杀掉所有相关 go run 进程
pkill -f "services/master-api/cmd/server" || true
pkill -f "services/ingest-gateway/cmd/server" || true
pkill -f "services/core-worker/cmd/worker" || true
pkill -f "services/probe-worker/cmd/worker" || true
pkill -f "agent/daemon/cmd/agent" || true

# 等一下确保端口释放
sleep 1

# 重新启动
set -a; source .env;

go run ./services/master-api/cmd/server &
go run ./services/ingest-gateway/cmd/server &
go run ./services/core-worker/cmd/worker &
go run ./services/probe-worker/cmd/worker &
go run ./agent/daemon/cmd/agent &

cd web && VITE_PROXY_TARGET=http://localhost:8080/ yarn dev
wait