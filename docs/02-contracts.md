# 协议与接口

## 当前主协议

当前运行态分成两类：

- 控制面以 HTTP/JSON 为准
- Agent 指标上报以 gRPC 为准

Proto 文件位于 [`api/proto/monitor/v1/`](../api/proto/monitor/v1)，其中 `MetricsIngestService` 已经用于当前服务的指标接入。

## `master-api`

服务前缀固定为 `/master`。

可用接口：

- `GET /master/healthz`
- `POST /master/api/v1/install/tenant`
- `POST /master/api/v1/agents/register`
- `POST /master/api/v1/agents/heartbeat`
- `GET /master/api/v1/hosts`
- `GET /master/api/v1/hosts/{host_uid}`
- `GET /master/api/v1/stream/hosts`
- `POST /master/api/v1/ops/maintenance`
- `POST /master/api/v1/ops/alerts/{alert_id}/ack`

关键说明：

- `GET /master/api/v1/hosts` 和 `GET /master/api/v1/hosts/{host_uid}` 支持 `?tenant=<tenantCode>`
- `GET /master/api/v1/stream/hosts` 也支持 `?tenant=<tenantCode>`
- Agent 注册时如果未指定 `tenant_code`，服务端可以分配租户并回写到响应里

## `ingest-gateway`

服务前缀固定为 `/ingest`。

可用接口：

- `GET /ingest/healthz`
- `POST /ingest/api/v1/events`
- `POST /ingest/api/v1/probes`
- `GET /ingest/debug/counters`
- gRPC `monitor.v1.MetricsIngestService/PushMetricBatch`

当前行为很轻：

- 校验并接收请求
- 增加计数器
- 记录日志
- 返回 `ack`

它现在还不会把这些数据再写回 `master-api` 或持久化存储。

## Agent URL 兼容规则

Agent 对上游 URL 做了兼容处理，以下写法都能被自动归一化：

- `http://127.0.0.1:8080`
- `http://127.0.0.1:8080/master`
- `http://127.0.0.1:8080/master/api/v1`

Agent 指标上报不再使用 `ingest-gateway` 的 HTTP 基地址，而是直接连接 `INGEST_GATEWAY_GRPC_ADDR`。

默认情况下，远端 gRPC 地址按 TLS 连接；`localhost` / `127.0.0.1` / `::1` 会自动降级为本地明文连接，方便本机联调。

## 关键请求对象

定义见 [`pkg/contracts/api.go`](../pkg/contracts/api.go)。

### Agent 注册

`POST /master/api/v1/agents/register`

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

### Heartbeat

`POST /master/api/v1/agents/heartbeat`

请求重点字段：

- `host_uid`
- `agent_id`
- `seq`
- `ts`
- `digest`

`digest` 是当前页面实时主机快照和历史窗口的核心输入。

### Metrics / Events / Probes

`ingest-gateway` 当前接收三类写入：

- gRPC `PushMetricBatchRequest`
- HTTP `PushEventBatchRequest`
- HTTP `ReportProbeResultsRequest`

这些写入当前主要用于接入层占位和调试计数，不会直接驱动 Dashboard。

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
