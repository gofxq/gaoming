# 运行链路

## 当前落地的最小链路

本仓库按 README 的“最小可落地组合”先收敛到 5 个进程：

1. `agent`
2. `master-api`
3. `ingest-gateway`
4. `core-worker`
5. `probe-worker`

## 时序

1. `master-api` 启动后暴露控制面与状态查询接口。
2. `ingest-gateway` 启动后接收指标、事件、探测结果。
3. `agent` 注册到 `master-api`，随后周期性发送 heartbeat 和 metric batch。
4. `probe-worker` 周期性探测 `master-api /healthz`，并把结果上报到 `ingest-gateway`。
5. `core-worker` 先作为合并后的后台 worker 占位，负责后续扩展 `status-engine + alert-engine + probe-scheduler`。

## 当前状态展示

- `GET /api/v1/hosts` 可以看到 agent 注册后的实时主机列表。
- `GET /debug/counters` 可以看到 ingest-gateway 已接收的 metric/event/probe 批次数。
