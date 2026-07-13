# Gaoming 文档

这里记录仓库当前已经实现的行为。设计设想、已完成的迁移步骤和临时方案不再与现状文档混放。

## 项目概要

Gaoming 是一套按租户展示的轻量主机监控系统。当前可用的主链路是：

```text
Agent 采集主机指标
  -> ingest-gateway（gRPC）
  -> PostgreSQL 当前快照 + Redis 短期窗口/事件
  -> master-api（HTTP + SSE）
  -> React Web Dashboard
```

当前已实现：

- Agent 采集 CPU、内存、磁盘、负载和网络指标，并以 gRPC 双向流上报。
- `ingest-gateway` 校验租户、建档主机、更新当前状态和短期指标窗口。
- `master-api` 提供主机查询、SSE、会话读取、用户管理及部分运维接口。
- Web 按租户展示主机状态，并通过 SSE 接收增量更新。
- PostgreSQL 保存权威当前快照，Redis 保存短期指标和实时事件。

当前尚未闭环：

- `probe-worker` 的探测结果只被接收和计数，尚未写入主机状态。
- `core-worker` 仍是占位任务，尚未执行状态计算或告警计算。
- 维护窗和告警表已有部分接口，但没有完整的状态/告警生产链路。
- 仓库内可以解析已有会话，但没有完整的登录和创建会话入口。

## 按重要性阅读

### 1. 核心文档（必读）

| 文档 | 内容 | 适合读者 |
| --- | --- | --- |
| [系统架构](./01-core/architecture.md) | 运行单元、数据流、时序、边界和已知缺口 | 所有人 |

### 2. 操作指南（开发时查阅）

| 文档 | 内容 | 适合读者 |
| --- | --- | --- |
| [本地开发与验证](./02-guides/local-development.md) | 配置、启动、Agent 联调、健康检查 | 开发与运维 |

### 3. 参考资料（按需查阅）

| 文档 | 内容 | 适合读者 |
| --- | --- | --- |
| [接口与数据模型](./03-reference/api-and-data.md) | HTTP、gRPC、SSE、状态码、指标和存储结构 | 前后端开发 |
| [仓库结构](./03-reference/repository-layout.md) | 目录职责和关键代码入口 | 新贡献者 |

前端视觉规则单独维护在 [`web/DESIGN_SYSTEM.md`](../web/DESIGN_SYSTEM.md)，Pixel 皮肤规范见 [`web/PIXEL_DESIGN_SYSTEM.md`](../web/PIXEL_DESIGN_SYSTEM.md)，贡献与检查流程见 [`CONTRIBUTING.md`](../CONTRIBUTING.md)。

## 快速开始

本地推荐组合是：后端和依赖运行在 Docker，Agent 运行在宿主机，Web 由 Compose 中的 Vite 容器提供。

```bash
make up
make smoke
```

另开一个终端运行 Agent：

```bash
make agent
```

再从第三个终端验证上报链路：

```bash
TENANT=default make smoke-agent
```

启动前需要准备被 `.gitignore` 排除的 `config/*.yml` 和根目录 `agent-config.yaml`。完整示例及验证方法见[本地开发与验证](./02-guides/local-development.md)。

常用入口：

- Web：`http://127.0.0.1:5173/default`
- master-api：`http://127.0.0.1:8080/master/healthz`
- ingest-gateway：`http://127.0.0.1:8090/ingest/healthz`

## 文档维护约定

- 文档描述当前代码，不把规划写成已实现能力。
- 接口或运行链路变化时，优先更新核心文档和对应参考文档。
- 临时实施计划放在 Issue/PR；完成后不长期保留在 `docs/`。
- 命令以 [`Makefile`](../Makefile) 中真实存在的 target 为准。
- 配置字段以各服务的 `internal/config/config.go` 为准。
