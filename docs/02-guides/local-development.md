# 本地开发与验证

## 推荐方式：本机统一后端

本地测试不使用 Docker。`master-api`、`ingest-gateway`、`core-worker`、`probe-worker` 在一个 Go 进程中运行，共享 PostgreSQL 连接池和 Redis 客户端。Agent 单独在宿主机运行，Vite 单独提供前端页面。

需要 Go（版本以 `go.mod` 为准）、Node/Yarn、PostgreSQL 服务端和 Redis。热更新另外需要 Air。原生依赖命令需要 `initdb`、`pg_ctl`、`postgres` 和 `redis-server`；仅安装 `psql` 客户端不够。PostgreSQL 会从 PATH、`pg_config --bindir` 和常见 Linux/Homebrew 安装目录中查找，也可指定 `PG_BIN_DIR`。

## 1. 启动本机 PostgreSQL 和 Redis

安装服务端后，一条命令准备项目自己的依赖：

```bash
make deps-up
make dev
```

`deps-up` 自动运行 `dev-init`，仅在缺失时生成 `config/server.yml`，并为 PostgreSQL、Redis 生成不同的随机密码；已有配置不会被改写。它创建当前用户拥有的 PostgreSQL 集群、开发数据库和 Redis 实例，不连接或停止系统/其他项目的实例。需要以普通用户执行，不能用 sudo 运行 PostgreSQL。

监听端口取自配置里的 `postgres_dsn` 和 `redis_addr`，默认 5432、6379；监听全部网卡，并启用 PostgreSQL SCRAM 和 Redis 密码认证。已有配置中的地址必须是本机地址，且两项密码不能为空；`postgres_dsn` 须使用明确带用户名和密码的 `postgres://` URL，不复用环境变量中的数据库密码来初始化新集群。若已有服务占用端口，该命令会报错；修改配置中的端口后再试，不会抢占端口或停止原进程。

数据、日志和私有状态放在 git 忽略的 `.local-data/`；PostgreSQL 数据持久保存，Redis 为缓存实例，重启不保留缓存。重复 `deps-up` 复用原集群，不重置数据。Ctrl-C 停止 `dev` 后，依赖仍运行；用下面的命令停止：

```bash
make deps-down
```

停止命令只通过项目数据目录控制 PostgreSQL，并通过私有 Unix socket 控制 Redis，保留数据。`make clean` 也会保留 `DEPS_DIR` 和已登记的 PostgreSQL 二进制目录，包括将它们自定义到构建目录内的情况。可用 `DEPS_DIR` 指定其他数据目录（应放在 git 忽略路径下，目录权限须为 0700），启动与停止时传入相同目录。

```bash
make deps-up PG_BIN_DIR=/path/to/postgresql/bin
make deps-down
```

修改依赖端口前先执行 `deps-down`；修改已初始化集群的用户名、数据库名或密码会被拒绝，需恢复原配置或使用新的 `DEPS_DIR`，不能通过删除旧数据来掩盖配置不一致。命令不会自动安装系统软件。

初始化或文件系统写入异常后，若出现没有完整管理状态的数据目录，命令会保留并拒绝自动接管该目录；应先检查日志和磁盘状态，或使用新的 `DEPS_DIR`。不会自动删除旧目录来重试。

如果使用自己已经启动的本机数据库/Redis，无需执行 `deps-up`，可单独运行 `make dev-init` 后编辑配置，使凭据与已有实例匹配。网页登录账号与数据库账号独立；不会生成默认网页登录密码。

默认监听：

- 统一 HTTP：`0.0.0.0:8080`，包含 `/master/*`、`/ingest/*`。
- Agent gRPC：`0.0.0.0:8091`。
- 前端：`0.0.0.0:5173`。

`core_interval_sec` 和 `probe_interval_sec` 控制后台任务周期。省略 `probe_target_url` 时，probe-worker 探测本进程的 `/master/healthz` 并向同一 HTTP 端口提交报告。

## 2. 启动本机服务

```bash
make dev
```

顺序安装前端依赖、迁移数据库、构建统一后端，再启动后端和 Vite。后端健康检查通过后启动前端；任一进程退出会停止本次联调进程。Ctrl-C 停止后端和前端，本机数据库与 Redis 继续运行，数据保留。后端日志在 `.tmp/local/logs/`。

页面：`http://127.0.0.1:5173/default`。

热更新或只运行后端：

```bash
make air                  # 自动迁移；统一后端 + Vite，需 Air
make migrate              # 手动迁移统一后端数据库
make backend              # 只运行统一后端，数据库须已迁移
make schema-check         # 检查统一后端数据库版本
```

端口冲突时：

```bash
make dev HTTP_PORT=18080 GRPC_PORT=18091 WEB_PORT=15173
make smoke HTTP_PORT=18080
```

`HTTP_PORT` 同时控制统一后端、Vite 代理、内部探测和 smoke 地址。`GRPC_PORT` 控制 gRPC。更改 `WEB_PORT` 时，在 `config/server.yml` 的 `allowed_origins` 中添加对应页面 Origin，例如 `http://127.0.0.1:15173`。修改数据库端口应直接修改 `postgres_dsn` / `redis_addr`。

`SERVER_CONFIG` 可指向其他 YAML 文件，例如 `make dev SERVER_CONFIG=config/server-local.yml`。`make backend`、`make dev`、`make air` 显式使用端口变量；直接运行二进制时使用 YAML 中的监听地址，也可通过 `GAOMING_HTTP_ADDR`、`GAOMING_GRPC_ADDR` 覆盖。数据库与 Redis 的监听端口在 `config/server.yml` 中设置。

## 3. 创建测试管理员

没有默认网页登录账号。使用统一后端配置创建账号，密码至少 12 个字符。在 zsh 终端中：

```zsh
read -rs 'task_password?输入测试密码: '
printf '\n'
printf '%s\n' "$task_password" | go run ./services/master-api/cmd/admin \
  -config config/server.yml -tenant default -username test-admin \
  -display-name 'Test Admin' -password-stdin create-admin
unset task_password
```

已有同名用户时该命令拒绝重复创建，不重置密码。密码通过 stdin 提供，不进入命令行参数或历史。然后在 `/default/login` 使用 `test-admin` 和自己设置的密码登录。初始化引导和生产登录配置见[登录手册](./authentication.md)。

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

Agent 上报后主机出现在看板中，`metric_batches` 持续增长；内置 probe-worker 的 `probe_reports` 持续增长。core-worker 保留现有周期任务行为，目前仅记录 pipeline tick，合并运行不额外实现告警/调度算法。

## 6. 可选 Docker 部署

本机开发只使用统一后端；独立服务二进制仍保留在 `services/*/cmd`。Docker 部署使用以下精简命令，不属于本机启动链路：

```bash
make docker-up
make docker-logs
make docker-ps
make docker-down
# 连容器 Agent 也启动时显式开启 profile：
make docker-up DOCKER_PROFILES="--profile web --profile container-agent"
```

`docker-up` 自动校验 Compose 配置。`make docker-smoke` 是独立临时 Docker 集成测试，完成后清理自身资源。验证已启动的 Docker 后端时，通过 `make smoke MASTER_URL=http://127.0.0.1:8080/master INGEST_URL=http://127.0.0.1:8090/ingest` 指定两个独立端口。

Compose 仍支持 `POSTGRES_PORT`、`REDIS_PORT`、`MASTER_PORT`、`INGEST_HTTP_PORT`、`INGEST_GRPC_PORT`、`WEB_PORT` 覆盖宿主机端口；容器内部仍使用 `postgres:5432` 和 `redis:6379`。网络启动失败留下残留容器时，执行 `docker-down` 后以同样参数重新 `docker-up`，不会删除数据库卷。

`deployments/sql/init.sql` 是 Docker 新数据库初始化定义；本机集群通过 `make migrate` 使用同一套版本化迁移，不依赖容器初始化。依赖启动器只创建数据库，不建业务表；数据库结构没有改变。

旧的 `up/down`、`agent`、`local-migrate/local-schema-check`、`docker-dev*`、`web-local` 等重复目标已移除。迁移统一使用 `migrate/schema-check` 和 `SERVER_CONFIG`；前端统一使用 `web-dev`，远程代理可通过 `WEB_PROXY_TARGET` 指定；类型检查由 `web-build` 覆盖，协议定义和生成代码校验统一由 `proto-check` 完成。

## 安全

开发模板使用 HTTP 和非 Secure Cookie，仅用于可信本机/局域网。端口监听 `0.0.0.0`，应通过防火墙限制访问；从其他设备访问时，在 `allowed_origins` 中明确添加 `http://<本机局域网 IP>:<Web 端口>`。生产使用 HTTPS 与 Secure Cookie，不设置已知默认网页登录密码。统一运行保留登录、CSRF、会话与租户隔离；Agent/worker 接入鉴权的既有限制仍见[登录手册](./authentication.md)。
