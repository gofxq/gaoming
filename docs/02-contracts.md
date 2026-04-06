# 协议与接口

## 当前真实生效的接口风格

当前首版运行栈以 HTTP/JSON 为准，直接服务本地联调和 Docker 运行。

proto 契约仍然保留在：

- [api/proto/monitor/v1/common.proto](/home/u/dev/github.com/gofxq/gaoming/api/proto/monitor/v1/common.proto)
- [api/proto/monitor/v1/agent.proto](/home/u/dev/github.com/gofxq/gaoming/api/proto/monitor/v1/agent.proto)
- [api/proto/monitor/v1/probe.proto](/home/u/dev/github.com/gofxq/gaoming/api/proto/monitor/v1/probe.proto)
- [api/proto/monitor/v1/query.proto](/home/u/dev/github.com/gofxq/gaoming/api/proto/monitor/v1/query.proto)
- [api/proto/monitor/v1/ops.proto](/home/u/dev/github.com/gofxq/gaoming/api/proto/monitor/v1/ops.proto)

但它们目前更像后续演进目标，而不是当前运行态主协议。

## 当前可用的 HTTP 接口

### `master-api`

- `GET /master/healthz`
- `POST /master/api/v1/agents/register`
- `POST /master/api/v1/agents/heartbeat`
- `GET /master/api/v1/hosts`
- `GET /master/api/v1/hosts/{host_uid}`
- `GET /master/api/v1/stream/hosts`
- `POST /master/api/v1/ops/maintenance`
- `POST /master/api/v1/ops/alerts/{alert_id}/ack`

### `ingest-gateway`

- `GET /ingest/healthz`
- `POST /ingest/api/v1/metrics`
- `POST /ingest/api/v1/events`
- `POST /ingest/api/v1/probes`
- `GET /ingest/debug/counters`

## 核心数据契约

当前主机 heartbeat digest 里已经包含这些指标：

- `cpu_usage_pct`
- `mem_used_pct`
- `disk_used_pct`
- `disk_read_bps`
- `disk_write_bps`
- `load1`
- `net_rx_bps`
- `net_tx_bps`

也就是说，前端页面看到的主机实时状态已经不再只是 load 或网络，而是完整的多指标快照。

## SSE 增量事件

状态页通过 `GET /api/v1/stream/hosts` 接收三类事件：

- `sync`
- `host_upsert`
- `host_delete`

页面首次连接时会收到 `sync` 全量快照，之后只接收增量变更。
