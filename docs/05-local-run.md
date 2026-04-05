# 本地启动与验证

## 直接在本机运行

```bash
make build
make run-master
make run-ingest
make run-core
make run-probe
make run-agent
```

## 使用 Docker Compose

推荐本地联调方式：

- `master-api / ingest-gateway / postgres / redis / core-worker / probe-worker` 走 Docker
- `agent` 直接运行在宿主机

```bash
make docker-up
make smoke
make run-agent
make smoke-agent
make docker-logs
make docker-down
```

如果只是想保留“容器里跑 agent”的对比模式：

```bash
make docker-up-full
```

## 关键接口

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/api/v1/hosts
curl http://127.0.0.1:8090/debug/counters
```

## 状态页面

浏览器直接访问：

```text
http://127.0.0.1:8080/
```

页面会通过 SSE 接收 `sync / host_upsert / host_delete` 增量事件，实时展示所有 agent 的状态，并支持按时间窗口查看最近一段时间内的负载曲线。

默认行为：

- 超过 15 秒没有 heartbeat 的 host 会被自动标记为 `OFFLINE`
- 页面会默认把 `OFFLINE` host 置灰并排到列表后面
- 可以通过“只看在线 Agent”开关过滤掉离线主机

SSE 推送端点：

```text
http://127.0.0.1:8080/api/v1/stream/hosts
```

页面现在已经从轮询升级为基于 `EventSource` 的实时推送。

## 说明

- 数据库初始化 SQL 会在 PostgreSQL 容器第一次启动时自动执行。
- 默认 `docker-up` 不会启动容器里的 agent，避免把宿主机测试流程和容器内 agent 混在一起。
- 当前最推荐的测试方式是宿主机直接运行 `make run-agent`，这样看到的 CPU、内存、负载、网络更接近真实宿主机数据。
- 宿主机 agent 默认每秒上报一次，页面时间窗口支持 CPU、内存、磁盘、负载、网络 RX、网络 TX。
- 当前 `master-api` 还没有把状态写入 PostgreSQL/Redis，首版先保证服务、容器、注册与上报链路全部可跑。
- 如果要继续向 README 的完整设计演进，下一步是把 `master-api` 和 `core-worker` 的内存存储替换成 PG/Redis/MQ。
