# 方案、完成状态与演进目标

这份文档用于记录 `Gaoming` 当前已经落地的能力、仍未完成的部分，以及后续建议按什么顺序推进。

它不是替代 [docs/00-summary.md](/home/u/dev/github.com/gofxq/gaoming/docs/00-summary.md)、[docs/01-data-model.md](/home/u/dev/github.com/gofxq/gaoming/docs/01-data-model.md)、[docs/02-contracts.md](/home/u/dev/github.com/gofxq/gaoming/docs/02-contracts.md)、[docs/03-runtime-flow.md](/home/u/dev/github.com/gofxq/gaoming/docs/03-runtime-flow.md) 的详细设计，而是站在“项目管理和演进决策”的视角做汇总。

## 项目方案总览

`Gaoming` 当前的定位是一套“可以在本地完整跑起来”的监控系统 MVP。

当前方案把系统拆成五类角色：

- `master-api`：控制面和状态面，负责 agent 注册、heartbeat、主机快照、SSE 页面、运维接口。
- `ingest-gateway`：写入接入层，负责接收指标、事件和探测结果。
- `core-worker`：后台异步处理层，未来承接状态引擎、告警引擎和调度逻辑。
- `probe-worker`：主动探测层，负责周期性 HTTP 探测并上报结果。
- `agent`：宿主机采集器，负责注册、heartbeat 和指标上报。

当前最推荐的运行拓扑是：

- `master-api / ingest-gateway / postgres / redis / core-worker / probe-worker` 走 Docker
- `agent` 直接运行在宿主机

这样做的目标很明确：

- 先验证“注册 -> 上报 -> 状态页 -> 探测”这条主链路已经跑通。
- 先把服务边界、接口和运行方式稳定下来。
- 再把持久化、告警、调度、多租户等能力逐步接回运行态。

## 当前完成状态

| 领域 | 当前状态 | 说明 |
| --- | --- | --- |
| 本地构建与运行链路 | 已完成 | `make build`、`make test`、`make docker-up`、`make smoke`、`make run-agent` 已成体系。 |
| Agent 注册与心跳 | 已完成 | `agent` 可以注册到 `master-api`，并持续上报 heartbeat。 |
| 宿主机指标采集 | 已完成 | 当前 agent 已采集 CPU、内存、磁盘、负载、网络等指标。 |
| 主机状态页与 SSE 推送 | 已完成 | 浏览器可通过 `GET /` 和 `GET /api/v1/stream/hosts` 实时看到主机变化。 |
| 主机实时快照与 1 小时指标窗口 | 已完成 | 已落在 `master-api` 内存状态里，但还不是持久化实现。 |
| 探测结果上报 | 已完成 | `probe-worker` 已能周期性探测目标并把结果发到 `ingest-gateway`。 |
| `ingest-gateway` 接入面 | 部分完成 | 已能接收 metrics/events/probes、计数、返回 ack，但还没有真正持久化或下游消费。 |
| `core-worker` 后台处理 | 占位完成 | 进程和循环框架已在，但还没有真正实现状态计算、告警、调度。 |
| 运维接口骨架 | 部分完成 | maintenance 和 alert ack 的接口已预留，但没有完整闭环。 |
| PostgreSQL / Redis 基础设施 | 基础已就绪 | Docker 容器和初始化 SQL 已存在，但还没有全面接入运行态读写。 |
| 协议演进目标 | 已预留 | proto 契约已保留，但当前实际运行协议仍然是 HTTP/JSON。 |
| 工程化基础设施 | 已完成 | `.editorconfig`、`.gitignore`、`CONTRIBUTING.md`、`make check` 等基础已经补齐。 |

## 当前已经明确但尚未完成的事情

### 1. 把内存运行态替换成真正的持久化实现

这是当前最关键的缺口。

现在 `master-api` 里的主机快照、指标窗口、SSE 广播都建立在进程内内存之上，意味着：

- 服务重启后状态会丢失。
- 多实例扩展没有共享状态基础。
- `postgres` 和 `redis` 还没有真正成为运行态依赖。

下一步需要完成的事情：

- 把 `hosts`、`agent_instances`、`host_status_current` 等表真正接入 `master-api`。
- 明确哪些数据进 PostgreSQL，哪些缓存或广播能力放到 Redis。
- 保留当前页面能力的同时，把内存状态切成可恢复、可扩展的状态模型。

详细拆解见 [docs/08-persistence-runtime.md](/home/u/dev/github.com/gofxq/gaoming/docs/08-persistence-runtime.md)。

### 2. 让 `ingest-gateway` 不只是接入口

当前 `ingest-gateway` 的价值主要是把高频写入从控制面拆出去，但它还没有成为真正的数据入口层。

下一步需要完成的事情：

- 明确 metrics、events、probes 的落库路径。
- 明确是否需要引入消息队列，还是先由 `ingest-gateway` 直接写入存储。
- 把当前 `/debug/counters` 这种“接到了多少”的可观察性，扩展成“数据已被处理到哪里”的链路状态。

### 3. 真正实现 `core-worker`

`core-worker` 现在还是一个结构占位，未来它应该是后台计算中心。

下一步需要完成的事情：

- 状态引擎：根据 heartbeat、metric、probe 结果更新 `overall_state`、`severity` 等状态。
- 告警引擎：加载 `alert_rules`，生成和更新 `alert_events`。
- 探测调度：根据 `probe_policies`、`probe_targets`、`probe_jobs` 去调度 `probe-worker`。

### 4. 把探测从“单目标演示”升级为“可配置调度”

当前 `probe-worker` 更像一个演示版探测器，目标地址由环境变量给定，功能上已经证明链路可跑，但还不是完整探测系统。

下一步需要完成的事情：

- 让探测目标来自 `probe_targets` 和 `probe_policies`。
- 让 `probe_jobs` 驱动实际执行，而不是固定单目标循环。
- 补齐多区域、多目标、失败重试、租约控制等机制。

### 5. 把运维能力做成闭环

从 SQL 初始化脚本可以看到，长期目标不仅仅是看状态，还包括操作和治理能力。

下一步需要完成的事情：

- maintenance window 真正影响状态计算和告警抑制。
- alert ack 写入持久化状态，并能回显到页面或查询接口。
- `remote_tasks`、`audit_logs` 从“表结构预留”走向真实功能。

### 6. 补上多租户和资源模型

当前 schema 已经为 `tenants`、`host_groups`、`labels`、`inventory` 预留了长期模型，但运行态还没有真正启用这些抽象。

下一步需要完成的事情：

- 明确首版到底是否要单租户先跑通，还是直接做可扩展的数据模型。
- 如果继续单租户优先，至少要把数据访问层设计成将来能平滑接 tenant scope。
- 把 host group、label、inventory 接入查询和过滤能力。

## 建议的演进顺序

### 阶段 1：完整跑起来

当前状态：已完成

阶段目标：

- 服务和依赖可启动
- agent 可注册和上报
- 页面可实时看到主机状态
- probe 结果可进入系统

这部分就是当前仓库已经做到的 MVP 能力。

### 阶段 2：持久化接入运行态

当前状态：未完成

阶段目标：

- `master-api` 从内存状态切到 PostgreSQL / Redis
- `ingest-gateway` 具备真正的写入和下游投递职责
- 页面和状态查询不依赖单进程内存

这是最应该优先推进的一步，因为它决定当前系统是“演示型运行栈”还是“可持续扩展的运行栈”。

### 阶段 3：状态引擎和告警引擎

当前状态：未完成

阶段目标：

- `core-worker` 根据 heartbeat、指标和 probe 结果统一生成状态
- 告警规则进入实际计算
- alert 生命周期和 ack 流程完整闭环

这一步完成后，系统才算从“可观察面板”升级到“监控系统”。

### 阶段 4：探测调度和运维闭环

当前状态：未完成

阶段目标：

- 探测目标和策略由数据模型驱动
- worker 具备调度、租约和重试能力
- maintenance、audit、remote task 形成完整运维能力

### 阶段 5：多租户与产品化

当前状态：未完成

阶段目标：

- tenant、group、label、inventory 等模型进入实际使用
- 查询、过滤、权限、审计逐步完整
- 系统从本地 MVP 走向长期可维护的产品化架构

## 当前建议优先级

如果只选最重要的三件事，建议按这个顺序推进：

1. 接入持久化，把 `master-api` 的内存运行态迁到 PostgreSQL / Redis。
2. 让 `ingest-gateway` 和 `core-worker` 形成真正的数据处理闭环。
3. 把 probe、alert、maintenance 三条链路接成完整状态与运维闭环。

## 文档维护建议

后续每次架构推进时，建议同步更新这份文档里的两部分：

- “当前完成状态”：反映哪些能力已经从设计变成运行态事实。
- “当前已经明确但尚未完成的事情”：反映下一步真实要做的事，而不是泛泛愿景。
