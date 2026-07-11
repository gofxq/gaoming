# 本地开发与验证

## 推荐拓扑

- PostgreSQL、Redis、后端服务和 Web：Docker Compose。
- Agent：宿主机运行，以采集真实宿主机指标。

需要 Docker、Go、Yarn，以及执行 `make check` 时使用的 `protoc`。

## 1. 准备后端配置

所有服务都只读取 YAML 文件；`config/` 被 `.gitignore` 排除。新环境先创建以下文件。

`config/master-api.docker.yml`：

```yaml
http_addr: ":8080"
runtime_backend: pg_redis
postgres_dsn: "postgres://gaoming:gaoming@postgres:5432/gaoming?sslmode=disable"
redis_addr: "redis:6379"
redis_password: ""
redis_db: 0
tenant_code: default
tenant_name: Default Tenant
allow_custom_tenant_code: true
session_cookie_name: gaoming_session
session_secret: change-me-in-non-local-environments
session_ttl_hours: 168
```

`config/ingest-gateway.docker.yml`：

```yaml
http_addr: ":8090"
grpc_addr: ":8091"
postgres_dsn: "postgres://gaoming:gaoming@postgres:5432/gaoming?sslmode=disable"
redis_addr: "redis:6379"
redis_password: ""
redis_db: 0
tenant_code: default
tenant_name: Default Tenant
allow_custom_tenant_code: true
```

`config/core-worker.docker.yml`：

```yaml
loop_interval_sec: 15
```

`config/probe-worker.docker.yml`：

```yaml
worker_id: probe-worker-local
target_url: http://master-api:8080/master/healthz
report_url: http://ingest-gateway:8090/ingest/api/v1/probes
region: local
probe_interval_sec: 15
```

若直接在宿主机运行后端，请创建不带 `.docker` 的同名文件，并把 PostgreSQL/Redis 地址改为 Compose 暴露的 `127.0.0.1:35432` 和 `127.0.0.1:36379`。

## 2. 启动后端与 Web

```bash
make up
make smoke
```

`make up` 会构建并启动依赖、后端服务和 `web` profile。页面地址：

- `http://127.0.0.1:5173/default`
- 用户管理：`http://127.0.0.1:5173/default/users`（需要已有管理员会话）

常用排查命令：

```bash
make docker-ps
make docker-logs
make compose-config
```

## 3. 启动宿主机 Agent

在仓库根目录创建 `agent-config.yaml`：

```yaml
ingest_gateway_grpc_addr: "127.0.0.1:8091"
region: local
env: dev
role: node
tenant_code: default
loop_interval_sec: 1
```

然后运行：

```bash
make agent
```

Agent 也可通过 `GAOMING_AGENT_CONFIG` 指定其他配置文件。首次上报要求 `tenant_code` 已存在；默认租户会在后端初始化时创建。租户不存在时服务端返回 `FailedPrecondition`，Agent 会退出。

另一个终端执行：

```bash
TENANT=default make smoke-agent
```

可覆盖的 smoke 参数包括：

- `MASTER_URL`：默认 `http://127.0.0.1:8080`。
- `INGEST_URL`：默认 `http://127.0.0.1:8090`。
- `TENANT`：按租户过滤主机列表。
- `EXPECT_METRICS=0`：不检查 metric batch 增长。

## 4. 手工验证

```bash
curl http://127.0.0.1:8080/master/healthz
curl "http://127.0.0.1:8080/master/api/v1/hosts?tenant=default"
curl http://127.0.0.1:8090/ingest/healthz
curl http://127.0.0.1:8090/ingest/debug/counters
```

应看到：

- Agent 运行后，主机出现在 `/hosts` 和 Dashboard。
- `metric_batches` 持续增长。
- Dashboard 的 SSE 状态连接成功，指标持续更新。
- 停止 Agent 约 15～20 秒后，主机变为 `OFFLINE`。
- `probe-worker` 运行时，`probe_reports` 持续增长；这只证明接收成功，不代表探测结果已落库。

## 5. 直接运行单个进程

准备对应的非 Docker 配置后，可使用：

```bash
make master
make ingest
make core
make probe
make web-local
```

Web 默认通过 Vite 把 `/master/*` 代理到 `http://localhost:8080`。若连接其他后端：

```bash
make web-dev WEB_API_ORIGIN=http://127.0.0.1:8080
```

## 6. 检查与清理

```bash
make test
make web-typecheck
make web-build
make check
make down
```

注意：`deployments/sql/init.sql` 只在 PostgreSQL 数据卷首次创建时执行。已有数据卷需要显式执行升级 SQL，不能依赖重启服务自动迁移。
