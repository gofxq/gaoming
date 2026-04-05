# 协议与接口

## gRPC proto

README 中的协议已经拆成独立 proto 文件：

- [api/proto/monitor/v1/common.proto](/home/u/dev/github.com/gofxq/gaoming/api/proto/monitor/v1/common.proto)
- [api/proto/monitor/v1/agent.proto](/home/u/dev/github.com/gofxq/gaoming/api/proto/monitor/v1/agent.proto)
- [api/proto/monitor/v1/probe.proto](/home/u/dev/github.com/gofxq/gaoming/api/proto/monitor/v1/probe.proto)
- [api/proto/monitor/v1/query.proto](/home/u/dev/github.com/gofxq/gaoming/api/proto/monitor/v1/query.proto)
- [api/proto/monitor/v1/ops.proto](/home/u/dev/github.com/gofxq/gaoming/api/proto/monitor/v1/ops.proto)

当前首版运行栈为了离线可构建，服务间走 HTTP/JSON MVP 接口，proto 作为后续切换到 gRPC 的契约基础。

## 当前可用的 HTTP MVP 接口

`master-api`

- `GET /healthz`
- `POST /api/v1/agents/register`
- `POST /api/v1/agents/heartbeat`
- `GET /api/v1/hosts`
- `GET /api/v1/hosts/{host_uid}`
- `POST /api/v1/ops/maintenance`
- `POST /api/v1/ops/alerts/{alert_id}/ack`

`ingest-gateway`

- `GET /healthz`
- `POST /api/v1/metrics`
- `POST /api/v1/events`
- `POST /api/v1/probes`
- `GET /debug/counters`
