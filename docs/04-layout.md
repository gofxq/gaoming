# 仓库结构

## 顶层目录

- `agent/`
  - 宿主机 Agent
- `api/proto/`
  - 预留中的 proto 契约
- `deployments/`
  - Dockerfile、SQL 初始化、安装和部署脚本
- `docs/`
  - 项目说明文档和计划
- `pkg/`
  - 共享模型、工具包
- `scripts/`
  - smoke、reload 等辅助脚本
- `services/`
  - 后端服务实现
- `web/`
  - React + Vite 前端

## `services/` 结构

### `services/master-api`

- `cmd/server`
  - 进程入口
- `internal/app`
  - 装配 PostgreSQL、Redis、Service、HTTP Server
- `internal/config`
  - 环境变量配置
- `internal/repository/postgres`
  - 主机状态、维护窗、告警 ACK 的 PostgreSQL 实现
- `internal/repository/redis`
  - 指标窗口和主机事件总线的 Redis 实现
- `internal/repository/memory`
  - 旧版内存实现，当前不在默认运行路径上
- `internal/service`
  - 注册、heartbeat、查询、离线对账
- `internal/transport/http`
  - `/master/*` HTTP 和 SSE 路由

### `services/ingest-gateway`

- `cmd/server`
- `internal/app`
- `internal/config`
- `internal/service`
  - metric/event/probe 接收计数
- `internal/transport/grpc`
  - Agent metrics gRPC 接入
- `internal/transport/http`
  - `/ingest/*` HTTP 路由（health、events、probes、debug）

### `services/core-worker`

- `cmd/worker`
- `internal/app`
- `internal/config`
- `internal/service`
  - 当前只有周期性占位 runner

### `services/probe-worker`

- `cmd/worker`
- `internal/app`
- `internal/config`
- `internal/service`
  - HTTP 探测与结果上报

## `agent/daemon` 结构

- `cmd/agent`
  - Agent 入口
- `internal/app`
  - 进程装配
- `internal/config`
  - 读取 `.env` 与 `agent-config.yaml`
- `internal/identity`
  - 主机身份采集
- `internal/service`
  - 注册、metrics、heartbeat、系统采样

## `web/` 结构

- `src/app`
  - Router、Provider、应用装配
- `src/components/layout`
  - 页面壳层
- `src/pages/dashboard`
  - 桌面 Dashboard 和实时数据订阅
- `src/pages/mobile`
  - PWA 风格移动端页面
- `src/styles`
  - 全局样式
- `vite.config.ts`
  - 开发代理与构建配置

## `pkg/` 共享包

- `pkg/contracts`
  - HTTP/JSON 契约
- `pkg/state`
  - 主机快照和窗口指标模型
- `pkg/clock`
  - 时间抽象
- `pkg/httpx`
  - HTTP 编解码工具
- `pkg/ids`
  - ID 生成
- `pkg/logx`
  - 日志初始化

## 运行相关文件

- [`Makefile`](../Makefile)
  - 本地开发、测试、Docker、Web 命令入口
- [`docker-compose.yml`](../docker-compose.yml)
  - 本地 Docker 拓扑
- [`deployments/sql/init.sql`](../deployments/sql/init.sql)
  - PostgreSQL 初始化表结构
- [`web/README.md`](../web/README.md)
  - 前端本地开发说明

## 当前实现取向

这个仓库当前已经不是“架构草图”。

它的真实状态更接近：

- `master-api` 有真实 PostgreSQL / Redis 依赖
- `agent` 可以跑真实宿主机采样
- `web` 已经是独立 React SPA
- `ingest-gateway / core-worker / probe-worker` 的工程边界已经有了，但后两段业务链路还没有完全打通
