# 运行链路

## 当前推荐拓扑

本地联调建议拆成三层：

1. Docker 后端
   - `postgres`
   - `redis`
   - `master-api`
   - `ingest-gateway`
   - `core-worker`
   - `probe-worker`
2. 宿主机 Agent
   - `agent`
3. 前端开发服务器
   - `web` (`Vite`)

## 实际时序

### 1. `master-api` 启动

启动时会：

- 校验 `MASTER_API_RUNTIME_BACKEND=pg_redis`
- 连接 PostgreSQL
- 连接 Redis
- 初始化 PostgreSQL 仓储和 Redis 仓储
- 启动每 5 秒一次的离线对账任务

### 2. `agent` 注册

`agent` 首次启动后调用：

```text
POST /master/api/v1/agents/register
```

当前服务端会：

- upsert 主机身份到 PostgreSQL
- upsert agent 实例到 PostgreSQL
- 初始化或更新 `host_status_current`
- 发布 `host_upsert` 到 Redis 事件总线

如果服务端分配了 `tenant_code`，agent 会把返回值写回本地配置文件。

### 3. 周期采集与上报

每个采集周期里，`agent` 会做两件事：

1. 把 metric batch 发给 `ingest-gateway`
2. 把 heartbeat 发给 `master-api`

注意这里的当前真实行为：

- `ingest-gateway` 只负责接收、计数、日志和 `ack`
- 页面展示所需的当前快照与窗口历史，实际上来自 `heartbeat.digest`

也就是说，当前 Dashboard 的主数据面不依赖 `ingest-gateway`。

### 4. `master-api` 处理 heartbeat

`master-api` 收到 heartbeat 后会：

- 更新 PostgreSQL 中的 agent 实例状态
- 更新 `host_status_current`
- 把 `digest` 中的 16 个指标写入 Redis 窗口
- 发布 `host_upsert` 事件到 Redis

### 5. 浏览器接入实时流

Web 页面会：

1. 先请求 `GET /master/api/v1/hosts?tenant=...`
2. 再建立 `EventSource("/master/api/v1/stream/hosts?tenant=...")`

SSE 首次连接时，`master-api` 会组装：

- PostgreSQL 中的主机快照
- Redis 中的各主机历史窗口
- 每个主机的最新指标点

然后发送一条 `sync` 事件。之后页面继续接收 `host_upsert` 增量事件。

### 6. 离线判定

`master-api` 的后台任务每 5 秒执行一次离线对账。

当主机满足：

- `last_agent_seen_at` 不为空
- 且早于 `now - 15s`

就会被标记为 `OFFLINE`，并再次发布 `host_upsert` 事件。

### 7. `probe-worker`

`probe-worker` 当前会：

- 定时请求 `PROBE_TARGET_URL`
- 生成 `ProbeResult`
- POST 到 `PROBE_REPORT_URL`

但当前这条链路只到 `ingest-gateway` 为止，还没有把探测结果并回主机当前状态。

### 8. `core-worker`

`core-worker` 当前只会按配置周期打印一次占位日志：

- `status-engine`
- `alert-engine`
- `probe-scheduler`

这些 pipeline 还没有实际实现。

## 当前可观察结果

- `GET /master/healthz`
- `GET /master/api/v1/hosts?tenant=default`
- `GET /master/api/v1/stream/hosts?tenant=default`
- `GET /ingest/debug/counters`

## 当前最重要的边界

为了避免误读仓库，当前需要记住这三点：

- 主机实时状态的权威来源是 `master-api + PostgreSQL`
- 主机最近窗口历史的来源是 `master-api + Redis`
- `ingest-gateway / probe-worker / core-worker` 还没有闭合成真正的数据处理链路
