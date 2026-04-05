# 项目总览

`Gaoming` 当前是一套可以在本地直接跑起来的监控系统 MVP。

它已经具备这些核心能力：

- `agent` 在宿主机注册到 `master-api`
- `agent` 每秒采集一次主机指标，并同时发送 `metrics` 与 `heartbeat`
- `master-api` 保存主机当前快照和最近 1 小时的指标窗口
- 浏览器通过 SSE 订阅主机增量事件，实时看到状态变化
- `probe-worker` 周期性探测目标并把结果上报到 `ingest-gateway`
- `docker compose` 可以拉起后端依赖和服务，宿主机 agent 单独运行

## 当前推荐运行方式

- `master-api / ingest-gateway / postgres / redis / core-worker / probe-worker` 走 Docker
- `agent` 直接运行在宿主机

这样页面里看到的是宿主机真实 CPU、内存、磁盘、网络，而不是容器视角的混合数据。

## 当前页面能力

状态页入口：

```text
http://127.0.0.1:8080/
```

当前页面支持：

- SSE 增量推送
- 暂停/恢复视图更新
- 只看在线 Agent
- `OFFLINE` 自动置灰和排序靠后
- 最近 1 分钟 / 5 分钟 / 15 分钟 / 1 小时窗口
- 同时展示 CPU、内存、磁盘用量、磁盘读、磁盘写、负载、网络 RX、网络 TX

## 当前服务边界

- `master-api`：控制面、主机状态、运维接口、SSE 状态页
- `ingest-gateway`：指标、事件、探测结果接入与计数
- `core-worker`：占位实现，为后续状态引擎和告警引擎预留
- `probe-worker`：周期性 HTTP 探测
- `agent`：宿主机指标采集、注册、heartbeat、metric batch 上报

## 当前存储状态

项目已经带有 PostgreSQL 初始化脚本和 Redis 容器，但主机实时状态目前仍然是 `master-api` 进程内内存存储。

也就是说：

- 服务、容器、注册、上报、SSE 页面都已经可用
- 持久化和真正的告警/调度引擎还没有完全落地

## 阅读顺序

- [docs/01-data-model.md](/home/u/dev/github.com/gofxq/gaoming/docs/01-data-model.md)
- [docs/02-contracts.md](/home/u/dev/github.com/gofxq/gaoming/docs/02-contracts.md)
- [docs/03-runtime-flow.md](/home/u/dev/github.com/gofxq/gaoming/docs/03-runtime-flow.md)
- [docs/04-layout.md](/home/u/dev/github.com/gofxq/gaoming/docs/04-layout.md)
- [docs/05-local-run.md](/home/u/dev/github.com/gofxq/gaoming/docs/05-local-run.md)
