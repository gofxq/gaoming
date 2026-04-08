# 数据模型

## 核心读模型

当前页面和查询接口围绕 [`pkg/state/state.go`](../pkg/state/state.go) 里的 `HostSnapshot` 展开。

这个结构承载了当前系统最重要的运行态：

- 主机身份
  - `host_uid`
  - `tenant_code`
  - `hostname`
  - `primary_ip`
- 状态码
  - `agent_state`
  - `reachability_state`
  - `service_state`
  - `overall_state`
- 当前指标
  - CPU、内存、Swap
  - 磁盘容量、吞吐、IOPS、inode 使用率
  - `load1`
  - 网络吞吐、网络包速率
- 时间戳
  - `last_agent_seen_at`
  - `last_metric_at`
  - `last_probe_at`
- 其他
  - `labels`
  - `open_alert_count`
  - `version`

## 指标键

当前真实支持的窗口指标键共有 16 个：

- `cpu_usage_pct`
- `mem_used_pct`
- `mem_available_bytes`
- `swap_used_pct`
- `disk_used_pct`
- `disk_free_bytes`
- `disk_inodes_used_pct`
- `disk_read_bps`
- `disk_write_bps`
- `disk_read_iops`
- `disk_write_iops`
- `load1`
- `net_rx_bps`
- `net_tx_bps`
- `net_rx_packets_ps`
- `net_tx_packets_ps`

这些定义同样在 [`pkg/state/state.go`](../pkg/state/state.go)。

## 状态码约定

状态码使用 `pkg/state.Code`：

- `0 UNKNOWN`
- `1 UP`
- `2 WARNING`
- `3 CRITICAL`
- `4 OFFLINE`
- `5 MAINTENANCE`
- `6 DISABLED`

当前离线判定逻辑由 `master-api` 周期任务执行：超过 15 秒没有 `heartbeat` 的主机会被标记为 `OFFLINE`。

## 契约模型

Agent 与服务之间的 HTTP/JSON 契约位于 [`pkg/contracts/api.go`](../pkg/contracts/api.go)。

其中最重要的几类对象是：

- `RegisterAgentRequest / Response`
- `HeartbeatRequest / Response`
- `PushMetricBatchRequest`
- `PushEventBatchRequest`
- `ReportProbeResultsRequest`
- `AckResponse`

`HeartbeatRequest.Digest` 与 `HostSnapshot` 的多数指标字段是一一对应的。当前页面展示的窗口趋势，正是由这份 `digest` 衍生出来。

## 当前存储落点

### PostgreSQL

初始化表定义在 [`deployments/sql/init.sql`](../deployments/sql/init.sql)，当前运行时已实际依赖其中这些表：

- `tenants`
- `hosts`
- `host_labels`
- `agent_instances`
- `host_status_current`
- `maintenance_windows`
- `alert_events`

其中：

- `hosts` 保存主机身份
- `agent_instances` 保存 agent 实例和配置版本
- `host_status_current` 保存当前快照，是页面和查询接口的主读模型

### Redis

当前 Redis 主要承载两类运行态：

- 最近窗口指标
  - key 形态为 `gaoming:metrics:<host_uid>:<metric_key>`
- 主机事件总线
  - channel 默认是 `gaoming:master-api:host-events`

## 当前未完全兑现的字段

模型里有些字段已经预留，但当前链路还没有真正写满：

- `last_probe_at`
  - `probe-worker` 已经会探测，但结果目前只到 `ingest-gateway`
- `reachability_state`
  - 当前还没有 probe 回写或统一状态计算引擎
- `open_alert_count`
  - 表结构已在，但当前还没有真正的告警流水驱动它持续变化

所以目前最可靠的实时信号仍然是：

- `register`
- `heartbeat`
- `heartbeat.digest`
