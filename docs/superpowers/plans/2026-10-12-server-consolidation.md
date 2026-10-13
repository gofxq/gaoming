# Server Consolidation Implementation Plan

> 后续调整：用户要求移除全部离线管理员与 Go 迁移逻辑。当前只保留 `server/cmd/server`；数据库由用户直接管理，见 `docs/02-guides/database-setup.md`。


> **For agentic workers:** Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** 将后端源码、入口和部署统一到顶层 server，并更新 AGENTS.md。
**Architecture:** 单一在线 server 和离线 admin，内部模块复用现有业务，保持 HTTP/gRPC 与存储行为。
**Tech Stack:** Go、PostgreSQL、Redis、Docker Compose。
**Spec:** docs/superpowers/specs/2026-10-12-server-consolidation.md

## Global Constraints

保留已有工作区修改、Agent 独立、认证行为不变，不提交本机配置或密钥。

## Review Focus

- 新源码不被 config 忽略规则排除。
- admin 和 server 默认使用同一配置文件。
- Docker Web/Agent 和 probe 均访问同一 server。
- 迁移必须先于 server 启动。
- 匿名主机读取继续返回 401，认证 Agent 上报继续成功。

### Task 1: 迁移源码与入口

- [x] 确认现有测试基线；记录默认配置路径的回归测试。
- [x] 将统一 cmd/app/config 移到 server，admin 移到 server/cmd/admin。
- [x] 四个模块移到 server/internal，删除独立 cmd/app/config，仅保留 master 配置加载能力。
- [x] 更新所有 Go 导入、Make/CI/Air 引用与忽略规则，执行 make test/build。

### Task 2: 部署与文档

- [x] Dockerfile 构建统一 server/admin，Compose 用单一后端替代四服务。
- [x] 更新 Agent/Web 地址、Docker 配置示例、隔离 smoke 和 reload 脚本。
- [x] 更新 AGENTS.md、README 与现行指南的结构、命令和部署说明。
- [x] 执行 make compose-config、隔离 Docker smoke 和 make smoke；检查差异与安全边界。

## Execution Record

默认配置回归测试先失败（仍读 master-api.yml），迁移后通过。make test、make build、make fmt-check、make compose-config 已通过。沙箱限制监听端口，测试在已获批准的执行环境运行，Go 构建缓存置于 /tmp。

保留已有工作区改动，不创建额外工作树或提交。原业务代码仅迁移导入路径，独立装配和启动入口移除。独立审阅未发现阻断级代码回归；旧端口说明已修正。Docker 标准联网构建因 proxy.golang.org TLS 握手超时未完成；使用本机公开依赖缓存，在 /tmp 增加 vendor 的临时构建上下文完成隔离验证，生产 Dockerfile 未增加环境专用配置。

隔离 Docker 迁移、管理员登录、匿名主机读取 401、Agent 上报及内置 probe 均通过，计数持续增长。仓库 Compose 拓扑也通过隔离 make docker-up、make smoke 和 make docker-down，临时容器与数据库卷已清理。

## 用户追加要求：移除离线管理员与迁移逻辑

已删除 cmd/admin、pkg/migrations 和嵌入迁移 SQL；删除启动版本校验、CLI 专用账号创建接口、Make 管理目标、Docker admin 二进制及迁移容器。在线登录、用户管理、setup 与空库启动管理员初始化继续保留。

测试夹具直接执行完整 init.sql。全量 Go 测试在真实隔离 PostgreSQL/Redis 下通过（含原生统一后端集成），Go 构建、Compose 校验、格式检查通过。使用本机公开依赖缓存的隔离 Docker 联调通过，覆盖新库初始化、登录、匿名 401、Agent 指标增长和 probe。发现并修复 PostgreSQL Unix socket 健康检查过早放行临时初始化实例的问题，改为 TCP 检查。

独立审阅无阻断问题；已修正首次启动文档顺序。直接维护数据库必须保持租户关联并保存 bcrypt 密码哈希。
