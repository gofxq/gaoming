# 持久化运行态替换方案

这份文档聚焦一个具体目标：把 `master-api` 当前依赖进程内内存的运行态，替换成基于 PostgreSQL 和 Redis 的可恢复、可扩展实现。

它回答四个问题：

- 当前内存实现到底承载了什么。
- 哪些数据应该进 PostgreSQL，哪些能力应该放到 Redis。
- `master-api`、`ingest-gateway`、`core-worker` 之间的职责边界应该怎么收敛。
- 这件事应该按什么顺序落地，才能在不打断现有页面能力的前提下平滑迁移。

## 当前问题在哪里

现在 `master-api` 的内存仓库同时承担了四类职责：

1. 主机注册与当前快照存储
2. 最近 1 小时指标窗口存储
3. maintenance / alert ack 等轻量运行态存储
4. SSE 订阅广播源

对应代码集中在 [services/master-api/internal/repository/memory/store.go](/home/u/dev/github.com/gofxq/gaoming/services/master-api/internal/repository/memory/store.go)。

这套实现适合 MVP，但有三个直接问题：

- 服务重启后，主机状态、指标窗口、订阅状态全部丢失。
- 多实例部署时，每个实例都有自己的内存视图，无法共享状态，也无法统一做 SSE 广播。
- `postgres` 和 `redis` 虽然已经在运行拓扑里存在，但没有进入主路径，导致它们只是“准备好了”，而不是“真的被依赖”。

## 迁移目标

迁移后的目标不是“把 map 换成数据库”这么简单，而是把当前内存里混在一起的职责拆开。

目标状态应该是：

- PostgreSQL 保存可恢复、可查询、可审计的事实数据。
- Redis 承担高频缓存、短期窗口、SSE 广播和多实例之间的状态分发。
- `master-api` 负责控制面 API 和状态查询，但不再自己持有唯一真相。
- `ingest-gateway` 负责高频写入接入，不把所有运行态逻辑继续塞回 `master-api`。
- `core-worker` 逐步接管状态计算和运行态聚合。

## 数据分层建议

### 应该进入 PostgreSQL 的数据

这些数据需要持久化、可追溯、可恢复：

- `hosts`
  负责主机身份信息，例如 `host_uid`、`hostname`、`primary_ip`、region、env、role。
- `agent_instances`
  负责 agent 维度的实例状态、版本、最后心跳、配置版本、能力集。
- `host_status_current`
  负责主机当前状态快照，是页面和查询接口的核心读模型。
- `maintenance_windows`
  负责维护窗定义。
- `alert_events`
  负责告警事件、ack 流程和状态闭环。
- `audit_logs`
  负责运维行为审计。

这部分数据的特点是“恢复后必须还在”，因此不能只放 Redis。

### 应该优先进入 Redis 的数据

这些能力更适合做高频、短期、广播型运行态：

- 最近 1 小时指标窗口
  可以先按 `host_uid + metric_key` 存成带 TTL 的时间窗口，支撑页面时序图。
- SSE 广播通道
  多实例 `master-api` 需要共享增量消息来源，Redis Pub/Sub 或 Stream 都比进程内 watcher 合适。
- 短期热点查询缓存
  例如“当前所有主机快照”这类高频读请求，可以从 PostgreSQL 同步到 Redis。
- 离线判定或状态变更事件的分发
  让状态变更先进入 Redis 事件流，再由各实例或 worker 消费。

这部分数据的特点是“高频、临时、适合分发”，因此适合 Redis。

### 仍然不建议只放 Redis 的数据

下面这些看起来也像运行态，但不应该只放 Redis：

- 主机身份信息
- 当前状态最终值
- 维护窗定义
- 告警 ack 结果

原因很简单：这些数据会被 API 查询、会影响业务语义、也需要重启后恢复。

## 目标读写路径

### Agent 注册

建议路径：

1. `agent -> master-api /api/v1/agents/register`
2. `master-api` upsert `hosts`
3. `master-api` upsert `agent_instances`
4. `master-api` 初始化或更新 `host_status_current`
5. `master-api` 把“host upsert”事件发布到 Redis
6. SSE 订阅实例消费 Redis 事件并推给浏览器

这里 PostgreSQL 负责事实落库，Redis 负责广播。

### Heartbeat

建议路径：

1. `agent -> master-api /api/v1/agents/heartbeat`
2. `master-api` 更新 `agent_instances.last_seen_at`
3. `master-api` 更新 `host_status_current` 中当前指标、状态、版本号、时间戳
4. `master-api` 把短期指标窗口写入 Redis
5. `master-api` 发布 `host_upsert` 事件到 Redis
6. 所有 SSE 实例从 Redis 消费增量事件

这里的关键点是：

- 当前状态最终值进 PostgreSQL。
- 页面需要的短期历史先保存在 Redis，而不是立即引入完整时序库。

### 查询主机列表和单主机详情

建议路径：

- `GET /api/v1/hosts` 从 `host_status_current + hosts` 读
- `GET /api/v1/hosts/{host_uid}` 从同样的读模型读取
- 若需要最近 1 小时曲线，再从 Redis 拉取指标窗口

这样可以把“当前状态查询”和“短期时序查询”拆开。

### SSE 推送

当前实现的问题是 watcher 在单进程内维护。

建议改成：

1. 状态变化后，发布结构化事件到 Redis
2. 每个 `master-api` 实例维护自己的 HTTP SSE 连接
3. 每个实例订阅 Redis 事件流
4. 新连接建立时，先从 PostgreSQL + Redis 组装 `sync`
5. 增量阶段只转发 Redis 里的 `host_upsert / host_delete`

这样 SSE 不再依赖单实例内存。

## 服务职责收敛建议

### `master-api`

迁移后应承担：

- agent 注册
- heartbeat 接入
- 主机状态查询
- 运维接口
- SSE 输出

迁移后不应继续承担：

- 唯一状态存储
- 长期指标窗口保管
- 单实例 watcher 广播

### `ingest-gateway`

迁移后应承担：

- metrics / events / probes 的统一写入入口
- 将高频数据转成可持久化或可消费的内部事件
- 给 `core-worker` 或后续存储链路提供稳定入口

如果这一步不推进，`master-api` 仍然会背负过多状态更新逻辑。

### `core-worker`

迁移后应逐步承担：

- 离线判定
- overall_state / severity 统一计算
- 告警规则计算
- probe 调度与结果聚合

这意味着后续 `host_status_current` 的最终写入者，长期看更应该是 `core-worker`，而不是所有状态都在 `master-api` 里直接写死。

## 建议的数据落点

| 能力 | PostgreSQL | Redis | 说明 |
| --- | --- | --- | --- |
| 主机注册信息 | 是 | 否 | `hosts`、`agent_instances` 需要持久化 |
| 当前状态快照 | 是 | 可选缓存 | 页面和接口的核心读模型 |
| 最近 1 小时指标窗口 | 否 | 是 | 先用 Redis 解决实时页面需要 |
| SSE 增量事件 | 否 | 是 | 适合 Redis Pub/Sub 或 Stream |
| maintenance / alert ack | 是 | 可选缓存 | 业务语义数据必须可恢复 |
| 全量 hosts 列表缓存 | 否 | 是 | 可作为热点读缓存 |

## 建议的迁移顺序

### 第一步：引入持久化仓储接口

先不要直接删除内存实现，而是先抽象 repository 接口。

建议先把这些能力抽象出来：

- `HostRepository`
- `AgentRepository`
- `HostStatusRepository`
- `MetricWindowRepository`
- `EventBus`

这样可以做到：

- 保留当前 memory 实现继续工作
- 新增 PostgreSQL / Redis 实现并行接入
- 逐步替换 service 层调用，而不是一次性重写

### 第二步：先迁移主机当前状态

优先级最高的是 `hosts + agent_instances + host_status_current`。

原因：

- 这是页面和查询接口的最小闭环。
- 只要这部分落库，服务重启后主机状态就不再完全丢失。
- 后续 SSE 和 worker 都能围绕这个读模型扩展。

### 第三步：迁移指标窗口和 SSE 事件总线

把当前内存 `histories` 和 `watchers` 替换成 Redis：

- 指标窗口进入 Redis
- `host_upsert / host_delete` 事件进入 Redis
- SSE 改成“连接在本地，事件在 Redis”

这一步完成后，多实例部署才有真正意义。

### 第四步：把离线判定和状态计算移给 `core-worker`

当前 `ReconcileOffline` 在 `master-api` 内完成，短期可以保留，但中期应该迁给 `core-worker`。

原因：

- 离线判定本质上是状态计算任务。
- 后面还会叠加 probe 状态、维护窗、告警抑制等逻辑。
- 这些逻辑继续堆在 API 服务里会越来越难维护。

### 第五步：再处理 maintenance / alert / remote task 等闭环

这部分依赖前面的运行态模型稳定之后再推进更合理，否则容易在不稳定的状态基础上继续叠功能。

## 对当前代码的直接改造建议

结合当前实现，第一批最值得改的点是：

1. 把 [services/master-api/internal/repository/memory/store.go](/home/u/dev/github.com/gofxq/gaoming/services/master-api/internal/repository/memory/store.go) 拆成更细的仓储接口，而不是一个大一统 `Store`。
2. 把 [services/master-api/internal/service/service.go](/home/u/dev/github.com/gofxq/gaoming/services/master-api/internal/service/service.go) 里的 `memory.Store` 直接依赖改成接口依赖。
3. 把 [services/master-api/internal/transport/http/stream.go](/home/u/dev/github.com/gofxq/gaoming/services/master-api/internal/transport/http/stream.go) 的订阅源从进程内 watcher 切成事件总线接口。
4. 让 `sync` 事件从“全量内存快照 + 全量内存历史”改成“PostgreSQL 当前状态 + Redis 短期窗口”组装。

## 这件事完成后的直接收益

如果这部分落地，项目会立刻获得这些收益：

- `master-api` 重启后仍能恢复主机当前状态。
- 多实例部署开始具备意义。
- PostgreSQL / Redis 从“摆在那里”变成“真正运行时依赖”。
- 页面实时能力可以保留，不需要为了持久化牺牲现在的可视化体验。
- 后续告警、调度、运维闭环有了稳定地基。

## 当前建议结论

如果要把这项工作再压缩成一句话：

不是把当前内存 `Store` 直接换成数据库，而是把“事实数据、短期窗口、事件广播、状态计算”四件事拆开，再分别落到 PostgreSQL、Redis 和 `core-worker`。
