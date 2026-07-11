# 接口与数据模型

本页是当前已挂载接口和主要数据结构的快速参考。完整运行关系见[系统架构](../01-core/architecture.md)。

## master-api HTTP

基础前缀：`/master`；API 前缀：`/master/api/v1`。

| Method | Path | 认证 | 说明 |
| --- | --- | --- | --- |
| GET | `/master/healthz` | 无 | 健康检查 |
| GET | `/master/api/v1/auth/session` | 可选 Cookie | 读取当前会话 |
| POST | `/master/api/v1/auth/logout` | 可选 Cookie | 删除会话并清 Cookie |
| POST | `/master/api/v1/install/tenant` | 无 | 分配安装租户 |
| POST | `/master/api/v1/agents/register` | 无 | HTTP 兼容注册入口，当前 Agent 不使用 |
| GET | `/master/api/v1/hosts?tenant=T` | 无 | 按租户查询主机 |
| GET | `/master/api/v1/hosts/:hostUID?tenant=T` | 无 | 查询单台主机 |
| GET | `/master/api/v1/stream/hosts?tenant=T` | 无 | 主机 SSE |
| POST | `/master/api/v1/ops/maintenance` | Session | 创建维护窗 |
| POST | `/master/api/v1/ops/alerts/:alertID/ack` | Session | 确认告警 |
| GET | `/master/api/v1/admin/users` | Admin | 查询会话租户用户 |
| PATCH | `/master/api/v1/admin/users/:userID` | Admin | 更新同租户用户 |

主机公开接口的租户来自查询参数。受保护接口使用 `gaoming_session` Cookie。

## ingest-gateway

### HTTP

| Method | Path | 行为 |
| --- | --- | --- |
| GET | `/ingest/healthz` | 健康检查 |
| POST | `/ingest/api/v1/events` | 接收事件；当前只计数和记录日志 |
| POST | `/ingest/api/v1/probes` | 接收探测结果；当前只计数和记录日志 |
| GET | `/ingest/debug/counters` | 返回进程内接收计数，重启后清零 |

### gRPC

监听默认端口 `8091`。

| Service / RPC | 类型 | 状态 |
| --- | --- | --- |
| `AgentControlService/RegisterAgent` | Unary | 已实现，兼容入口 |
| `AgentControlService/GetConfig` | Unary | Proto 已定义，未实现 |
| `AgentControlService/Heartbeat` | Unary | Proto 已定义，未实现 |
| `MetricsIngestService/PushMetricBatch` | Unary | 已实现，兼容入口 |
| `MetricsIngestService/StreamMetricBatches` | 双向流 | 已实现，当前 Agent 默认路径 |
| `MetricsIngestService/PushEventBatch` | Unary | 仅计数和日志 |

指标写入时的主要错误映射：租户不存在为 `FailedPrecondition`，主机/Agent 不存在为 `NotFound`，其他错误为 `Internal`。

## SSE

连接：

```http
GET /master/api/v1/stream/hosts?tenant=<tenantCode>
Accept: text/event-stream
```

| 事件 | 载荷 | 说明 |
| --- | --- | --- |
| `sync` | `{items, histories, latest, server_time}` | 建连后的全量当前状态和短期历史 |
| `host_upsert` | `{item, latest, server_time}` | 单台主机增量更新 |
| `host_delete` | `{host_uid, server_time}` | 删除事件；当前租户流的处理仍有限制 |
| 注释心跳 | `: keep-alive` | 每 20 秒保持连接 |

## Web 路由

| 路径 | 页面 | 权限 |
| --- | --- | --- |
| `/` | 跳转 `/default` | 无 |
| `/:tenantCode` | Dashboard | 公开 |
| `/:tenantCode/login` | 登录不可用提示或会话跳转 | 公开 |
| `/:tenantCode/users` | 用户管理 | 已登录且 Admin |

当前代码没有独立移动端/PWA 页面路由。

## 核心读模型

主机读模型是 [`pkg/state/state.go`](../../pkg/state/state.go) 中的 `HostSnapshot`，主要字段分为：

- 身份：`host_uid`、`tenant_code`、hostname、IP、OS、架构、region/env/role、labels。
- 状态：`agent_state`、`reachability_state`、`service_state`、`overall_state`。
- 指标：CPU、内存、Swap、磁盘容量/吞吐/IOPS/inode、load1、网络吞吐/包速率。
- 时间：`last_agent_seen_at`、`last_metric_at`、`last_probe_at`。
- 运维：`open_alert_count`、`version`。

状态码：

| 值 | 名称 |
| --- | --- |
| 0 | `UNKNOWN` |
| 1 | `UP` |
| 2 | `WARNING` |
| 3 | `CRITICAL` |
| 4 | `OFFLINE` |
| 5 | `MAINTENANCE` |
| 6 | `DISABLED` |

当前最可靠的状态信号来自 Agent 指标上报；Probe、告警和维护窗尚未统一参与状态计算。

## 窗口指标

Redis 当前保存 16 个指标键：

```text
cpu_usage_pct
mem_used_pct
mem_available_bytes
swap_used_pct
disk_used_pct
disk_free_bytes
disk_inodes_used_pct
disk_read_bps
disk_write_bps
disk_read_iops
disk_write_iops
load1
net_rx_bps
net_tx_bps
net_rx_packets_ps
net_tx_packets_ps
```

运行时 key 为 `gaoming:metrics:<host_uid>:<metric_key>`，元素是 `{ts,value}` JSON。写入使用 `LPUSH`，裁剪到 60 个点，并设置 2 小时 TTL；读取后转换为时间正序。

## PostgreSQL 表状态

| 表 | 当前状态 |
| --- | --- |
| `tenants` | 活跃；默认租户和安装分配 |
| `hosts`、`host_labels` | 活跃；身份与标签 |
| `agent_instances` | 活跃；Agent 实例和最后活跃时间 |
| `host_status_current` | 活跃；当前快照主读模型 |
| `users`、`user_identities`、`user_sessions` | 会话解析、用户查询/更新可用 |
| `maintenance_windows` | 可创建，尚未参与状态计算 |
| `alert_events` | 可 ACK，当前没有告警生产者 |
| `host_groups`、`host_inventory` | 仅 Schema 定义 |
| `probe_*` | 仅 Schema/Proto 设计，当前不落库 |
| `alert_rules`、`remote_tasks`、`audit_logs` | 仅 Schema 定义 |

## 仅定义、未挂载的服务

以下 Proto 已生成 Go 代码，但没有进程注册对应服务：

- `StatusQueryService`：当前由 master-api HTTP/SSE 替代。
- `OpsService`：当前由 master-api HTTP 运维接口替代。
- `ProbeCoordinatorService`：当前 probe-worker 使用固定 YAML 配置，没有任务租约。
