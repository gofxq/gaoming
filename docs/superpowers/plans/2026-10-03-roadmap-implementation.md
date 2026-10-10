# Gaoming M0～M5 完整实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `docs/roadmap.md` 中首次接入、可信看板、告警、HTTP 探测、历史与团队管理、移动/PWA 的用户流程实现为可验证的运行能力。

**Architecture:** 沿用 Go 单仓、React/Vite、PostgreSQL 权威业务状态及 Redis 短期查询/通知。新增跨服务共享的访问、业务仓储和出站访问控制包；core-worker 消费持久化任务，probe-worker 领取租约任务。各阶段的 API、存储、前端、迁移和故障验收一起交付。

**Tech Stack:** Go 1.25.8、Gin、gRPC/protobuf、GORM/PostgreSQL、Redis、React 18、TypeScript、Yarn。本地密码采用现有 `golang.org/x/crypto` 的 bcrypt；前端引入 Vitest/Testing Library 和 Playwright，安装时核对官方文档及兼容版本并锁定依赖。

**Spec:** [产品 Roadmap](../../roadmap.md)，[工程 TODO](../../TODO.md)。

## Global Constraints

- 当前文件是实施计划，所有未勾选任务都未实现、未验收。执行前完成计划审阅；执行期间根据证据更新状态。
- 覆盖 M0～M5 的确定功能；DNS/TCP/证书检查、多地点探测、浏览器推送、细粒度角色和进程/敏感资产采集在 roadmap 中属于另行评估，不能默认开启或宣称完成。
- 保留现有用户改动，包括 `scripts/dev-air.sh`、`web/DESIGN_SYSTEM.md`、已有采集性能资料和测试；开始执行时重新检查差异。当前是普通 checkout，未创建 worktree；执行时按已有环境和用户选择确定隔离方式。
- 沿用 Go 目录分层、gofmt、TypeScript 2 空格及现有现代/Pixel 设计规则；新页面不依赖 Go embed。
- 凭据不写入 URL、日志、版本库或公开错误；安装配置权限 0600，Windows 配置限制到服务身份与管理员。
- 未登录私有请求拒绝；会话/分享/Agent/worker 的租户范围来自已验证身份，不能信任请求 tenant 字段。
- 修改 SQL 同步 `deployments/sql/init.sql` 和有序存量迁移；禁止通过删卷升级。每个阶段提供配置升级、回滚与数据恢复说明。
- 每项实施遵循先写关键行为回归、确认失败、实现、确认通过；Go/前端/协议/联调检查按受影响范围执行。向真实外部渠道发送消息需另有授权，通知验收使用本地接收器。
- roadmap 的产品衡量先建立观察口径与人工验收记录，不默认加入收集用户行为的遥测。

## Review Focus

1. 身份或租户被省略、伪造、切换：读取/流式/写入拒绝越权，前端不能残留前一空间结果。T02/T03/T08/T19 覆盖。
2. ACK 丢失、旧批次补传、依赖故障与重启：旧数据不能覆盖新数据，投影可恢复，资源和退出时间有界。T06/T07/T09 覆盖。
3. 缺测、时间偏差、慢采集与过期：零值不代表缺测，迟到数据不延长在线状态，告警不误恢复。T06/T08/T12/T17 覆盖。
4. 出站 URL 的重定向、DNS 重绑定和私有地址：探测/webhook 不能突破允许范围；租约重试不能重复业务效果。T13/T14/T15 覆盖。
5. 撤销后旧连接、旧缓存、旧凭据与重放：分享、邀请、Agent、成员和 PWA 都失去访问，删除主机不自动复活。T03/T04/T18/T19/T21 覆盖。

## 范围和实现决策

| 能力 | 本次实现决策 |
| --- | --- |
| 登录 | 空库由一次性 setup token 引导首位管理员；现有库由本机管理命令建立账号。用户名在租户内唯一，密码 12～72 字节，bcrypt cost 12；登录失败统一提示。不会自动生成已知默认密码 |
| 会话 | 保留 32 字节随机 token、SHA-256 查库、默认 168 小时过期。生产 Cookie Secure/HttpOnly/SameSite=Lax；会话写操作检查允许 Origin 与 CSRF token；登录/初始化也检查 Origin |
| 角色 | admin 可管理接入/成员/配置，member 可看数据、确认告警；两者均受租户限制；最后一位有效管理员不能被降级或禁用 |
| 接入 | 管理员创建 15 分钟、一次兑换的 enrollment token；Agent 兑换为绑定 tenant+hostUID 的随机凭据，支持撤销/轮换；流中持续核验撤销状态 |
| 分享/演示 | 默认私有；演示只对配置的演示租户和允许字段开放。分享由管理员限定主机集合、字段和最长 7 天 TTL，可撤销；不授予写权限 |
| 告警 | 离线、CPU、内存、磁盘和 HTTP 服务失败模板；使用结构化规则，不能执行任意表达式。确认与恢复分开；活动告警 fingerprint 唯一 |
| 通知 | 首版实现出站 webhook、测试发送、投递记录、触发/恢复通知与指数退避；避免在业务事务内网络发送；消息只含允许摘要及需授权的站内详情地址 |
| 探测 | HTTP/HTTPS，默认只访问公开地址及 80/443；私网需管理员在 worker 配置中明确授权 CIDR/端口，租户表单不能扩大该范围 |
| 历史 | PG 分区时序，固定摘要默认 7 天；基础精度默认 60 秒，短期 Redis 保留可配置；动态历史默认关闭，管理员按指标/维度授权并限制序列数；历史精度在 UI 明示 |
| 用户组织 | 用户仍属于一个租户；邀请、成员启停、分组/标签、服务关联分组、保存视图与审计。跨租户多成员身份本次不引入 |
| PWA | 同一 React 应用响应式路由；只缓存公共静态应用壳，默认不缓存私有 API/通知/图表数据；离线页面说明数据不可用，现存内存快照标记过期 |
| 升级与规模 | 安装升级支持版本、校验和、来源校验及回退；默认不后台远程执行任意命令。记录容量基线，超大批次在实测需要时启用受配额分片 |

上述默认值是本计划选择，实施时成为可配置项；任何更改记录原因和验收影响。

## 文件边界与公共接口

- `pkg/access/{types,tokens,store}.go`：跨服务身份、凭据与分享范围；`pkg/access/postgres/store.go` 保存/撤销哈希凭据。`Principal{Kind,TenantID,TenantCode,UserID,Role,HostUID,WorkerID,AllowedHosts}`；`Authenticate(ctx context.Context, token string, kind Kind, now time.Time) (Principal,error)`。
- `pkg/control/{types,repository}.go` 与 `pkg/control/postgres/`：分拆 alerts/maintenance/probes/events/groups/views/audit 的业务存储。服务内部依赖窄接口，不能把所有职责塞进现有 hostruntime store。
- `pkg/outbound/{policy,client}.go`：探测和 webhook 共用 DNS/地址/重定向策略；`NewClient(policy Policy,resolver Resolver) *http.Client`；Policy 授权来源是管理员配置，测试 resolver 可注入。
- `pkg/metrics/{types,repository}.go` 与 `pkg/metrics/postgres/`：长期点、目录和有界查询；`Query{TenantID,HostUID,Name,Labels,From,To,Step}`，`Series{Points,ResolutionSec,AvailableFrom,AvailableTo,Gaps}`。
- `pkg/migrations/`、`deployments/sql/migrations/`：锁定版本、事务迁移与可检查执行结果；已有升级 SQL 对接版本记录。
- `services/master-api/internal/{auth,service,transport/http}`：会话/成员认证、授权 API 和领域服务；按 auth/install/alerts/probes/history/admin 分文件。
- `services/core-worker/internal/service/`：projection/status/alerts/notifications/scheduler/retention 分文件与接口；从 PG 领取持久化任务。
- `services/probe-worker/internal/service/`：lease/executor/report；Agent `internal/service/` 分拆 transport/queue，保留采集器职责。
- `web/src/shared/features/{auth,hosts,install,alerts,probes,history,team,views}`：typed API、状态与领域工具；`web/src/pc/pages/` 页面；跨端共享业务，视觉使用现有主题。
- `web/public/` 或现有 Vite publicDir `web/favicon/`：manifest、Service Worker 与安装图标；选择一个入口并同步 Vite 配置，不能双重注册。

所有 master API 路径以下省略 `/master/api/v1`；非公开请求依赖会话。API 错误统一 `{code,message,request_id}`，列表统一 `{items,next_cursor}`，无权资源不泄露其存在性。写操作携带 CSRF token；GET 私有数据设置 no-store。

## 执行顺序

T01 → T02 → T03/T04 → T05 → T06/T07/T08/T09/T10 → T11/T12/T13 → T14/T15/T16 → T17/T18/T19 → T20/T21/T22 → T23。

斜线表示无硬依赖，不要求并行代理。默认由当前执行者按顺序完成；每个阶段有完整回归和文档更新，不能仅以表/按钮/计数增长验收。

### T01：数据库迁移与质量门禁基础（P1-06/P1-07）

**Files:** 新建 `pkg/migrations/runner.go`、`services/master-api/cmd/admin/main.go`、`deployments/sql/migrations/001-baseline.sql`、`.github/workflows/check.yml`、`scripts/smoke-roadmap.sh`；修改 `Makefile`、`web/package.json`。

**Interfaces:** `migrations.Run(ctx context.Context,db *sql.DB) error`；新增 `make migrate`、`make web-test`、`make web-e2e`、`make smoke-roadmap`。迁移在独立管理命令运行，服务检查版本，不能并行隐式修改 schema。

- [x] 回归 `TestMigrationEmptyAndExistingDatabaseMatch`：新库与现有初始化库升级后结构一致；失败事务不记录成功版本；重复执行不重复数据。为现有升级 SQL 验证兼容路径。
- [x] 实现迁移锁/版本表/管理命令，增加全仓 Go/协议/Web 门禁与前端行为测试工具，PR 只授予读仓库权限。
- [x] 执行 `make test`、`make build`、`make proto-check`、`make web-typecheck`、`make web-build`；PG 迁移集成检查使用独立测试库，保留结果。记录开始执行前的基线与已有失败原因。

2026-10-10：T01 的命令、5 项真实 PG 回归、3 项 Web 单元回归、1 项浏览器回归及独立 Docker/Agent smoke 均通过。详细证据与安全检查见 [TODO 实施记录](../../TODO.md)。结构比较目前覆盖列名/类型/可空性；索引/约束/默认值及完整备份恢复继续在 P1-07/T10 补齐。迁移占用 001/002，后续功能迁移编号顺延。改动按用户要求保留在当前目录供审阅。

### T02：本地登录、管理员引导与会话（M0，P0-02/P0-05）

**Files:** 修改 `internal/auth/{types,store}.go`、`internal/service/auth.go`、`internal/transport/http/server.go`、master config/app、`web/src/shared/features/auth/AuthProvider.tsx`、`LoginPage.tsx`；新建 `internal/auth/password.go`、`internal/transport/http/auth.go`、`web/src/pc/pages/SetupPage.tsx` 与 `002-auth.sql`。

**Interfaces:** `POST /auth/setup {setup_token,tenant_code,username,display_name,password}`、`POST /auth/login {tenant_code,username,password}`、`GET /auth/csrf`、现有 session/logout；`Service.Login(ctx context.Context,req LoginRequest) (auth.Session,error)`。CSRF token 不含原始会话 token。

- [x] 回归 `TestSetupTokenSingleUse`、`TestLoginDisabledAndExpiredSession`、`TestLoginOriginAndPasswordBoundaries`：并发初始化只有一位初始管理员；错误登录不泄露账号；session/cookie/Origin/CSRF 按决策生效。
- [x] 新增租户唯一密码身份及管理员命令；接通登录/初始化页面、字段错误、忙碌与过期反馈，站内 return_to 校验拒绝 `//`、反斜线与未授权空间。
- [x] 执行 `go test ./services/master-api/...`、前端登录行为回归、`make web-build`；验收无需手工写会话，密码/原始 token 不出现在 JSON 或日志。

2026-10-10：T02 完成真实 PG/HTTP、race、15 项 Web 单元、4 项 Chromium 场景及更新后的 Docker/Agent smoke。独立审阅发现的跨身份成员缓存问题已通过 RED→GREEN 修复。认证迁移为 003，后续从 004 编号。详细记录和截图见 TODO；其他阶段未完成。

### T03：读取授权、运维权限与受限分享（M0，P0-01/P0-03）

**Files:** 新建 `pkg/access/`、`internal/transport/http/access.go`、`internal/service/shares.go`、`web/src/pc/pages/SharePage.tsx`、`003-access.sql`；修改 hosts/SSE/ops handlers、公共 OperationsStore 接口及 memory/PG 实现。

**Interfaces:** `CreateMaintenance(ctx,principal,req)` 与 `AckAlert(ctx,principal,alertID)` 在服务层验权；`POST/GET /shares`、`DELETE /shares/:id`；公开 `POST /public/shares/exchange` 将 token 兑换为窄范围 HttpOnly Cookie，链接 token 放 fragment 并在兑换后移除。demo 按配置指定租户与字段。

- [ ] 回归 `TestPrivateReadsRejectMissingOrForgedTenant`、`TestOpsUseSessionTenantAndActor`、`TestShareScopeExpiryAndRevocation`：遗漏 tenant 不返回全量；任意 host/alert/scope 引用不越权；撤销后 SSE 关闭，刷新/历史同样受范围约束。
- [ ] 将租户/操作者从 principal 传入仓储，返回真实资源不存在状态；界面展示无权限/分享过期，分享管理说明允许字段及到期。
- [ ] 执行授权、HTTP 和 SSE 回归及 Web 构建，核对不能靠隐藏按钮或不可猜 tenant 达成隔离。

### T04：Agent/worker 接入凭据、资源边界和 TLS（M0，P0-04/P0-06/P0-07）

**Files:** 新建 `internal/service/enrollment.go`、ingest gRPC/HTTP access middleware、`pkg/access/postgres/`、`004-enrollment.sql`；修改 proto/生成代码、Agent config/app/service、所有安装脚本、worker config、服务装配和 Compose。

**Interfaces:** `POST/GET /install/enrollments`、`DELETE /install/enrollments/:id`；gRPC `ExchangeEnrollment` 返回长期 Agent 凭据，tenant/host 绑定不可改；metadata `authorization: Bearer ...`。worker 凭据由管理命令创建且绑定租户/region；HTTP 上报同样鉴权。

- [ ] 回归 `TestEnrollmentConcurrentExchange`、`TestAgentSpoofAndRevokedStream`、`TestRequestLimitsBeforeStorage`、`TestTLSExplicitMode`：一次性并发兑换；旧流撤销；HostUID/AgentID/tenant 不匹配拒绝；非法 metadata、超配额与证书错误拒绝。
- [ ] 实现认证、身份字段/消息大小/写入与连接配额、TLS 显式配置；安装脚本支持安全凭据文件/stdin，不把秘密作为持久命令行参数；配置替换原子写入并保留 ACL。
- [ ] 回归注册/单次/双向流/事件/探测入口；运行 proto 检查、Agent 跨平台构建、Compose 和本地 smoke。提供现有匿名 Agent 的受控迁移与重新授权步骤。

### T05：首次接入向导与失败帮助（M0 产品新增）

**Files:** 新建 `web/src/pc/pages/InstallPage.tsx`、`shared/features/install/{api,model}.ts`；修改 router/PcShell/Dashboard 空状态、主机列表 API 返回接入关联；更新 README/开发指南。

**Interfaces:** 路由 `/:tenantCode/install`；`GET /install/enrollments/:id` 返回 pending/connected/expired/revoked 及授权主机，不虚构设备安装进度。

- [ ] 行为回归 `installJourney`：三平台指引与复制、过期/撤销、首次上报跳转、无权限区别于无主机，页面不能在兑换后继续显示可重复使用凭据。
- [ ] 完成安装/更新/卸载说明、网络/权限/凭据排查、凭据生命周期；失败保留非敏感输入，原始秘密只短暂显示一次。
- [ ] 执行 Web 行为回归/构建；用独立测试租户完成首次用户接入场景并保存截图，记录各平台实际验收范围。

### T06：采样质量、幂等、时间和有界补传（M1，P1-01/P1-02/P1-05/P1-11）

**Files:** Agent 新建 `queue.go`、`transport.go`；修改 metric validation/contracts/proto/hostruntime PG；新建 `005-ingest-batches.sql`。

**Interfaces:** 批次身份 `(tenant_id,host_uid,agent_id,batch_seq)` 唯一；显式 collected_at/received_at/alive_at、指标 quality；ACK 包含 accepted/deduplicated/rejected 与确认配置版本。配置 ACK timeout 默认 5 秒、最多 120 个待确认批次/16MiB，溢出丢最旧并记录缺口；默认内存缓冲，文档说明进程重启后的丢失范围。

- [ ] 回归 `TestDuplicateAndOutOfOrderBatch`、`TestLateBatchDoesNotReviveHost`、`TestAckTimeoutQueueBoundAndShutdown`、`TestMissingMetricIsNotZero`：旧批次不覆盖新状态，重新发送无重复点；缺测和首次速率不伪装有效零；identity 主从矛盾拒绝。
- [ ] 实现持久批次幂等、质量和时钟偏差检查、ACK 期限/退避抖动/取消；摘要与强类型来自同一采样，兼容规则为一致性校验后使用强类型，否则兼容 points。
- [ ] 执行 Agent/ingest/仓储定向回归；模拟 ACK 丢失、离线 10 分钟与恢复，确认边界与丢弃可解释；保持已有采集预算。

### T07：事务 outbox 与 Redis 投影恢复（M1，P1-03）

**Files:** 新建 `pkg/control/postgres/outbox.go`、core `projection.go`、`006-outbox.sql`；修改 ingest service、PG batch 事务和 core app/config。

**Interfaces:** `ClaimOutbox(ctx context.Context,owner string,limit int,lease time.Duration) ([]OutboxItem,error)`、`CompleteOutbox(ctx,id,leaseToken) error`；PG 一事务保存批次/当前快照/outbox，ACK 表示 PG 持久提交。历史/最新快照/通知由可幂等投影处理。

- [ ] 回归 `TestProjectionFailureAfterEachStep`：事务失败不 ACK；Redis/通知失败重试后收敛；乱序 outbox 用版本保护最新快照，摘要点按批次/指标去重；重复投递不重复告警。
- [ ] 实现租约、指数重试、积压上限与告警、保留清理；core 不将 Pub/Sub 当任务队列；恢复时从 PG 重建约定短期投影。
- [ ] 以独立 PG/Redis 环境做故障注入与恢复 smoke，记录快照、历史、outbox 各步结果及延迟边界。

### T08：可信看板与查询上下文（M1 产品新增）

**Files:** 新建 `shared/features/hosts/{filters,quality,metricCatalog}.ts`、`HostDetailPage.tsx`；修改 Dashboard/PcShell/router/model/实时 Hook/CSS。

**Interfaces:** `/:tenantCode/hosts/:hostUID`，URL 参数 `q,status,sort,view,window`；过滤和排序函数是纯函数；quality 状态 valid/missing/unsupported/failed/stale；历史响应包含实际覆盖及精度。

- [ ] 回归 `hostDiscoveryAndQuality`：摘要点击正确筛选，异常优先排序、卡片/列表一致，前进后退/刷新恢复条件，迟到旧租户请求不写状态，零/缺测/过期分别呈现。
- [ ] 加入中文名称/单位/解释、按需设备维度、真实时间范围/缺口、适用样本平均值、独立加载/错误/重试反馈；保持两种主题与键盘焦点。
- [ ] 运行纯函数与交互回归、`make web-typecheck`/`make web-build`；保存桌面与手机宽度截图，验证无整页横向滚动。

### T09：SSE 一致性、撤销与慢连接（M1，P1-04/P1-08）

**Files:** 修改 stream.go、hostruntime event bus/contracts、web Hook；新建 master `event_hub.go`、SSE 运行指标。

**Interfaces:** 每个 master 共享上游订阅，授权的下游有界队列；先订阅再读取快照、依据 host version 合并；删除事件包含 tenant 和版本。上游中断触发重新同步，无回放时不谎称支持 Last-Event-ID 恢复。

- [ ] 回归 `TestUpdateDuringInitialSync`、`TestDeleteTenantAndRevocation`、`TestSlowClientBound`：同步期间写入不丢，删除不跨租户，禁用/分享撤销关闭流，积压超限断开后客户端可重新同步。
- [ ] 实现连接/写截止、周期权限重验和业务/连接新鲜度区分；HTTP 请求读取边界与 gRPC 关闭兜底兼容长流。
- [ ] 跑 SSE 回归及连接中断 smoke，验证一个慢客户端不阻塞其他连接，进程规定期限内退出。

### T10：运行指标、配置、部署与公共仓储收敛（P1-06～P1-10）

**Files:** 新建 `deployments/config-examples/`、生产 Web Dockerfile/反代配置、备份恢复脚本、`docs/02-guides/operations.md`；修改 Make/CI/部署脚本；迁移旧 master 仓储行为后清理无引用实现。

**Interfaces:** `/healthz` 存活、`/readyz` 依赖就绪；受保护 `/metrics` 输出 ACK/outbox/连接/存储/拒绝计数和延迟；部署参数不绑定作者目录。

- [ ] 回归 `TestReadinessAndBoundedShutdown`、配置校验及迁移/备份恢复检查；确认去掉旧实现后有效行为仍由公共仓储覆盖。
- [ ] 示例仅含占位秘密；PG/Redis 默认回环暴露，生产静态 Web 与 SSE 反代；CI 包含服务/协议/前端路径并限制发布写权限。
- [ ] 干净检出验证 Make、Compose、安装与部署命令；独立卷验备份恢复/存量升级；记录真实未覆盖的平台，不用交叉编译代替运行验证。

### T11：规则管理、状态融合与维护窗（M2，P2-01/P2-03）

**Files:** 新建 `pkg/control/{types,repository}.go`、PG `alerts.go`/`maintenance.go`、core `status.go`/`alerts.go`、master `rules.go`/`maintenance.go`、前端 RulesPage/MaintenancePage、`007-alerts.sql`。

**Interfaces:** `GET/POST/PATCH /alert-rules`、`GET/POST/PATCH /ops/maintenance`；结构化规则 `{kind,metric,threshold,recovery_threshold,for_seconds,scope_type,scope_ref,severity,enabled}`，kind metric/offline/service；maintenance 采用 UTC、左闭右开区间。

- [ ] 回归 `TestRuleScopeDurationAndMissingValues`、`TestMaintenanceOverlapAndExpiry`、`TestAgentAndProbeStatesRemainDistinct`：维度缺测不触发正常恢复，跨租户 scope 拒绝，维护重叠/结束按规则处理。
- [ ] 为离线/CPU/内存/磁盘实现模板、状态原因和唯一写入职责；计算和巡检用行锁/版本防止并发覆盖。
- [ ] 验规则修改/停用、阈值与持续时间、不同上报间隔、维护窗到期；页面能解释抑制范围与条件。

### T12：告警生命周期和处置页面（M2，P2-02）

**Files:** 新建 PG `alert_events.go`、master handlers/service、AlertsPage/AlertDetailPage、shared alerts api；迁移补 fingerprint 活动唯一约束与事件时间线。

**Interfaces:** `GET /ops/alerts`、`GET /ops/alerts/:id`、现有 `POST /ops/alerts/:id/ack`；页面 `/:tenantCode/alerts/:id`。resolved 为生命周期，ack 作为独立操作者/时间，不以确认代替恢复。

- [ ] 回归 `TestAlertRepeatedEvaluateAndRestart`、`TestAckDoesNotResolve`、`TestMissingDataDoesNotResolve`：持续异常一个活动告警；恢复只记录一次，重启不丢持续时间状态。
- [ ] 持久化状态转换、去重、原因/阈值/实测值、分页与筛选；详情跳转主机与发生时段，记录真实处理者。
- [ ] 演练触发→列表→详情→确认→恢复，验证无权限/不存在/重复确认的明确响应，保存截图。

### T13：webhook 通知、投递重试与出站策略（M2，P2-04）

**Files:** 新建 `pkg/outbound/`、control PG `notifications.go`、core `notifications.go`、master 通知 handlers、NotificationSettingsPage、`008-notifications.sql`。

**Interfaces:** `GET/POST/PATCH /notification-channels`、`POST /notification-channels/:id/test`、`GET /notification-deliveries`；delivery 按 alert+transition+channel 唯一，含 idempotency_key；出站默认超时 5 秒，重试 5 次，记录最终失败。

- [ ] 回归 `TestWebhookPrivateAddressRedirectAndRebind`、`TestNotificationRetryAndDedup`：解析后 IP 验证，重定向再次校验，失败重试不创建新任务；接收端可依 idempotency key 去重，页面不得承诺网络层 exactly-once。
- [ ] 事务产生投递任务；后台安全发送测试/触发/恢复，UI 提供结果与重试；敏感 URL/headers 不回显或记录。
- [ ] 用本地测试接收器与注入 DNS 演练失败/恢复/超时；不能向未经授权真实渠道发送消息。

### T14：探测目标与结果持久化（M3，P2-05）

**Files:** 新建 control PG `probes.go`、master probes service/handler、ingest probes 验证、ProbesPage/ProbeDetailPage、`009-probes.sql`。

**Interfaces:** `GET/POST/PATCH /probes`、`GET /probes/:id/results`、`POST /probes/:id/test`；target 可关联授权 host/group；worker 报告包含 lease token、execution_id、target/job 与时间，事务去重。

- [ ] 回归 `TestProbeResultTenantLeaseAndDuplicate`、`TestProbePaginationAndRetention`：错误 worker/目标/租约拒绝，不计入有效统计，重复不增加历史。
- [ ] 保存延迟/状态/错误原因，目标创建关联 policy/job；本地测试探测同样走实际任务队列和出站策略。
- [ ] 验目标创建/编辑/启停、最近结果/短期趋势；无结果或任务未执行不是服务正常。

### T15：任务领取、SSRF 防护与 worker 恢复（M3，P2-06）

**Files:** core `scheduler.go`、probe `lease.go`/`executor.go`/`report.go`、master worker handlers、worker config/app。

**Interfaces:** `POST /workers/probe-jobs/claim`、`POST /workers/probe-jobs/:id/renew`，凭据绑定 tenant/region；租约大于单次配置超时且续约可取消，PG `FOR UPDATE SKIP LOCKED` 领取。

- [ ] 回归 `TestConcurrentClaimAndWorkerLoss`、`TestSSRFIPv4IPv6RedirectAndDNS`、`TestExpiredLeaseResultRejected`：失联重领；探测不出授权网段/端口；重试不会采纳旧租约晚结果。
- [ ] 去掉固定 JobID/TargetID，使用受控 DialContext 校验实际连接地址；HTTP/HTTPS 期望响应、体积/超时/并发有界；私网授权只能来自管理员配置。
- [ ] 本地服务模拟超时/错误响应/恢复，验领取→执行→存储→查询；再接通状态原因与 M2 服务规则。

### T16：事件生产与统一时间线（M3，P2-07）

**Files:** Agent 新建 `events.go`，control PG `events.go`、master events API、shared timeline/主机与探测详情，`010-events.sql`。

**Interfaces:** `GET /events?host_uid=&target_id=&from=&to=&cursor=`；事件含 tenant、source、对象、occurred_at、received_at、去重键与 allowlisted attrs。

- [ ] 回归 `TestEventReplayAndSensitiveFields`：初始 Agent 启动/恢复、采集器能力变化、服务状态变化可检索；重复事件不重复，任意 attrs 不允许上传秘密/连接明细。
- [ ] 事件本地有界缓冲，鉴权/持久提交与 ACK 接通；时间线显示来源、时间偏差和授权关联对象。
- [ ] 从实际 Agent/探测产生事件验证查询和分页，不能以计数增长替代落库验收。

### T17：有界历史、指标目录与时间对照（M4，P2-08/P2-09）

**Files:** 新建 `pkg/metrics/`、PG repository、core retention、master history handlers、HistoryPage/metric catalog、`011-metric-history.sql`。

**Interfaces:** `GET /hosts/:hostUID/metrics/catalog`、`GET /hosts/:hostUID/metrics/history?name=&from=&to=&step=&labels=`；目录包含 name/单位/类型/允许维度，结果为 Series。默认单次最多 10000 点、范围不超保留时间，聚合仅白名单。

- [ ] 回归 `TestHistoryBoundsGapsAndAggregation`、`TestDynamicSeriesQuotaAndRetention`：时间范围/标签/租户校验，缺口不补零，降采样算法和单位一致，超序列额度拒绝且可观测。
- [ ] 分区写入/去重、基础 7 天保留与清理、按配置开放动态历史；UI 标注实际范围和精度，以同一 UTC 时间轴关联资源/告警/探测/事件。
- [ ] 写入、查询、分区清理/恢复集成验证；从告警进入实际发生时段并完成前后对比，不能开放无历史依据的选项。

### T18：主机、分组、标签和配置生命周期（M4，P2-10）

**Files:** control PG `groups.go`/`hosts.go`、master admin handlers、Agent desired config/proto、HostsManagePage/GroupsPage、`012-host-management.sql`。

**Interfaces:** `PATCH/DELETE /hosts/:uid`、`POST /hosts/:uid/revoke-agent`、groups/labels CRUD；配置 `desired_version/applied_version/error`，ACK 附配置及确认；服务 target 增加 group 关联。

- [ ] 回归 `TestDisableDeleteDoesNotRebootstrap`、`TestGroupCrossTenantReference`、`TestConfigApplyRollbackAndAck`：停用/删除/撤销差异明确，已撤销不能自动建档，配置错误不覆盖最后有效配置。
- [ ] 主机删除先撤销凭据并删除当前投影，历史按保留策略关联 tombstone；分组层次防环；UI 明示告警/探测/历史影响。
- [ ] 验配置间隔变化与离线阈值、版本状态、删除后 SSE/界面/历史收敛及审计记录。

### T19：成员邀请、保存视图与审计（M4 产品新增）

**Files:** control PG `invites.go`/`views.go`/`audit.go`、master team APIs、UsersPage/ViewsPage/AuditPage、`013-team.sql`。

**Interfaces:** `POST/GET/DELETE /admin/invites`、`POST /auth/invites/accept`；views CRUD 保存 typed 筛选/排序/范围；`GET /admin/audit` 分页；邀请默认 24 小时一次性，只创建绑定邀请租户的 member。

- [ ] 回归 `TestInviteExpiryReplayAndTenantBinding`、`TestLastAdminAndDisabledMember`、`TestSavedViewShareScope`：并发接受一次、最后管理员保护、成员撤销会话/连接失效、共享过滤条件不能扩大允许主机范围。
- [ ] 成员加入/启停、角色说明、个人/团队视图及审计；权限修改与核心资源写入同事务记录 actor，不记录秘密 payload。
- [ ] 用两个测试用户和两个租户完成邀请/筛选/分享/撤销/审核流程，保存角色差异截图。

### T20：移动布局、无障碍和图表交互（M5）

**Files:** Dashboard/详情/告警/探测/管理页面 CSS、共享图表/状态控件、appearance、router；复用业务状态，不复制 PC/H5 查询逻辑。

**Interfaces:** 触屏与键盘共用选择状态；图表提供可聚焦点和文字摘要；移动核心操作为列表/筛选/详情/确认/返回。

- [ ] 回归 `mobileCoreJourneyAndKeyboard`：320/375/768px 核心页面无整页横向滚动，关键动作不依赖 hover，状态有文字/图标，深浅色与现代/Pixel 一致。
- [ ] 异常/告警优先移动首页，按需展开明细、触屏读数与时间切换、稳定焦点和忙碌反馈。
- [ ] Web 构建及浏览器场景检查，每种外观保存桌面/手机截图；真实设备支持范围如实记录。

### T21：PWA 安装、更新与隐私缓存（M5，P3-01）

**Files:** publicDir manifest/service-worker/icons、main、PwaStatus 控件、Vite 构建配置与静态部署。

**Interfaces:** `/:tenantCode/pwa` 展示安装与支持条件；Service Worker 仅缓存带构建版本的公开静态资源，`/master/*` network-only；更新由用户确认并保留导航上下文。

- [ ] 回归 `pwaInstallUpdateLogoutAndOffline`：无缓存私有请求/响应，退出和身份变化不残留内存敏感数据，离线状态不声称实时，旧版本缓存可清理。
- [ ] 完成 manifest、安装说明、版本提示和离线应用壳，HTTPS 环境验收；不把 PWA 安装等同于浏览器推送。
- [ ] 构建静态站与浏览器模拟安装/升级/离线再上线，检查 CacheStorage 与授权连接，记录目标浏览器差异。

### T22：规模基线、超大批次与受控升级（M5，P3-02～P3-04）

**Files:** `scripts/load-roadmap.sh`、Agent version/upgrade、安装/发布脚本、proto 批次分片、ingest assembler、`docs/03-reference/capacity.md`。

**Interfaces:** 容量参数 hosts/interval/batch_bytes/SSE_clients；记录 p50/p95/p99、错误率、CPU/内存/PG/Redis。分片如启用使用 batch_id/fragment_index/fragment_count/total_bytes/hash，整批提交 ACK；每身份最多 2 个组装批次、TTL 30 秒、总重组不超 16MiB，单片不超 1MiB。

- [ ] 回归 `TestFragmentMissingDuplicateAndQuota`、`TestUpgradeChecksumSourceAndRollback`：乱序/缺片不部分覆盖快照；损坏或未授权来源拒绝安装；失败恢复原二进制与配置。
- [ ] 先压测测定需要，再优化共享订阅/分页与批量读取；超过单批上限有明确错误和受控分片策略；升级使用管理员显式安装流程、校验清单和来源验证，提供版本/影响/回退文档。
- [ ] 保存压测参数和结果，支持规模只按实测声明；跨平台升级在各自平台验收，校验和不能被文档描述成发布者签名；不自动增加敏感采集。

### T23：全阶段验收、文档与安全审阅

**Files:** `docs/01-core/architecture.md`、`docs/03-reference/api-and-data.md`、开发/运维指南、README、TODO/roadmap 状态与 `docs/02-guides/roadmap-acceptance.md`。

**Interfaces:** 验收记录逐条对应 roadmap 功能、用户任务、工程 TODO、实现文件和实际命令；可选评估项独立标注。

- [ ] 检查每阶段完整用户流程、跨租户拒绝、过期撤销、故障恢复、部署/迁移/备份与两种主题截图；人工产品观察记录首次接入、定位异常、理解数据、通知与复盘任务的结果。
- [ ] 执行 `make test`、`make build`、`make proto-check`、`make web-typecheck`、`make web-test`、`make web-build`、`make web-e2e`；受影响联调执行 `make compose-config`、`make docker-up`、`make smoke`、`make smoke-agent`、`make smoke-roadmap`、`make docker-down`，独立测试数据/配置不影响用户现有卷。
- [ ] 审阅凭据、授权、SSRF、限流、缓存、日志、SQL 迁移、安装来源和发布权限；修复证据中的失败后才标完成，给出 Git message 和仍需真实环境验证的限制。

## 计划覆盖检查

| Roadmap 要求 | 实施任务 |
| --- | --- |
| M0 登录、引导、访问/分享/失败、三平台接入与撤销 | T02～T05 |
| M1 摘要筛选、排序、卡片/列表、路由、中文指标、质量、新鲜度、窗口与错误反馈 | T06～T09 |
| M2 模板、配置、列表/详情、确认/恢复、通知与维护窗 | T11～T13 |
| M3 目标管理、期望响应、手动验证、结果/延迟、主机/分组、状态/通知、事件 | T14～T16，T18 分组关联 |
| M4 历史范围、动态维度、统一时间轴、分组/标签/视图、配置、删除、邀请/权限/审计/分享 | T17～T19，T03 分享基础 |
| M5 移动核心操作、触屏图表、安装/更新/离线、主题/键盘、容量、升级说明 | T20～T22 |
| 跨阶段操作反馈、权限提示、上下文、可信数据、两种主题、可访问性 | T02～T05、T08/T09、所有页面任务、T20/T23 |
| 产品观察与度量，工程配额/恢复/CI/迁移/部署 | T01/T06/T07/T10/T17/T22/T23 |
| 另行评估能力：新增探测协议/多地点/浏览器推送/细粒度权限/敏感采集 | 保留 roadmap 的需求验证条件；T22/T23 记录决策，不默认开发或开启 |

## 当前状态与执行交接

2026-10-10 已开始实施，T01/T02 已完成本地验收；其他未勾选任务尚未完成。实际进展、命令与安全检查以 [TODO](../../TODO.md) 为准。没有部署到生产或发送外部通知。

用户已要求按 roadmap/TODO 完整实施，并选择在当前目录保留改动供审阅；由主执行者顺序实施，逐步骤记录证据。下一步从 T03 开始，已有工作区内容须完整保留。

安全检查：本次仅新增计划文档，无新运行风险。接入鉴权、公开访问和租户绑定仍有既有风险；计划将其作为 M0 交付条件，并对出站探测/webhook、缓存、恢复和升级设置相应回归。
