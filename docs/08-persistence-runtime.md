# 持久化运行态现状

这份文档描述当前代码已经落地的持久化方案，以及还没有打通的部分。

- `master-api` 默认运行态已经是 `pg_redis`
- 旧版内存仓储不再是默认主路径
- 但全系统还没有形成“ingest -> worker -> 状态回写”的完整闭环

## 已经落地的部分

### PostgreSQL

`master-api` 启动时会连接 PostgreSQL，并使用 [`services/master-api/internal/repository/postgres`](../services/master-api/internal/repository/postgres) 作为主机状态仓储。

当前已经实际依赖的持久化能力包括：

- 租户分配
- 主机注册与身份更新
- Agent 实例状态更新
- `host_status_current` 当前快照读写
- 维护窗创建
- 告警 ACK 更新

这意味着：

- 服务重启后，主机当前状态不会像旧版内存实现那样全部丢失
- 租户、主机、标签、状态版本都能从 PostgreSQL 恢复

### Redis

当前 Redis 承担两类运行态：

1. 指标窗口
   - `master-api` 在处理 heartbeat 时，把 16 个指标逐个写入 Redis 列表
   - 默认最多保留 3600 个点
   - 默认 TTL 为 2 小时
2. 事件总线
   - `master-api` 使用 Redis Pub/Sub 广播 `host_upsert / host_delete`
   - `SSE` 连接订阅这条总线获取增量更新

这意味着：

- 多个 `master-api` 实例理论上可以共享同一条主机事件流
- 浏览器首屏 `sync` 可以由 PostgreSQL 当前快照 + Redis 窗口历史拼出来

## 当前真实读写路径

### 注册

```text
agent -> master-api -> PostgreSQL -> Redis Pub/Sub
```

`register` 会更新 PostgreSQL 当前状态，并发布 `host_upsert`。

### Heartbeat

```text
agent -> master-api -> PostgreSQL + Redis(window) + Redis(pubsub)
```

这是当前页面最核心的数据路径：

- 当前快照进 PostgreSQL
- 窗口历史进 Redis
- 增量事件进 Redis Pub/Sub

### 页面读取

```text
browser -> master-api -> PostgreSQL(current) + Redis(history)
```

因此页面展示依赖的是 `master-api`，不是 `ingest-gateway`。

## 还没有打通的部分

### `ingest-gateway`

当前 `ingest-gateway` 仍然只是接入层占位：

- `metrics`：计数并记录日志
- `events`：计数并记录日志
- `probes`：计数并记录日志

它还没有：

- 持久化高频指标
- 把探测结果写回 PostgreSQL
- 把事件投递给 `core-worker`

### `probe-worker`

`probe-worker` 已经会真实探测并上报到 `ingest-gateway`，但当前结果不会继续影响：

- `host_status_current.last_probe_at`
- `reachability_state`
- `overall_state`

所以模型里虽然有 probe 相关字段，当前页面并不能依赖它们。

### `core-worker`

`core-worker` 当前没有消费任何持久化状态或消息流，只是周期性打印占位日志。

它还没有接管：

- 统一状态计算
- 告警规则计算
- probe 调度
- 异步回写

## 当前架构的收益

和旧版内存实现相比，现在已经得到这些直接收益：

- `master-api` 重启后仍能恢复主机当前快照
- 租户和主机信息不再依赖单进程内存
- 窗口指标和 `SSE` 增量事件已经拆到了 Redis

## 当前剩余缺口

如果要把架构继续往前推进，最值得优先做的是：

1. 让 `ingest-gateway` 的 `metrics / events / probes` 进入可消费的持久化或消息队列
2. 让 `core-worker` 消费这些输入并统一计算状态
3. 把 probe 结果并回 `host_status_current`
4. 让页面逐步消费“真实统一状态”，而不只依赖 agent metric batch 派生的当前快照

在这些步骤完成前，当前仓库最可靠的实时监控闭环仍然是：

```text
agent metric batch -> ingest-gateway -> PostgreSQL/Redis -> SSE -> web
```
