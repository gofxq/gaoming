# 数据模型与存储分层

README 中的数据建模内容已经拆到本目录，便于后续按模块维护。

## 存储分层

- PostgreSQL：租户、主机、分组、Agent 注册、配置、告警规则、维护窗口、审计。
- Redis：当前主机状态快照、分组汇总、打开中的告警计数。
- TSDB：CPU、内存、磁盘、网络、探测时延等时序指标。
- ClickHouse 或 PostgreSQL 分区表：`probe_results`、`alert_events`、`audit_logs` 等大体量流水。

## 状态码约定

- `0 UNKNOWN`
- `1 UP`
- `2 WARNING`
- `3 CRITICAL`
- `4 OFFLINE`
- `5 MAINTENANCE`
- `6 DISABLED`

## 初始化脚本

当前仓库已经提供 PostgreSQL 初始化脚本：

- [deployments/sql/init.sql](/home/u/dev/github.com/gofxq/gaoming/deployments/sql/init.sql)

它覆盖 README 中定义的核心表：

- `tenants`
- `host_groups`
- `hosts`
- `host_group_rel`
- `host_labels`
- `host_inventory`
- `agent_instances`
- `host_status_current`
- `probe_policies`
- `probe_targets`
- `probe_jobs`
- `probe_results`
- `alert_rules`
- `alert_events`
- `maintenance_windows`
- `remote_tasks`
- `audit_logs`
