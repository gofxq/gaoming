# 运行链路

## 当前最推荐的运行拓扑

本地联调时，推荐组合是：

1. `master-api`
2. `ingest-gateway`
3. `core-worker`
4. `probe-worker`
5. `postgres`
6. `redis`
7. 宿主机上的 `agent`

其中后端服务走 Docker，`agent` 直接跑宿主机。

## 实际时序

1. `master-api` 启动，暴露控制面、状态页和 SSE 推送端点。
2. `ingest-gateway` 启动，接收指标批、事件批和探测结果。
3. 宿主机 `agent` 启动，先向 `master-api` 注册。
4. `agent` 每秒采样一次主机指标，并在同一个周期里同时发送：
   - `metrics` 到 `ingest-gateway`
   - `heartbeat` 到 `master-api`
5. `master-api` 更新主机当前快照，并把增量变更通过 SSE 推给页面。
6. `probe-worker` 周期性探测目标，把结果发给 `ingest-gateway`。
7. 如果某个 host 超过 15 秒没有 heartbeat，`master-api` 会把它标成 `OFFLINE`。

## 当前页面链路

浏览器访问 `/` 后会：

1. 渲染状态页 HTML
2. 建立 `EventSource("/api/v1/stream/hosts")`
3. 接收 `sync / host_upsert / host_delete`
4. 重绘主机表和时间窗口指标卡片

页面端还支持：

- 暂停/恢复视图更新
- 暂停期间积压事件计数
- 只看在线 Agent

## 当前可观察结果

- `GET /api/v1/hosts`：当前 host 快照
- `GET /api/v1/stream/hosts`：SSE 增量事件
- `GET /debug/counters`：ingest-gateway 接收计数
