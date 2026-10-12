# 本机统一后端

用户选择：本地测试不使用 Docker，保留本机 PostgreSQL 和 Redis，先合并四个业务后端。

新增 `services/server/cmd/server`，在一个 Go 进程中装配 master-api、ingest-gateway、core-worker、probe-worker。共享 PostgreSQL 连接池和 Redis 客户端，HTTP 保留 `/master/*` 与 `/ingest/*` 路由，同端口监听；gRPC 独立端口。四个原有可执行入口保留。通过各服务公开的 runtime 装配包复用内部实现，遵守 Go internal 可见性规则。Agent 仍是独立宿主机进程。

默认配置 `config/server.yml`，HTTP `0.0.0.0:8080`，gRPC `0.0.0.0:8091`，数据库与 Redis 使用本机 5432、6379。初始化只复制缺失配置，绝不覆盖用户配置；没有固定登录密码。迁移沿用现有 SQL，作为明确的启动前步骤；统一后端只校验版本，不自动篡改已有数据库。

`make dev` 顺序迁移、构建后端并监督后端和 Vite，Ctrl-C 停止本次子进程；任一子进程退出使联调退出。`make backend` 只运行后端，`make air` 热更新统一后端。Docker 专用命令显式标记 docker 前缀。前端代理与 smoke 的统一服务地址跟随 HTTP 端口；原有 Docker 端口兼容。

监听端口不可用时不遗留其他监听器；后台任务在监听器准备好后启动；退出按取消后台任务、限时停止 HTTP/gRPC、关闭共享连接顺序执行。gRPC 长连接和 SSE 不得阻止有限时间内退出。保留登录、CSRF、会话及租户隔离。

验证：配置错误与默认值测试、同端口 master/ingest 路由与真实 gRPC 上报、登录访问与匿名拒绝、探测计数增长、取消及长连接关闭、绑定失败清理；使用本机依赖运行隔离集成测试。无依赖时集成测试显式跳过，不能声称通过；完整 Go 测试及前端构建必须执行。
