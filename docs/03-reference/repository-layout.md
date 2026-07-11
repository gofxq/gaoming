# 仓库结构

## 顶层目录

| 目录 | 职责 |
| --- | --- |
| `agent/daemon` | 宿主机 Agent |
| `api/proto`、`api/gen` | gRPC Proto 与生成代码 |
| `deployments` | Dockerfile、SQL、Agent 安装与服务脚本 |
| `docs` | 当前架构、指南和参考文档 |
| `pkg` | 跨服务共享模型、仓储和工具 |
| `scripts` | smoke 与本地开发辅助脚本 |
| `services` | 后端进程 |
| `web` | React + Vite 前端 |

## 后端服务

各服务遵循相近结构：`cmd` 是进程入口，`internal/app` 负责装配，`internal/transport` 负责协议，`internal/service` 负责业务。

- `services/master-api`
  - `internal/app`：装配 PostgreSQL、Redis、认证仓储和 HTTP Server。
  - `internal/auth`：用户、身份和会话模型/仓储。
  - `internal/service`：查询、运维与认证业务。
  - `internal/transport/http`：`/master/*` 路由和 SSE。
- `services/ingest-gateway`
  - `internal/app`：装配共享 PostgreSQL/Redis 仓储及离线任务。
  - `internal/service`：注册、metric batch、事件和 probe 接收。
  - `internal/transport/grpc`：Agent gRPC 接入。
  - `internal/transport/http`：health、events、probes 和 counters。
- `services/core-worker`：当前为周期占位 runner。
- `services/probe-worker`：固定目标 HTTP 探测与结果上报。

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

`services/master-api/internal/repository` 是旧仓储实现，当前 `master-api/internal/app` 不使用它。

## Web

- `web/src/app`：Router 和应用级 Provider。
- `web/src/components/layout`：应用壳层。
- `web/src/features/hosts`：主机模型和实时数据 Hook。
- `web/src/pc`：桌面 Dashboard。
- `web/src/pages/auth`、`web/src/pages/admin`：登录提示和用户管理。
- `web/src/lib`：HTTP 与 Query Client。
- `web/src/styles`：全局样式。
- `web/DESIGN_SYSTEM.md`：前端视觉与组件规范。

## 运行与验证入口

- [`Makefile`](../../Makefile)：构建、测试、Compose、Agent 和 Web 命令。
- [`docker-compose.yml`](../../docker-compose.yml)：本地拓扑。
- [`deployments/sql/init.sql`](../../deployments/sql/init.sql)：完整数据库 Schema。
- [`scripts/smoke-backend.sh`](../../scripts/smoke-backend.sh)：后端 smoke。
- [`scripts/smoke-agent.sh`](../../scripts/smoke-agent.sh)：Agent 数据链路 smoke。
- [`CONTRIBUTING.md`](../../CONTRIBUTING.md)：贡献与检查流程。
