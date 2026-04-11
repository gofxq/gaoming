# 协议与接口

## 当前主协议

当前运行态分成两类：

- 浏览器查询与 SSE 以 `master-api` 的 HTTP/JSON 为准
- Agent 在缺省租户场景下会先向 `master-api` 申请 `tenant_code`
- Agent 控制面和指标面都以 `ingest-gateway` 的 gRPC 为准

Proto 文件位于 [`api/proto/monitor/v1/`](../api/proto/monitor/v1)，其中 `MetricsIngestService` 已经用于当前服务的指标接入。

## `master-api`

服务前缀固定为 `/master`。

可用接口：

- `GET /master/healthz`
- `GET /master/api/v1/hosts`
- `GET /master/api/v1/hosts/{host_uid}`
- `GET /master/api/v1/stream/hosts`
- `POST /master/api/v1/ops/maintenance`
- `POST /master/api/v1/ops/alerts/{alert_id}/ack`

关键说明：

- `GET /master/api/v1/hosts` 和 `GET /master/api/v1/hosts/{host_uid}` 支持 `?tenant=<tenantCode>`
- `GET /master/api/v1/stream/hosts` 也支持 `?tenant=<tenantCode>`
- `POST /master/api/v1/install/tenant` 用于 Agent 运行时申请 `tenant_code`

## `ingest-gateway`

服务前缀固定为 `/ingest`。

可用接口：

- `GET /ingest/healthz`
- `POST /ingest/api/v1/events`
- `POST /ingest/api/v1/probes`
- `GET /ingest/debug/counters`
- gRPC `monitor.v1.AgentControlService/RegisterAgent`
- gRPC `monitor.v1.MetricsIngestService/PushMetricBatch`
- gRPC `monitor.v1.MetricsIngestService/StreamMetricBatches`

当前行为：

- gRPC `RegisterAgent` 会注册主机并确认 `tenant_code`
- gRPC `StreamMetricBatches` 是当前 Agent 默认指标上报通道
- gRPC `PushMetricBatch` 会更新主机当前状态
- gRPC `PushMetricBatch` 会把最近窗口指标写入 Redis
- gRPC `PushMetricBatch` 会发布 `host_upsert` 事件
- HTTP `events / probes` 目前仍主要用于接入层占位和调试计数

## Agent 上游地址

Agent 运行时需要：

- `MASTER_API_URL`
- `INGEST_GATEWAY_GRPC_ADDR`

默认情况下，远端 gRPC 地址按 TLS 连接；`localhost` / `127.0.0.1` / `::1` 会自动降级为本地明文连接，方便本机联调。

## 关键请求对象

定义见 [`pkg/contracts/api.go`](../pkg/contracts/api.go)。

### Agent 注册

gRPC `monitor.v1.AgentControlService/RegisterAgent`

请求主体由两部分组成：

- `host`
  - 主机身份、地域、系统、标签、租户
- `agent`
  - `agent_id`
  - `version`
  - `capabilities`
  - `boot_time`

响应返回：

- `host_uid`
- `tenant_code`
- `config`

### Metrics / Events / Probes

`ingest-gateway` 当前接收三类写入：

- gRPC `PushMetricBatchRequest`
- gRPC 流 `StreamMetricBatches`
- HTTP `PushEventBatchRequest`
- HTTP `ReportProbeResultsRequest`

其中 `StreamMetricBatches` 是 Agent 默认写入路径，`PushMetricBatchRequest` 保留为兼容 unary 入口。

## SSE 事件

主机流接口为：

```text
GET /master/api/v1/stream/hosts?tenant=<tenantCode>
```

当前支持三类事件：

- `sync`
- `host_upsert`
- `host_delete`

事件载荷结构：

- `sync`
  - `items`: 全量 `HostSnapshot`
  - `histories`: 各主机的窗口历史
  - `latest`: 每个主机每个指标的最新点
  - `server_time`
- `host_upsert`
  - `item`
  - `latest`
  - `server_time`
- `host_delete`
  - `host_uid`
  - `server_time`

当前前端启动时会先拉一次 `GET /master/api/v1/hosts?tenant=...`，但即使这个请求失败，也可以依赖 `sync` 事件完成首屏初始化。

## 前端租户路由

Web SPA 路由位于 [`web/src/app/router.tsx`](../web/src/app/router.tsx)。

当前租户入口是：

- `/:tenantCode`
- `/:tenantCode/pwa`
- `/:tenantCode/pwa/:hostUID`

也就是说：

- 页面路径用 `/:tenantCode`
- API 和 `SSE` 用 `?tenant=...`
