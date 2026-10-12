# 仓库结构

## 顶层目录

| 目录 | 职责 |
| --- | --- |
| `agent/daemon` | 宿主机 Agent |
| `api/proto`、`api/gen` | gRPC Proto 与生成代码 |
| `deployments` | Dockerfile、SQL、Agent 安装与服务脚本 |
| `docs` | 当前架构、指南和参考文档 |
| `pkg` | 后端与 Agent 共用的模型、仓储和工具 |
| `scripts` | smoke 与本地开发辅助脚本 |
| `server` | 唯一后端服务 |
| `web` | React + Vite 前端 |

## 后端服务

- `server/cmd/server`：唯一在线后端入口。
- `server/internal/app`：装配 HTTP/gRPC、后台任务及共享 PostgreSQL/Redis 连接，统一管理退出。
- `server/internal/config`：读取 `config/server.yml` 和端口环境变量。
- `server/internal/master`：查询、认证、用户管理和 `/master/*` HTTP/SSE；含 auth、config、service、transport。
- `server/internal/ingest`：Agent 注册、指标流、离线扫描及 `/ingest/*` HTTP/gRPC 接入。
- `server/internal/core`：周期占位 runner。
- `server/internal/probe`：固定目标 HTTP 探测与结果上报。

上述业务模块在同一进程运行，不提供独立启动入口。
## Agent

- `agent/daemon/cmd/agent`：进程入口。
- `internal/config`：从 `agent-config.yaml` 或 `GAOMING_AGENT_CONFIG` 读取配置。
- `internal/identity`：稳定主机 UID 和身份采集。
- `internal/service`：gopsutil 采样、gRPC 流和 ACK 处理。

## 共享包

- `pkg/contracts`：HTTP/JSON 契约。
- `pkg/state`：`HostSnapshot`、状态码和指标枚举。
- `pkg/hostruntime/repository`：当前 master/ingest 共用的仓储接口及实现。
- `pkg/clock`、`pkg/httpx`、`pkg/ids`、`pkg/logx`：基础工具。

`server/internal/master/repository` 是旧仓储实现，当前统一 app 不使用它。

## Web

- `web/src/app`：Router 和应用级 Provider。
- `web/src/pc/pages`：PC 页面和 Semi UI 应用壳层。
- `web/src/pc/components/pixel`：PC 专属 Pixel 展示覆写。
- `web/src/h5/pages`、`web/src/h5/components/pixel`：H5 页面和 H5 Pixel 展示边界。
- `web/src/shared/features`：跨端共享的鉴权、租户、配置、外观状态和主机实时数据。
- `web/src/shared/lib`：HTTP 与 Query Client。
- `web/src/shared/styles/themes/pixel`：Pixel 字体、通用 token 和基础材质。
- `web/src/styles`：全局样式。
- `web/DESIGN_SYSTEM.md`：现代皮肤和全局组件规范。
- `web/PIXEL_DESIGN_SYSTEM.md`：Pixel 皮肤设计与实现规范。

## 运行与验证入口

- [`Makefile`](../../Makefile)：构建、测试、Compose、Agent 和 Web 命令。
- [`docker-compose.yml`](../../docker-compose.yml)：本地拓扑。
- [`deployments/sql/init.sql`](../../deployments/sql/init.sql)：完整数据库 Schema。
- [`scripts/smoke-backend.sh`](../../scripts/smoke-backend.sh)：后端 smoke。
- [`scripts/smoke-agent.sh`](../../scripts/smoke-agent.sh)：Agent 数据链路 smoke。
- [`CONTRIBUTING.md`](../../CONTRIBUTING.md)：贡献与检查流程。
