# 项目总览

`Gaoming` 当前是一套可以在本地直接跑起来的监控系统 MVP。

它已经具备这些核心能力：

- `agent` 在宿主机注册到 `master-api`
- `agent` 每秒采集一次主机指标，并同时发送 `metrics` 与 `heartbeat`
- `master-api` 保存主机当前快照和最近 1 小时的指标窗口
- 浏览器通过 SSE 订阅主机增量事件，实时看到状态变化
- `probe-worker` 周期性探测目标并把结果上报到 `ingest-gateway`
- `docker compose` 可以拉起后端依赖和服务，宿主机 agent 单独运行

## 当前推荐运行方式

- `master-api / ingest-gateway / postgres / redis / core-worker / probe-worker` 走 Docker
- `agent` 直接运行在宿主机

这样页面里看到的是宿主机真实 CPU、内存、磁盘、网络，而不是容器视角的混合数据。

## 当前页面能力

状态页入口：

```text
http://127.0.0.1:8080/
```

当前页面支持：

- SSE 增量推送
- 暂停/恢复视图更新
- 只看在线 Agent
- `OFFLINE` 自动置灰和排序靠后
- 最近 1 分钟 / 5 分钟 / 15 分钟 / 1 小时窗口
- 同时展示 CPU、内存、磁盘用量、磁盘读、磁盘写、负载、网络 RX、网络 TX

## 当前服务边界

- `master-api`：控制面、主机状态、运维接口、SSE 状态页
- `ingest-gateway`：指标、事件、探测结果接入与计数
- `core-worker`：占位实现，为后续状态引擎和告警引擎预留
- `probe-worker`：周期性 HTTP 探测
- `agent`：宿主机指标采集、注册、heartbeat、metric batch 上报

## 当前存储状态

项目已经带有 PostgreSQL 初始化脚本和 Redis 容器，但主机实时状态目前仍然是 `master-api` 进程内内存存储。

也就是说：

- 服务、容器、注册、上报、SSE 页面都已经可用
- 持久化和真正的告警/调度引擎还没有完全落地

## 阅读顺序

- [docs/01-data-model.md](/home/u/dev/github.com/gofxq/gaoming/docs/01-data-model.md)
- [docs/02-contracts.md](/home/u/dev/github.com/gofxq/gaoming/docs/02-contracts.md)
- [docs/03-runtime-flow.md](/home/u/dev/github.com/gofxq/gaoming/docs/03-runtime-flow.md)
- [docs/04-layout.md](/home/u/dev/github.com/gofxq/gaoming/docs/04-layout.md)
- [docs/05-local-run.md](/home/u/dev/github.com/gofxq/gaoming/docs/05-local-run.md)
- [docs/08-persistence-runtime.md](/home/u/dev/github.com/gofxq/gaoming/docs/08-persistence-runtime.md)
- [docs/99-status-roadmap.md](/home/u/dev/github.com/gofxq/gaoming/docs/99-status-roadmap.md)

整体定位

这四个模块可以按职责分成四层：

master-api 是控制面和状态面，负责“谁在线、当前状态是什么、页面看什么”。
ingest-gateway 是数据接入面，负责“谁来上报数据、先把数据收进来”。
core-worker 是后台异步处理面，预留给“状态计算、告警、调度”这类不适合放在请求链路里的工作。
probe-worker 是主动探测面，负责“系统自己去打目标，补充 agent 之外的可用性信号”。
这个划分在 docs/00-summary.md 和 docs/03-runtime-flow.md 里是一致的，本质上是在把“控制”、“接入”、“计算”、“探测”拆开，避免所有职责都塞进一个服务。

模块作用

master-api

它是系统的主控入口，处理 Agent 注册、Heartbeat、主机查询、运维接口和 SSE 状态推送，接口定义可以看 docs/02-contracts.md。
从实现看，它直接维护主机快照、指标历史、订阅推送、离线判定这些状态能力，核心服务在 services/master-api/internal/service/service.go，当前存储是内存仓库 services/master-api/internal/repository/memory/store.go。
所以它更像“当前运行态的事实来源”。页面看到的主机列表、在线/离线状态、最近时间窗口指标，本质上都是它在组织和输出。
ingest-gateway

它的职责是统一接收写入流量，包括指标批、事件批、探测结果。对应接口是 POST /api/v1/metrics、POST /api/v1/events、POST /api/v1/probes，见 docs/02-contracts.md。
从代码看，它现在做的事很克制：接收请求、计数、打日志、返回 ack，核心逻辑在 services/ingest-gateway/internal/service/service.go。
这说明它当前更像“接入口”而不是“处理引擎”。好处是把高频上报和控制面解耦；代价是现阶段它还没有沉淀成真正的数据管道或持久化链路。
core-worker

它代表后台异步 worker 的位置，未来适合承接状态引擎、告警引擎、探测调度等任务，这一点日志里已经直接写出来了，见 services/core-worker/internal/service/runner.go。
目前它还是占位实现，只是周期性 tick，并没有真正处理任务。
也就是说，设计上它是系统的“计算大脑”，但在当前版本里还没落地业务。
probe-worker

它做的是主动探测，不依赖 agent 上报宿主机指标，而是自己周期性请求目标 URL，记录成功/失败、状态码、延迟，再把结果提交给 ingest-gateway。
实现非常直接：定时发 HTTP GET，构造 ProbeResult，再 POST 到报告地址，见 services/probe-worker/internal/service/runner.go。
这个模块的价值在于补上“外部视角”的监控信号。Agent 解决主机内部状态，probe-worker 解决服务可达性和探测延迟，它们不是替代关系，而是互补关系。
它们之间怎么配合

当前主链路在 docs/03-runtime-flow.md 里写得很清楚：

agent 向 master-api 注册并持续发 heartbeat。
agent 同时把指标批发到 ingest-gateway。
master-api 更新主机当前快照，并通过 SSE 推给页面。
probe-worker 周期性探测目标，再把探测结果发到 ingest-gateway。
core-worker 未来应该消费这些状态和事件，做更重的后台决策。
如果一句话概括：

master-api 管“当前状态和控制”。
ingest-gateway 管“数据接入”。
core-worker 管“后台计算”。
probe-worker 管“主动探测”。
从仓库现状看，真正已经承担核心业务的是 master-api 和 probe-worker，ingest-gateway 现在更偏轻量入口，core-worker 还处在架构占位阶段。
