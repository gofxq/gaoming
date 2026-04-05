# 仓库结构

## 顶层目录

- `agent/daemon`：宿主机 agent
- `api/proto`：保留中的 proto 契约
- `deployments/docker`：通用 Docker 构建文件
- `deployments/sql`：PostgreSQL 初始化脚本
- `docs`：项目说明文档
- `pkg`：共享数据结构和工具包
- `scripts`：本地 smoke 和辅助脚本
- `services`：后端服务实现

## `services/` 结构

- `services/master-api`
  - `cmd/server`
  - `internal/app`
  - `internal/config`
  - `internal/repository`
  - `internal/service`
  - `internal/transport`
- `services/ingest-gateway`
  - `cmd/server`
  - `internal/app`
  - `internal/config`
  - `internal/service`
  - `internal/transport`
- `services/core-worker`
  - `cmd/worker`
  - `internal/app`
  - `internal/config`
  - `internal/service`
- `services/probe-worker`
  - `cmd/worker`
  - `internal/app`
  - `internal/config`
  - `internal/service`

## `agent/daemon` 结构

- `cmd/agent`：启动入口
- `internal/app`：装配入口
- `internal/config`：agent 配置
- `internal/identity`：主机身份采集
- `internal/service`：注册、heartbeat、指标采集与上报

## `pkg/` 共享包

- `pkg/contracts`：HTTP/JSON 契约
- `pkg/state`：主机快照和指标窗口模型
- `pkg/clock`：时间抽象
- `pkg/httpx`：HTTP 小工具
- `pkg/ids`：ID 生成
- `pkg/logx`：日志初始化

## 当前实现取向

当前仓库优先保证：

- `go build ./...` 和 `go test ./...` 可直接通过
- 后端可用 Docker 跑起来
- 宿主机 agent 可直接联到本地后端
- 页面和接口已经能看到真实主机状态

也就是说，这个仓库已经不是只有设计图，而是“可以直接运行的 MVP + 后续演进骨架”。
