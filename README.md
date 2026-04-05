# Gaoming

一个按 README 设计文档落地的监控系统 monorepo。当前仓库已经补齐：

- 拆分后的设计文档
- gRPC proto 契约
- PostgreSQL 初始化脚本
- 5 个可构建的 Go 进程
- Docker Compose 本地运行栈
- 一键构建、启动、测试的 Makefile

## 文档索引

- [docs/01-data-model.md](/home/u/dev/github.com/gofxq/gaoming/docs/01-data-model.md)
- [docs/02-contracts.md](/home/u/dev/github.com/gofxq/gaoming/docs/02-contracts.md)
- [docs/03-runtime-flow.md](/home/u/dev/github.com/gofxq/gaoming/docs/03-runtime-flow.md)
- [docs/04-layout.md](/home/u/dev/github.com/gofxq/gaoming/docs/04-layout.md)
- [docs/05-local-run.md](/home/u/dev/github.com/gofxq/gaoming/docs/05-local-run.md)
- [docs/06-repository-hygiene.md](/home/u/dev/github.com/gofxq/gaoming/docs/06-repository-hygiene.md)

## 快速开始

本机构建：

```bash
make fmt
make build
make test
```

Docker 启动：

```bash
make docker-up
make smoke
make docker-ps
make docker-logs
make docker-down
```

常用校验：

```bash
make check
make compose-config
```

## 当前实现范围

当前版本优先让项目“完整跑起来”，因此先实现：

- `master-api`：Agent 注册、Heartbeat、主机查询、运维接口。
- `ingest-gateway`：指标、事件、探测接入与计数。
- `core-worker`：合并后的后台 worker 占位。
- `probe-worker`：周期性 HTTP 探测并上报结果。
- `agent`：自动注册、上报 heartbeat 和 metric batch。

README 原始的大型设计内容已经拆分到 `docs/` 中继续维护。
