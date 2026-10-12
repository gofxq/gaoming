# 本地开发与验证

## 推荐方式：本机统一后端

本地仅 PostgreSQL 和 Redis 使用 Docker。`master-api`、`ingest-gateway`、`probe-worker` 在一个 Go 进程中运行，共享 PostgreSQL 连接池和 Redis 客户端。Agent 单独在宿主机运行，Vite 单独提供前端页面。

需要 Go（版本以 `go.mod` 为准）、Node/Yarn 和 Docker Compose。PostgreSQL、Redis 使用 Docker 容器；四个业务后端合并成一个 Go 进程，在宿主机运行，前端使用本机 Vite。热更新另外需要 Air。

## 1. 启动 Docker PostgreSQL 和 Redis

```bash
make deps-up
```

首次使用空库时，接下来通过数据库客户端执行 `deployments/sql/init.sql`，然后按第 2 节启动服务。

`deps-up` 自动运行 `dev-init`，仅在缺失时生成 `config/server.yml` 和不同的随机 PostgreSQL/Redis 密码。已有配置不会被改写。容器端口、用户名、数据库名和密码均从该文件读取，默认 PostgreSQL 5432、Redis 6379，发布地址为 `0.0.0.0`。不需要安装 `initdb`、`pg_ctl` 或本机 Redis 服务端。

该命令只启动 PostgreSQL、Redis 两个容器，使用由 `DEPS_DIR` 派生的独立 Compose 项目，不启动后端或前端容器，也不操作其他部署项目。等待健康检查通过后验证宿主机能连接数据库。端口冲突时修改 `postgres_dsn` / `redis_addr` 中的端口，使用 `127.0.0.1` 本机地址，然后重新启动（端口发布使用 IPv4，配置不能使用 `::1`）。

密码通过 git 忽略的 `.local-data/` 内私有文件挂载到容器，配置和状态文件权限为 0600，目录权限须为 0700。PostgreSQL 使用持久 Docker 数据卷；Redis 是缓存实例，重启不保留缓存。`deps-up` 不重复初始化已有数据，修改已初始化的数据库凭据会被拒绝，避免容器环境变量与存量数据库密码不一致。

停止依赖：

```bash
make deps-down
```

只停止本项目的依赖容器并保留 PostgreSQL 卷；`make clean` 也不删除数据卷或依赖状态目录。启动失败时可执行 `deps-down` 再重试；容器和卷都不会因为失败被自动清空。可用 `DEPS_DIR` 指定另一个私有状态目录，启动和停止需使用同一个目录及 Docker context。

原本机 PostgreSQL 数据目录保留，不自动复制到 Docker 数据卷。若已有本机实例占用端口，先停止该实例或更改配置中的端口；不要删除旧数据库目录。旧依赖管理文件仍作为数据保留与清理保护依据。

如果使用自己已经启动的本机数据库或 Redis，可单独运行 `make dev-init` 后编辑配置，使凭据与已有实例匹配，并直接 `make dev`。网页登录账号与数据库账号独立，统一后端的空用户表自动初始化规则见下文。

默认监听：

- 统一 HTTP：`0.0.0.0:8080`，包含 `/master/*`、`/ingest/*`。
- Agent gRPC：`0.0.0.0:8091`。
- 前端：`0.0.0.0:5173`。

`probe_interval_sec` 控制探测任务周期。省略 `probe_target_url` 时，probe-worker 探测本进程的 `/master/healthz` 并向同一 HTTP 端口提交报告。

## 2. 启动本机服务

首次启动前，连接 PostgreSQL 并在空库执行 `deployments/sql/init.sql`，见[数据库初始化](./database-setup.md)。

```bash
make dev
```

顺序安装前端依赖、构建统一后端，再启动后端和 Vite。后端健康检查通过后启动前端；任一进程退出会停止本次联调进程。Ctrl-C 停止后端和前端，本机数据库与 Redis 继续运行，数据保留。后端日志在 `.tmp/local/logs/`。

页面：`http://127.0.0.1:5173/default`。

热更新或只运行后端：

```bash
make air                  # 统一后端 + Vite，需 Air
make backend              # 只运行统一后端，数据库须已初始化
```

端口冲突时：

```bash
make dev HTTP_PORT=18080 GRPC_PORT=18091 WEB_PORT=15173
make smoke HTTP_PORT=18080
```

`HTTP_PORT` 同时控制统一后端、Vite 代理、内部探测和 smoke 地址。`GRPC_PORT` 控制 gRPC。更改 `WEB_PORT` 时，在 `config/server.yml` 的 `allowed_origins` 中添加对应页面 Origin，例如 `http://127.0.0.1:15173`。修改数据库端口应直接修改 `postgres_dsn` / `redis_addr`。

`SERVER_CONFIG` 可指向其他 YAML 文件，例如 `make dev SERVER_CONFIG=config/server-local.yml`。`make backend`、`make dev`、`make air` 显式使用端口变量；直接运行二进制时使用 YAML 中的监听地址，也可通过 `GAOMING_HTTP_ADDR`、`GAOMING_GRPC_ADDR` 覆盖。数据库与 Redis 的监听端口在 `config/server.yml` 中设置。

## 3. 网页初始化与登录

后端启动不会创建用户，没有默认管理员或默认密码。首次使用空库时，打开 `http://127.0.0.1:5173/default/setup`，输入引导凭据、用户名、显示名称和 12～72 字节密码，创建并登录管理员。

`make dev-init` 为新配置生成独立随机 `setup_token`，存放在权限 0600 的 `config/server.yml` 中。已有配置保持不变：若缺少凭据，请自行添加至少 32 字节的随机 `setup_token`，或通过环境变量 `GAOMING_SETUP_TOKEN` 传入；环境变量优先。不要把凭据写入 URL、日志、截图或版本库。

网页 setup 只在整个数据库没有用户、且 `auth_bootstrap_state` 尚无完成记录时开放。并发请求只能成功一次。初始化成功后撤销引导凭据；重启或删除所有用户都不会重新开放初始化。

已有用户直接访问 `/default/login`（或所属租户的登录页）。旧版本已经创建的管理员账号保持不变；本次更新不会删除或重置已有账号。账号丢失时由可信数据库管理员维护，密码必须保存为 bcrypt 哈希。

Docker 首次使用前通过部署环境提供 `GAOMING_SETUP_TOKEN`，再执行 `make docker-up`；已初始化部署无需该变量。更多配置见[登录手册](./authentication.md)。

## 4. 启动宿主机 Agent

创建 git 忽略的 `agent-config.yaml`：

```yaml
master_api_url: "http://127.0.0.1:8080"
ingest_gateway_grpc_addr: "127.0.0.1:8091"
region: local
env: dev
role: node
tenant_code: default
loop_interval_sec: 1
```

另一终端执行：

```bash
make run-agent
```

自定义端口时同步修改 Agent 配置中的 HTTP/gRPC 地址。`GAOMING_AGENT_CONFIG` 可指定其他 Agent 配置文件。首次上报要求 `tenant_code` 已存在，默认租户由服务启动时创建。

## 5. 验证

```bash
make smoke
SESSION_COOKIE_FILE=/path/to/login-cookies.txt make smoke-agent
make test
make web-install
make test-all
make web-build
make check
make help
```

`smoke-agent` 必须使用已登录的 Cookie jar，验证 Agent 注册及指标 batch 增长；可通过 `MASTER_URL`、`INGEST_URL` 覆盖远程地址，或 `HTTP_PORT` 覆盖本机统一端口。`EXPECT_METRICS=0` 只验证注册。Go 集成测试可配置 `TEST_POSTGRES_DSN`，统一后端集成测试还需 `TEST_REDIS_ADDR`（可选 `TEST_REDIS_PASSWORD`）。请使用独立测试依赖；PostgreSQL 测试自行创建并清理临时 schema，统一后端集成测试同时隔离并清理 Redis 命名空间。

```bash
curl http://127.0.0.1:8080/master/healthz
curl http://127.0.0.1:8080/ingest/healthz
curl http://127.0.0.1:8080/ingest/debug/counters
curl --cookie /path/to/login-cookies.txt http://127.0.0.1:8080/master/api/v1/hosts
```

Agent 上报后主机出现在看板中，`metric_batches` 持续增长；内置 probe-worker 的 `probe_reports` 持续增长。

## 6. 可选 Docker 部署

本机与 Docker 均使用 `server/cmd/server`，不再保留独立模块服务入口。Docker 部署使用以下命令，不属于本机启动链路：

```bash
make docker-up
make docker-logs
make docker-ps
make docker-down
# 连容器 Agent 也启动时显式开启 profile：
make docker-up DOCKER_PROFILES="--profile web --profile container-agent"
```

`docker-up` 自动校验 Compose 配置。`make docker-smoke` 是独立临时 Docker 集成测试，完成后清理自身资源。验证已启动的 Docker 后端时，通过 `make smoke MASTER_URL=http://127.0.0.1:8080/master INGEST_URL=http://127.0.0.1:8080/ingest` 验证同一个 HTTP 端口。

Compose 仍支持 `POSTGRES_PORT`、`REDIS_PORT`、`HTTP_PORT`、`GRPC_PORT`、`WEB_PORT` 覆盖宿主机端口；容器内部仍使用 `postgres:5432` 和 `redis:6379`。网络启动失败留下残留容器时，执行 `docker-down` 后以同样参数重新 `docker-up`，不会删除数据库卷。

`deployments/sql/init.sql` 是完整的新库初始化定义。`make deps-up` 只创建空数据库，需要自行通过数据库客户端执行该文件；全栈 Compose 在新 PostgreSQL 数据卷首次启动时导入它。已有库由使用者自行维护，见[数据库初始化](./database-setup.md)。

旧的 `up/down`、`agent`、`local-migrate/local-schema-check`、`docker-dev*`、`web-local` 等重复目标已移除。数据库通过 PostgreSQL 客户端管理；统一后端使用 `SERVER_CONFIG`；前端统一使用 `web-dev`，远程代理可通过 `WEB_PROXY_TARGET` 指定；类型检查由 `web-build` 覆盖，协议定义和生成代码校验统一由 `proto-check` 完成。

## 安全

统一 HTTP 端口同时提供 `/master/*` 和 `/ingest/*`。`/ingest/api/v1/events` 和 `/ingest/api/v1/probes` 沿用现有无认证接收行为；公开反向代理应限制这些接入路由，只向可信 Agent/探测端开放。

开发模板使用 HTTP 和非 Secure Cookie，仅用于可信本机/局域网。端口监听 `0.0.0.0`，应通过防火墙限制访问；从其他设备访问时，在 `allowed_origins` 中明确添加 `http://<本机局域网 IP>:<Web 端口>`。生产使用 HTTPS 与 Secure Cookie，不设置已知默认网页登录密码。统一运行保留登录、CSRF、会话与租户隔离；Agent/worker 接入鉴权的既有限制仍见[登录手册](./authentication.md)。
