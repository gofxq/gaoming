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

```bash
make docker-up
make smoke
make docker-logs
make docker-down
```

## 关键接口

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/api/v1/hosts
curl http://127.0.0.1:8090/debug/counters
```

## 说明

- 数据库初始化 SQL 会在 PostgreSQL 容器第一次启动时自动执行。
- 当前 `master-api` 还没有把状态写入 PostgreSQL/Redis，首版先保证服务、容器、注册与上报链路全部可跑。
- 如果要继续向 README 的完整设计演进，下一步是把 `master-api` 和 `core-worker` 的内存存储替换成 PG/Redis/MQ。
