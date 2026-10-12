# 接口与数据模型

本页是当前已挂载接口和主要数据结构的快速参考。完整运行关系见[系统架构](../01-core/architecture.md)。

## master-api HTTP

基础前缀：`/master`；API 前缀：`/master/api/v1`。

| Method | Path | 认证 | 说明 |
| --- | --- | --- | --- |
| GET | `/master/healthz` | 无 | 健康检查 |
| GET | `/master/api/v1/auth/setup-status` | 无 | 查看管理员初始化是否可用 |
| POST | `/master/api/v1/auth/setup` | 引导凭据 + Origin | 一次性创建初始管理员并建立会话 |
| POST | `/master/api/v1/auth/login` | 本地密码 + Origin | 登录指定空间，轮换浏览器旧会话 |
| GET | `/master/api/v1/auth/csrf` | Session | 获取会话派生 CSRF token |
| GET | `/master/api/v1/auth/session` | 可选 Cookie | 读取当前会话 |
| POST | `/master/api/v1/auth/logout` | Session + Origin + CSRF | 删除会话并清 Cookie |
| POST | `/master/api/v1/install/tenant` | 无 | 分配安装租户 |
| POST | `/master/api/v1/agents/register` | 无 | HTTP 兼容注册入口，当前 Agent 不使用 |
| GET | `/master/api/v1/hosts` | Session | 会话空间的主机（可选 tenant 只能匹配会话） |
| GET | `/master/api/v1/hosts/:hostUID` | Session | 查询自己空间的主机 |
| GET | `/master/api/v1/hosts/:hostUID/metrics/latest` | Session | 私有动态指标，先校验主机归属 |
| GET | `/master/api/v1/stream/hosts` | Session | 会话空间 SSE，周期权限重验 |
| POST | `/master/api/v1/ops/maintenance` | Admin + Origin/CSRF | 创建自身空间维护窗并审计 |
| POST | `/master/api/v1/ops/alerts/:alertID/ack` | Session | 确认告警 |
| GET | `/master/api/v1/admin/users` | Admin | 查询会话租户用户 |
| PATCH | `/master/api/v1/admin/users/:userID` | Admin | 更新同租户用户 |
| POST/GET | `/master/api/v1/shares` | Admin | 创建/列出受限分享；写入需 Origin/CSRF |
| DELETE | `/master/api/v1/shares/:shareID` | Admin + Origin/CSRF | 撤销自身空间分享和全部兑换会话 |
| POST | `/master/api/v1/public/shares/exchange` | 分享 token + Origin | POST body 兑换窄路径 Cookie |
| GET | `/master/api/v1/public/shares/session` | Share Cookie | 查看允许字段和范围数量 |
| GET | `/master/api/v1/public/shares/hosts`、`/hosts/:hostUID`、`/hosts/:hostUID/metrics/latest`、`/stream/hosts` | Share Cookie | 受限主机/字段投影，非私有 API |
| GET | `/master/api/v1/public/demo/session`、`/hosts`、`/hosts/:hostUID`、`/hosts/:hostUID/metrics/latest`、`/stream/hosts` | 显式 demo 配置 | 默认关闭，固定租户/主机/字段白名单 |

表中 shares/demo 同一行的后缀路径均继承该行的完整公开 namespace，不是私有 `/hosts` 的别名。

默认主机接口均为私有，空间来自已验证会话，省略 tenant 不扩大范围；伪造其他 tenant 返回 403。分享 Cookie 不能访问这些私有接口，所有私有/分享响应 no-store。范围与运维审计见[访问与分享](../02-guides/access-and-sharing.md)。

所有受保护写操作额外要求允许的 Origin 和 `X-CSRF-Token`，认证响应不允许缓存。Cookie 默认 Secure/HttpOnly/SameSite=Lax，过期、用户禁用或租户停用后会话失效。初始化只在尚无用户且未完成引导时开放；本地恢复命令不开放为 HTTP 接口。配置、密码限制和请求限流见[登录手册](../02-guides/authentication.md)。私有读取、受限分享和运维范围已在 T03 接通；Agent 接入鉴权仍在 T04 中处理。

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
| `/:tenantCode` | Dashboard | 已登录且匹配会话空间 |
| `/:tenantCode/login` | 本地密码登录，安全站内回跳 | 公开 |
| `/:tenantCode/setup` | 一次性初始管理员引导 | 引导凭据 |
| `/:tenantCode/users` | 用户管理 | 已登录且 Admin |
| `/:tenantCode/shares` | 分享管理 | 已登录且 Admin |
| `/share` | 受限只读分享 | fragment 兑换后的 Share Cookie |
| `/demo` | 显式演示 | 配置主机/字段白名单 |

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
| `users`、`user_identities`、`user_sessions` | 登录会话、用户查询/更新可用 |
| `user_passwords`、`auth_bootstrap_state` | 租户内本地密码身份、一次性初始化状态 |
| `access_shares`、`access_share_hosts`、`access_share_sessions` | 哈希分享凭据、可信范围、窄会话与撤销 |
| `maintenance_windows` | 可创建，尚未参与状态计算 |
| `alert_events` | 可 ACK，当前没有告警生产者 |
| `host_groups`、`host_inventory` | 仅 Schema 定义 |
| `probe_*` | 仅 Schema/Proto 设计，当前不落库 |
| `alert_rules`、`remote_tasks`、`audit_logs` | 仅 Schema 定义 |

## 仅定义、未挂载的服务

以下 Proto 已生成 Go 代码，但没有进程注册对应服务：

- `StatusQueryService`：当前由 master-api HTTP/SSE 替代。
- `OpsService`：当前由 master-api HTTP 运维接口替代。
- `ProbeCoordinatorService`：当前内置 probe 模块使用统一 YAML 配置，没有任务租约。
