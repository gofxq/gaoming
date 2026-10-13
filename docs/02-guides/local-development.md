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

## 3. 管理员登录

`make dev`、`make air` 和 `make backend` 都运行 `server/main.go`。启动时，整个 `users` 表为空就自动创建 `default` 租户下的启用管理员，用户名 `admin`。密码优先读取 `GAOMING_ADMIN_PASSWORD`，未设置或为空时使用 `G@oming`。

页面 `http://127.0.0.1:5173/default/login`；新空库默认使用 `admin` / `G@oming` 登录。

已有任何租户的任何用户时跳过初始化，不修改原密码。所有用户删除后，下次启动重新初始化；并发启动通过事务锁确保只有一个账号。密码只存 bcrypt 哈希，日志仅记录用户名、租户和是否使用默认密码，不打印密码。

需要强密码时，在首次启动前通过环境变量传入。例如 zsh：

```zsh
read -rs 'GAOMING_ADMIN_PASSWORD?输入管理员初始化密码: '
printf '\n'
export GAOMING_ADMIN_PASSWORD
make air
unset GAOMING_ADMIN_PASSWORD
```

修改环境变量不会重置已存在的账号。默认短密码只在启动初始化路径允许，普通网页创建仍保持 12～72 字节校验。初始化同时标记网页 setup 已完成。默认密码是公开弱密码，非本机测试应在首次启动前设置强密码并限制网络访问。

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

本机与 Docker 均使用 `server/main.go`，不再保留独立模块服务入口。Docker 部署使用以下命令，不属于本机启动链路：

```bash
make docker-up
make docker-logs
make docker-ps
make docker-down
# 连容器 Agent 也启动时显式开启 profile：
make docker-up DOCKER_PROFILES="--profile container-agent"
```

Docker 构建时执行前端生产构建，将 `web/dist` 放入同一 server 镜像，由 Go 进程在 8080 提供页面和 API。页面地址为 `http://127.0.0.1:8080/default`；不再启动独立 Web 容器，也不发布 Docker 前端 5173 端口。前端使用同源 `/master/*`，无需 Vite 代理。

`docker-up` 自动校验 Compose 配置。`make docker-smoke` 是独立临时 Docker 集成测试，完成后清理自身资源。验证已启动的 Docker 后端时，通过 `make smoke MASTER_URL=http://127.0.0.1:8080/master INGEST_URL=http://127.0.0.1:8080/ingest` 验证同一个 HTTP 端口。

修改 Docker 的 `HTTP_PORT` 时，需在 Docker 配置的 `allowed_origins` 中加入浏览器实际 Origin；HTTPS 反向代理则填写对应 HTTPS Origin。

Compose 仍支持 `POSTGRES_PORT`、`REDIS_PORT`、`HTTP_PORT`、`GRPC_PORT` 覆盖宿主机端口（`WEB_PORT` 仅用于本机 Vite）；容器内部仍使用 `postgres:5432` 和 `redis:6379`。网络启动失败留下残留容器时，执行 `docker-down` 后以同样参数重新 `docker-up`，不会删除数据库卷。

`deployments/sql/init.sql` 是完整的新库初始化定义。`make deps-up` 只创建空数据库，需要自行通过数据库客户端执行该文件；全栈 Compose 在新 PostgreSQL 数据卷首次启动时导入它。已有库由使用者自行维护，见[数据库初始化](./database-setup.md)。

旧的 `up/down`、`agent`、`local-migrate/local-schema-check`、`docker-dev*`、`web-local` 等重复目标已移除。数据库通过 PostgreSQL 客户端管理；统一后端使用 `SERVER_CONFIG`；前端统一使用 `web-dev`，远程代理可通过 `WEB_PROXY_TARGET` 指定；类型检查由 `web-build` 覆盖，协议定义和生成代码校验统一由 `proto-check` 完成。

## 安全

统一 HTTP 端口同时提供 `/master/*` 和 `/ingest/*`。`/ingest/api/v1/events` 和 `/ingest/api/v1/probes` 沿用现有无认证接收行为；公开反向代理应限制这些接入路由，只向可信 Agent/探测端开放。

开发模板使用 HTTP 和非 Secure Cookie，仅用于可信本机/局域网。端口监听 `0.0.0.0`，应通过防火墙限制访问；从其他设备访问时，在 `allowed_origins` 中明确添加 `http://<本机局域网 IP>:<Web 端口>`。生产使用 HTTPS 与 Secure Cookie，并在首次启动前设置独立的管理员强密码。统一运行保留登录、CSRF、会话与租户隔离；Agent/worker 接入鉴权的既有限制仍见[登录手册](./authentication.md)。

## 本机预览合并产物

默认本机开发保留独立 Vite 热更新。需要通过 Go 服务预览生产前端时执行 `make web-build WEB_API_ORIGIN=`，然后设置 `GAOMING_WEB_DIR=web/dist make backend`；也可在 YAML 中设置 `web_dir`。目录必须包含构建后的 `index.html`，不要将源码或配置目录作为静态目录。
