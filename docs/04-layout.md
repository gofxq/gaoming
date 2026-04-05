# 仓库结构

## 顶层目录

- `api/proto`：README 拆出的 gRPC 契约。
- `pkg`：共享包。
- `services/master-api`：控制面、状态查询、运维接口。
- `services/ingest-gateway`：指标、事件、探测接入。
- `services/core-worker`：合并后的后台 worker。
- `services/probe-worker`：HTTP 探测执行器。
- `agent/daemon`：主机侧 agent。
- `deployments/sql`：数据库初始化脚本。
- `deployments/docker`：通用 Docker 构建文件。
- `docs`：从 README 拆出来的设计与运行说明。

## 当前实现说明

为了让项目先完整跑起来，仓库当前优先实现：

- 单体 root module，保证 `go build ./...` 可直接通过。
- 离线友好的 HTTP/JSON MVP 接口。
- PostgreSQL 与 Redis 容器化依赖。
- 后续仍然保留 proto 合约，便于继续切回 README 中规划的 gRPC 拆分。
