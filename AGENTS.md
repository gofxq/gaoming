# Repository Guidelines

# 重要

每次完成之后都要检查是否可能引入造成安全风险的代码，并给出提醒。

## 项目结构与模块组织

`gaoming` 是一个面向主机监控的单仓库项目。后端源码统一位于 `server/`：`cmd/server` 是唯一在线服务入口；`internal/app` 负责统一装配，`internal/config` 读取统一配置，`internal/master`、`internal/ingest`、`internal/core`、`internal/probe` 是同一进程内的业务模块，不再保留独立服务入口。宿主机 Agent 位于 `agent/daemon`，独立运行。通用 Go 包放在 `pkg/`，协议定义放在 `api/proto/`，React + Vite 前端位于 `web/`。`README.md` 保持高层说明，设计细节和运行手册放入 `docs/`。

## 构建、测试与开发命令

优先使用 `make` 目标，而不是临时拼接命令：

- `make fmt`：执行 `go fmt ./...`。
- `make test`：运行全部 Go 测试。
- `make build`：构建所有 Go 可执行文件。
- `make check`：检查格式、运行 Go/前端测试、构建、proto 校验和前端构建。
- `make deps-up` / `make dev`：启动 Docker 数据依赖，在宿主机运行统一后端和前端。
- `make backend`：只运行统一后端，默认读取 `config/server.yml`。
- `make docker-up` / `make docker-down`：启动或停止本地后端联调环境。
- `make run-agent`：在当前机器运行 Agent。
- `make smoke`、`make smoke-agent`：验证后端链路和 Agent 上报链路。
- `make web-dev WEB_PROXY_TARGET=http://127.0.0.1:8080`：启动前端开发环境，并通过 Vite 代理本地 API。
- `make web-build`：构建前端产物。

## 代码风格与命名约定

Go 代码必须保持 `gofmt` 结果一致，并沿用现有目录分层：入口放在 `cmd/`，装配逻辑放在 `internal/app`，配置放在 `internal/config`，可复用能力放在 `pkg/`。前端使用 TypeScript，统一 2 空格缩进；组件文件采用 PascalCase，例如 `DashboardPage.tsx`；Hook 和工具函数采用 camelCase，例如 `useLiveHostsData.ts`。

## 测试规范

Go 测试文件与实现文件相邻放置，文件名使用标准 `_test.go` 后缀。提交 PR 前至少运行 `make test`；开发过程中可用 `go test ./server/internal/master/...` 这类命令做定向验证。如果改动影响 Docker、联调链路或集成行为，还应运行 `make compose-config`、`make docker-up`、`make smoke` 和 `make docker-down`。前端有单元测试和 Playwright 验证，前端改动至少执行 `make web-build` 或 `cd web && yarn typecheck`。

## 提交与 Pull Request 规范

最近的提交历史以简短主题为主，常见前缀包括 `feat:`、`fix:`、`refactor:`，请保持一致。PR 说明应写清受影响的服务或页面、涉及的环境变量或配置变更；若修改 Dashboard 或 PWA 页面，应附带截图。有对应 issue 时请关联，并在描述中列出已执行的验证命令。

## 安全与配置提示

不要提交密钥或机器本地专用配置。数据库由使用者直接连接 PostgreSQL 管理，空库执行 `deployments/sql/init.sql`；后端不提供离线管理员命令、自动迁移或迁移版本校验。`config/` 是被忽略的本机配置目录；Docker 示例配置位于 `deployments/server-config.docker.yaml`。统一 HTTP 8080 提供 `/master/*` 和 `/ingest/*`，gRPC 8091 提供 Agent 接入。首次启动非本机环境前设置强 `GAOMING_ADMIN_PASSWORD`，生产使用 HTTPS/Secure Cookie 并限制数据库与 Redis 访问。若变更影响数据库初始化或启动流程，请同步更新 `deployments/sql/init.sql` 及相关文档。

## 执行给出一个合适的git message
