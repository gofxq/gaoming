# 项目总览

`Gaoming` 当前是一套可本地联调、按租户展示的轻量主机监控系统。

当前代码里已经稳定落地的主路径是：

- `agent` 从本地配置读取 `tenant_code`，并通过 gRPC 持续流式上报指标
- `ingest-gateway` 在收到首个 metric batch 时校验租户是否存在；存在则建档，不存在则拒绝并让 agent 退出
- `ingest-gateway` 将主机当前状态落到 PostgreSQL
- `ingest-gateway` 将最近窗口指标写入 Redis
- `ingest-gateway` 通过 Redis Pub/Sub 发布 `host_upsert` 事件，`master-api` 负责订阅并转发给浏览器
- Web SPA 通过 `SSE` 订阅主机增量事件，并按 `tenant` 展示实时状态

需要特别说明的当前事实：

- 页面上的窗口趋势数据，来自 `metric batch -> ingest-gateway -> Redis` 这条链路
- `ingest-gateway` 当前已经接管 agent 指标驱动的主机状态更新与窗口写入
- `probe-worker` 已经会真实发起 HTTP 探测，但探测结果目前还没有写回 `master-api` 的主机状态
- `core-worker` 仍是占位循环，还没有接管状态计算和告警流程

## 当前推荐运行方式

- `postgres / redis / master-api / ingest-gateway / core-worker / probe-worker` 走 Docker
- `agent` 直接运行在宿主机
- `web` 用 `Vite` 本地开发服务器运行

这样最接近当前仓库真实形态：

- 后端运行态依赖 PostgreSQL 和 Redis
- Agent 采集的是宿主机真实 CPU、内存、磁盘和网络数据
- React SPA 独立开发，不由 `master-api` 直接托管静态文件

## 当前页面能力

Web 位于 [`web/`](../web)，当前主要入口有：

- `/:tenantCode`：桌面 Dashboard
- `/:tenantCode/pwa`：移动端总览
- `/:tenantCode/pwa/:hostUID`：移动端单主机详情

当前页面已经支持：

- 以 `tenant` 为范围的主机列表和详情
- `SSE` 实时刷新
- 最近 1 分钟 / 5 分钟 / 15 分钟 / 1 小时窗口
- CPU、内存、Swap、磁盘容量、磁盘吞吐、磁盘 IOPS、负载、网络吞吐、网络包速率
- 用户本地保存“展示哪些指标”的偏好

## 当前服务边界

- `master-api`
  - 主机查询、维护窗、告警 ACK
  - 从 PostgreSQL 读取主机当前快照
  - 从 Redis 读取主机最近窗口指标
  - 对浏览器提供 `SSE`
- `ingest-gateway`
  - Agent gRPC 注册
  - Agent metric stream 接入
  - 写入主机当前状态与 Redis 指标窗口
- `core-worker`
  - 当前只定时打印占位日志
- `probe-worker`
  - 周期性探测目标 URL，并把结果上报到 `ingest-gateway`
- `agent`
  - 宿主机指标采集
  - gRPC 注册
  - 流式指标上报

## 当前持久化状态

- PostgreSQL
  - `tenants / hosts / host_labels / agent_instances / host_status_current`
  - `maintenance_windows / alert_events` 等运维表
- Redis
  - 每个主机的最近窗口指标
  - `master-api` 的主机事件总线

仍未打通的部分：

- `ingest-gateway` 接收的 metric/event/probe 还没有进入长期存储
- `probe-worker` 结果还没有回写 `host_status_current.last_probe_at`
- `core-worker` 还没有消费任何状态流

## 阅读顺序

- [docs/01-data-model.md](./01-data-model.md)
- [docs/02-contracts.md](./02-contracts.md)
- [docs/03-runtime-flow.md](./03-runtime-flow.md)
- [docs/04-layout.md](./04-layout.md)
- [docs/05-local-run.md](./05-local-run.md)
- [docs/08-persistence-runtime.md](./08-persistence-runtime.md)
