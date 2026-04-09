# 本地启动与验证

## 推荐联调方式

- 后端与依赖走 Docker
- Agent 跑宿主机
- Web 跑 `Vite`

对应命令：

```bash
make docker-up
make smoke
make run-agent
make smoke-agent
make web-dev WEB_API_ORIGIN=http://127.0.0.1:8080
```

页面入口：

- `http://127.0.0.1:5173/default`
- `http://127.0.0.1:5173/default/pwa`

## 后端健康检查

```bash
curl http://127.0.0.1:8080/master/healthz
curl "http://127.0.0.1:8080/master/api/v1/hosts?tenant=default"
curl http://127.0.0.1:8090/ingest/healthz
curl http://127.0.0.1:8090/ingest/debug/counters
curl -v 127.0.0.1:8091
```

如果 `probe-worker` 已在运行，`/ingest/debug/counters` 里的 `probe_reports` 会持续增长。

## Agent 本地运行

```bash
make run-agent
```

Agent 当前配置来源优先级是：

1. 环境变量
2. 当前目录下 `.env`
3. 当前目录下 `agent-config.yaml`
4. 代码默认值

几个常用变量：

- `MASTER_API_URL`
- `INGEST_GATEWAY_GRPC_ADDR`
- `AGENT_REGION`
- `AGENT_ENV`
- `AGENT_ROLE`
- `AGENT_TENANT`
- `AGENT_LOOP_INTERVAL_SEC`

如果要从宿主机测试 gRPC 上报，可以直接这样跑：

```bash
MASTER_API_URL=http://127.0.0.1:8080 \
INGEST_GATEWAY_GRPC_ADDR=127.0.0.1:8091 \
AGENT_CONFIG_PATH=/tmp/gaoming-agent-grpc.yaml \
make run-agent
```

然后再执行：

```bash
TENANT=<agent-config.yaml 里的 tenant_code> \
MASTER_URL=http://127.0.0.1:8080 \
INGEST_URL=http://127.0.0.1:8090 \
make smoke-agent
```

第一次注册成功后，如果服务端返回了 `tenant_code`，Agent 会把它持久化回 `agent-config.yaml`。

`make smoke-agent` 会同时检查：

- `master-api` 是否已经能查到主机
- `ingest-gateway` 的 `metric_batches` 是否继续增长

它支持这些环境变量：

- `MASTER_URL`，默认 `http://127.0.0.1:8080`
- `INGEST_URL`，默认 `http://127.0.0.1:8090`
- `TENANT`，指定后按 tenant 过滤主机列表
- `EXPECT_METRICS=0`，只验证注册，不检查 `metric_batches`

## 纯本机方式运行后端

如果你不想跑容器，也可以直接启动各服务，但要先准备 PostgreSQL 和 Redis。

`master-api` 默认连的是：

- PostgreSQL `127.0.0.1:5432`
- Redis `127.0.0.1:6379`

如果你复用 `docker compose` 暴露的端口，则需要显式覆盖：

```bash
MASTER_API_POSTGRES_DSN='postgres://gaoming:gaoming@127.0.0.1:35432/gaoming?sslmode=disable' \
MASTER_API_REDIS_ADDR='127.0.0.1:36379' \
make run-master
```

其余服务可分别启动：

```bash
make run-ingest
make run-core
make run-probe
```

## 当前页面链路

Web 页面当前不由 `master-api` 直接托管。

开发模式下：

- `Vite` 运行在 `5173`
- `/master/*` 通过代理转发到 `WEB_API_ORIGIN`

因此本地看页面时，应访问 `5173`，不是 `8080`。

## 关键验证点

### 注册与租户

- 运行 `make run-agent` 后，`GET /master/api/v1/hosts?tenant=default` 应该能看到主机
- `agent-config.yaml` 里会保存当前租户

### 实时更新

- 刷新 `http://127.0.0.1:5173/default`
- 页面应该通过 `SSE` 收到 `sync` 和后续 `host_upsert`

### 离线判定

- 停掉 `agent`
- 等待约 15 秒到 20 秒
- 主机会被标记为 `OFFLINE`

### 接入层计数

- `metric_batches` 会随着 Agent 周期上报增长
- `probe_reports` 会随着 `probe-worker` 周期探测增长

## 常用命令

```bash
make docker-logs
make docker-ps
make docker-down
make test
make check
```
