# 数据模型与当前存储状态

## 当前真正落地的数据模型

当前运行时最核心的数据对象是 `HostSnapshot`，定义在：

- [pkg/state/state.go](/home/u/dev/github.com/gofxq/gaoming/pkg/state/state.go)

它承载页面和控制面的实时主机状态，主要包括：

- 主机身份：`host_uid`、`hostname`、`primary_ip`
- 状态码：`agent_state`、`reachability_state`、`service_state`、`overall_state`
- 当前指标：CPU、内存、磁盘用量、磁盘读写、负载、网络 RX/TX
- 时间戳：最后 agent 心跳、最后指标时间、最后探测时间
- 版本号：每次 heartbeat 或状态变化都会递增

## 指标窗口

`master-api` 当前会在内存里为每个 host 保留最近 1 小时的指标点：

- `cpu_usage_pct`
- `mem_used_pct`
- `disk_used_pct`
- `disk_read_bps`
- `disk_write_bps`
- `load1`
- `net_rx_bps`
- `net_tx_bps`

这些窗口数据用于前端状态页的时间序列卡片。

## 状态码约定

- `0 UNKNOWN`
- `1 UP`
- `2 WARNING`
- `3 CRITICAL`
- `4 OFFLINE`
- `5 MAINTENANCE`
- `6 DISABLED`

## 当前存储实现

当前仓库里，实时主机状态仍然是内存存储：

- `master-api` 使用内存仓库保存 `HostSnapshot`
- SSE 订阅直接基于内存状态广播增量事件
- 超过 15 秒没有 heartbeat 的 host 会被标记为 `OFFLINE`

这意味着当前版本优先解决的是“跑起来”和“可观察”，不是完整持久化。

## 已接好的持久化基础设施

仓库已经带有 PostgreSQL 初始化脚本：

- [deployments/sql/init.sql](/home/u/dev/github.com/gofxq/gaoming/deployments/sql/init.sql)

初始化脚本里已经包含未来版本会接入的核心表，例如：

- `hosts`
- `agent_instances`
- `host_status_current`
- `probe_targets`
- `probe_jobs`
- `probe_results`
- `alert_rules`
- `alert_events`
- `maintenance_windows`
- `audit_logs`

## 现阶段结论

- 主机实时快照：已落地，内存实现
- 时序窗口：已落地，内存实现
- PostgreSQL/Redis：容器和初始化脚本已接好
- 真正的持久化读写：还没有完全切入运行态
