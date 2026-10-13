# 系统架构

本文描述 Gaoming 当前代码的实际运行方式。接口和 SQL 中尚未接入运行时的设计会明确标为“未闭环”。

## 系统总图

```mermaid
flowchart LR
    Agent[宿主机 Agent]
    Web[React Web：Docker :8080 / 开发 :5173]
    subgraph Server["server：单一进程，共享连接池"]
        Master["master HTTP/SSE :8080"]
        Ingest["ingest HTTP :8080 / gRPC :8091"]
        Probe[probe HTTP 探测]
        Probe --> Ingest
    end
    PG[(PostgreSQL)]
    Redis[(Redis)]
    Agent --> Ingest
    Ingest --> PG
    Ingest --> Redis
    Master --> PG
    Master --> Redis
    Master -->|SSE| Web
    Web --> Master
```

当前可靠闭环是：

```text
Agent metric batch
  -> ingest-gateway
  -> PostgreSQL + Redis
  -> master-api SSE
  -> Web Dashboard
```

`probe-worker` 可以发起探测并上报，但结果没有继续落库。

## 运行单元

唯一后端入口为 `server/main.go`。以下 master、ingest、probe 是内部模块，均在同一进程中运行。

| 单元 | 默认端口/入口 | 职责 | 状态 |
| --- | --- | --- | --- |
| Web | Docker `:8080` / 开发 `:5173` | 租户路由、主机展示、SSE 实时更新、用户管理 | 已实现 |
| Agent | 主动连接 gRPC | 主机身份发现、系统采样、指标流上报 | 已实现 |
| master-api | HTTP `:8080` | 查询、SSE、会话解析、用户与运维接口 | 已实现 |
| ingest-gateway | 共享 HTTP `:8080`、gRPC `:8091` | Agent 接入、快照写入、指标窗口、离线判定 | 已实现 |
| probe-worker | 主动发起 HTTP | 固定目标探测与结果上报 | 部分实现 |
| PostgreSQL | Compose 宿主 `:35432` | 租户、主机、当前状态、用户和运维数据 | 已实现 |
| Redis | Compose 宿主 `:36379` | 短期指标列表和主机事件 Pub/Sub | 已实现 |

## 核心运行时序

### 首包建档与指标上报

Agent 不先调用独立注册接口，而是直接建立 `StreamMetricBatches`。首批数据携带主机身份、`tenant_code` 和 Agent 元数据。

```mermaid
sequenceDiagram
    autonumber
    actor A as Agent
    participant I as ingest-gateway
    participant P as PostgreSQL
    participant R as Redis
    participant M as master-api
    actor W as Web

    A->>+I: StreamMetricBatches（首包含身份）
    I->>P: 校验 tenant_code

    alt 租户存在
        rect rgb(236, 253, 245)
            I->>P: UPSERT host / agent / current status
            P-->>I: 事务提交
            I->>R: LPUSH 指标
            I->>R: PUBLISH host_upsert
        end
        par 实时通知
            R-->>M: host_upsert
            M-->>W: SSE host_upsert
        and 批次确认
            I-->>A: MetricBatchAck
        end
    else 租户不存在
        I-->>A: FailedPrecondition
        Note over A: 停止运行并退出
    end
    deactivate I
```

每个后续 batch 的处理顺序是：

1. PostgreSQL 事务更新 `agent_instances` 和 `host_status_current`。
2. 将 16 个窗口指标写入 Redis Lists。
3. 向 Redis Pub/Sub 发布 `host_upsert`。
4. 成功后向 Agent 返回对应 `batch_seq` 的 ACK。

PostgreSQL 与 Redis 不在同一事务内。若数据库提交后 Redis 失败，当前快照已经更新，但历史点或 SSE 通知可能缺失。

### Web 首屏与实时更新

Web 同时发起一次 REST 查询并建立 SSE：

```text
GET /master/api/v1/hosts?tenant=<tenantCode>
GET /master/api/v1/stream/hosts?tenant=<tenantCode>
```

SSE 连接建立后，`master-api` 从 PostgreSQL 读取当前主机列表，从 Redis 读取短期历史，发送 `sync`；后续把 Redis Pub/Sub 中同租户的 `host_upsert` 转发给浏览器。连接中断后浏览器会自动重连，新连接用新的 `sync` 收敛状态。

### 离线判定

`ingest-gateway` 每 5 秒扫描一次。主机超过 15 秒没有 Agent 上报时，会被标记为 `OFFLINE` 并发布新的 `host_upsert`。

### Probe

`probe-worker` 定时 GET 配置的目标地址，再 POST 到 `/ingest/api/v1/probes`。`ingest-gateway` 当前仅增加进程内计数，不写 `probe_results`，也不更新 `last_probe_at` 或可达性状态。

core 占位任务已移除。

## 服务内部边界

`master-api` 和 `ingest-gateway` 共用 `pkg/hostruntime/repository` 定义的仓储接口及 PostgreSQL/Redis 实现：

```mermaid
flowchart TB
    subgraph APIs[传输层]
        direction LR
        MasterHTTP["master-api<br/><small>HTTP / SSE</small>"]
        IngestHTTP["ingest-gateway<br/><small>HTTP / gRPC</small>"]
    end

    subgraph Services[业务层]
        direction LR
        MasterService["master service"]
        IngestService["ingest service"]
    end

    subgraph Repository[共享仓储层]
        Shared["pkg/hostruntime/repository<br/><small>接口与 PostgreSQL / Redis 实现</small>"]
    end

    subgraph Storage[数据层]
        direction LR
        PG[(PostgreSQL)]
        Redis[(Redis)]
    end

    MasterHTTP --> MasterService
    IngestHTTP --> IngestService
    MasterService --> Shared
    IngestService --> Shared
    Shared --> PG
    Shared --> Redis

    classDef transport fill:#eff6ff,stroke:#3b82f6,stroke-width:1.5px,color:#1e3a8a
    classDef service fill:#f5f3ff,stroke:#8b5cf6,stroke-width:1.5px,color:#4c1d95
    classDef repository fill:#fff7ed,stroke:#f97316,stroke-width:1.5px,color:#7c2d12
    classDef storage fill:#ecfdf5,stroke:#10b981,stroke-width:1.5px,color:#064e3b
    class MasterHTTP,IngestHTTP transport
    class MasterService,IngestService service
    class Shared repository
    class PG,Redis storage

    style APIs fill:#f8fafc,stroke:#bfdbfe
    style Services fill:#faf5ff,stroke:#ddd6fe
    style Repository fill:#fffaf5,stroke:#fed7aa
    style Storage fill:#f0fdf4,stroke:#a7f3d0
```


## 数据职责

### PostgreSQL

当前主链路使用：

- `tenants`：租户。
- `hosts`、`host_labels`：主机身份和标签。
- `agent_instances`：Agent 实例、版本、序号和最后活跃时间。
- `host_status_current`：Dashboard 的权威当前快照。
- `users`、`user_identities`、`user_sessions`：用户和已有会话解析。
- `maintenance_windows`、`alert_events`：部分运维操作。

新数据库结构见 `deployments/sql/init.sql`；数据库结构由使用者直接连接 PostgreSQL 管理。Compose 仅在新数据卷首次启动时执行初始化 SQL；已有库需自行更新。服务不校验迁移版本，也不在启动时创建账号；初始管理员只通过网页 setup 创建。操作说明见[数据库初始化](../02-guides/database-setup.md)。

### Redis

- 指标窗口：`gaoming:metrics:<host_uid>:<metric_key>`，List 中保存时间和值。
- 事件总线：`gaoming:master-api:host-events`，使用 Pub/Sub 传递 `host_upsert`/`host_delete`。

设置 `redis_namespace` 时，指标键和事件频道分别为 `<namespace>:metrics:*` 和 `<namespace>:host-events`。

当前 App 每项指标最多保留 60 个点，TTL 为 2 小时。Pub/Sub 不可重放，PostgreSQL 仍是当前状态的权威来源。

## 租户与认证边界

- 主机列表、详情、最新指标和 SSE 默认要求会话；租户只来自已认证用户，query 不能扩大范围。分享与演示使用独立公开 namespace 和主机/字段白名单，不借用私有会话。
- 受保护接口通过 `gaoming_session` Cookie 查找会话。
- 用户管理要求已登录且角色为 `admin`，并限制在会话租户内。
- 已接通本地密码登录和一次性初始管理员引导，已有环境通过页面或直接连接数据库维护账号；暂无 OAuth 回调。Cookie 默认 Secure/HttpOnly/SameSite=Lax，写操作校验 Origin 和会话派生 CSRF；认证响应 no-store。详见[登录手册](../02-guides/authentication.md)。
- 维护窗和告警 ACK 从可信 principal 校验当前角色、租户和资源，忽略请求操作者，写入和审计同事务；分享支持到期/撤销及 SSE 重验。详见[访问与分享](../02-guides/access-and-sharing.md)。

## 部署形态

`docker-compose.yml` 默认定义 PostgreSQL、Redis、一个 server 后端服务；Web 静态资源由 server 同一 Go 进程提供；容器 `agent` 通过 profile 启用：

- `make docker-up`：构建并启动一个 Server/Web 容器和 PostgreSQL、Redis，Web 与 API 共用 8080。
- `make docker-up DOCKER_PROFILES="--profile container-agent"`：额外启用 `container-agent` profile。
- 宿主机 Agent 使用根目录 `agent-config.yaml`。
- 后端默认读取 `config/server.yml`；Compose 将 `deployments/server-config.docker.yaml` 只读挂载到该路径。

`config/` 被 `.gitignore` 排除，本地或部署环境必须自行准备配置，示例见[本地开发与验证](../02-guides/local-development.md)。

## 已知缺口与演进优先级

1. **数据一致性**：PostgreSQL、Redis Lists 和 Pub/Sub 缺少统一事务或可靠事件机制。
2. **Probe 闭环**：探测结果未落库，未参与 `reachability_state` 和 `overall_state` 计算。
3. **后台计算**：状态、告警和调度尚未实现，原 core 占位 runner 已移除。
4. **可靠上报**：Agent 没有磁盘队列，`batch_seq` 不去重，进程重启后重新计数。
5. **认证与租户**：登录、私有读取、受限分享和运维租户绑定已接通；Agent/worker 上报认证、TLS/网络边界仍待 T04。
6. **扩展性**：每个 SSE 连接有独立 Redis 订阅，多实例 ingest 会重复执行离线扫描。

## 关键代码入口

| 主题 | 位置 |
| --- | --- |
| Agent 主循环 | `agent/internal/service/agent.go` |
| Agent 身份 | `agent/internal/identity/hostinfo.go` |
| Proto | `api/proto/monitor/v1/*.proto` |
| JSON 契约 | `pkg/contracts/api.go` |
| 主机快照与指标 | `pkg/state/state.go` |
| 共享仓储接口 | `pkg/hostruntime/repository/repository.go` |
| ingest 处理 | `server/internal/ingest/service/service.go` |
| master HTTP/SSE | `server/internal/master/transport/http` |
| Web 路由 | `web/src/app/router.tsx` |
| Web 实时数据 | `web/src/shared/features/hosts/useLiveHostsData.ts` |
| 数据库 Schema | `deployments/sql/init.sql` |
