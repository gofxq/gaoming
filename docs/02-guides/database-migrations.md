# 数据库迁移

服务只检查版本，不在启动时修改 schema。首次部署或升级前，停止写入服务、备份数据库，然后使用与待部署版本配套的管理员命令迁移。禁止通过删除数据卷升级。

```sh
make migrate
make schema-check
```

Make 命令默认读取 `config/server.yml`，可用 `SERVER_CONFIG` 指定其他文件。直接运行管理员二进制时仍默认读取 `config/master-api.yml`，也可使用 `GAOMING_POSTGRES_DSN` 环境变量（由部署系统注入，不写入仓库或命令参数）；不要开启 shell tracing。自定义配置和期限：

```sh
go run ./services/master-api/cmd/admin -config /path/to/master-api.yml -timeout 5m migrate
```

Compose 的 `migrate` 一次性服务在 PostgreSQL 就绪后运行，master/ingest 等待它成功退出。先检查数据库连接配置，再执行 `make docker-up`；迁移失败时服务不会继续启动。容器内也可执行 `/app/admin schema-check`。

## 版本与失败恢复

SQL 位于 `deployments/sql/migrations/`，编号从 001 连续递增，随二进制嵌入。已发布文件不可修改，修正须增加新迁移，同时同步 `init.sql`。`schema_migrations` 保存名称、SHA-256 与应用时间；未知未来版本、缺号或已应用文件 checksum 不匹配会阻止启动。

整个待执行集合在一个事务内运行，advisory lock 防止并发迁移。失败或超时会回滚全部本次变更，不记录成功版本；修复配置或使用修正版本后可以重跑。旧的未版本化初始化库由幂等 baseline 接管，002 补齐旧版动态指标摘要列，不删除已有数据。物理列顺序可能不同，查询不得依赖 `SELECT *` 的位置。

备份和恢复应由数据库管理员在隔离数据库验证。迁移成功后再启动新服务；schema 已升级时，旧二进制可能拒绝未来版本，因此回滚须使用兼容版本或恢复升级前备份，不能删除版本记录伪装兼容。完整备份恢复脚本和业务恢复验收在 P1-07/T10 中继续交付；目前没有自动降级迁移。

## 验证

`make migration-test` 必须提供独立 PostgreSQL 数据库的 `TEST_POSTGRES_DSN`，缺少配置会失败。测试只创建自己的临时 schema 并清理，覆盖空库/存量兼容、保留数据、重复执行、失败回滚、checksum、未来版本、并发和锁等待取消。普通 `make test` 在未配置数据库时明确跳过这些集成测试；CI 强制执行。

```sh
TEST_POSTGRES_DSN='<独立测试库连接>' make migration-test
make fmt-check test build proto-check web-test web-build
```

协议检查使用 `protoc 3.21.12`、`protoc-gen-go v1.34.0`、`protoc-gen-go-grpc v1.5.1`，对临时生成目录执行 diff，不覆盖本地生成文件。浏览器验收先在 `web/` 执行 `yarn playwright install chromium`，再运行 `make web-e2e`。

`make docker-smoke` 创建独立 Compose 项目、新数据库卷、临时配置及随机回环端口，验证迁移→服务启动→Agent 注册/指标，然后只清理该测试项目。它不会读取现有 `config/` 或删除部署数据卷。这是当前阶段的基础链路验收，后续业务阶段会增加授权、故障恢复及产品流程检查。
