# 数据库初始化与维护

数据库由使用者通过 PostgreSQL 客户端直接管理。后端不执行 DDL、迁移版本校验或离线账号管理命令。

## 新数据库

连接到配置中的空数据库后，执行 [`deployments/sql/init.sql`](../../deployments/sql/init.sql)。例如使用 psql 的连接配置（密码通过受保护的 `.pgpass` 或交互输入提供）：

```sh
psql -h 127.0.0.1 -p 25432 -U gaoming -d gaoming -v ON_ERROR_STOP=1 --single-transaction -f deployments/sql/init.sql
```

连接参数以 `config/server.yml` 为准。SQL 包含当前完整表结构，只用于空库；不要在已有业务数据的数据库重复导入。

`make deps-up` 只启动 PostgreSQL/Redis，首次使用需要自行执行初始化 SQL。全栈 Docker Compose 已将该文件挂载到 PostgreSQL 初始化目录，只有新数据卷第一次启动时自动执行；已有数据卷不会重新执行。

## 已有数据库与账号

升级时先备份，再用数据库客户端检查现有结构并执行所需 SQL；后端不再维护 `schema_migrations` 版本账本。旧账本可以保留，服务不读取它。不要通过删除数据卷升级。

账号日常管理使用页面已有功能。直接维护数据库时须保持租户关联及用户身份、密码和会话表的一致性；密码保存为 bcrypt 哈希，不能写入明文。已有用户的密码不会因修改启动环境变量自动重置。

## 验证

`TEST_POSTGRES_DSN` 指向独立测试库时，Go 集成测试在各自临时 schema 中直接执行初始化 SQL，结束后清理自己的 schema。生产库不用于测试。

`make docker-smoke` 创建独立数据库卷，通过初始化 SQL 建表、启动统一服务、登录并验证 Agent 上报，只清理自己的测试项目。
