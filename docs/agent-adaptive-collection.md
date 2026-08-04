# Agent 自适应采集改造跟进

## 目标

Agent 不再以固定的少量主机摘要指标作为采集能力边界。客户端应在每轮采集时探测当前操作系统、运行环境和权限实际支持的能力，成功采集的数值遥测全部放入指标批次上报；单个采集器失败不得阻断其他采集器和兼容指标上报。

当前阶段完成协议、Agent、最新快照存储、Master API 按需查询和 Dashboard 展示。Ingest Gateway 仍从兼容指标生成 `AgentDigest` 维护原有状态和趋势，同时把本轮全部动态数值点、标签维度和采集器状态保存为主机最新快照。

## 协议

`monitor.v1` 的服务、消息和枚举统一维护在 `api/proto/monitor/v1/monitor.proto`，生成代码统一为 `monitor.pb.go` 和 `monitor_grpc.pb.go`。

`PushMetricBatchRequest` 新增以下字段：

- `system_metrics`：强类型的 `SystemMetrics`，显式定义 runtime、host、CPU、memory、swap、load、filesystem、disk I/O、network 和 temperature 字段。
- `digest`：强类型的兼容摘要和速率指标，包括 CPU/内存/交换/根文件系统、磁盘 BPS/IOPS、负载和网络 BPS/PPS。
- `collectors`：每轮采集器状态。

每个 `CollectorStatus` 包含：

- `name`：稳定的采集器名称。
- `state`：`AVAILABLE`、`UNAVAILABLE` 或 `DEGRADED`。
- `metric_names`：本轮实际产生的指标名称，已排序去重。
- `duration_micros`：本轮采集器耗时。
- `timed_out`：是否因整轮预算耗尽而被舍弃或只返回部分结果。

`points` 继续使用已有的通用 `MetricPoint` 兼容旧服务端；客户端同时填充强类型字段，不再要求新服务端通过指标名字符串猜测数据语义。

## 当前进展

- [x] 保留原有 16 个主机摘要指标和 `runtime_uptime_seconds`。
- [x] 增加运行时自适应的 host、CPU、memory、swap、load、filesystem、disk I/O、network 和 sensors 采集器。
- [x] 在 protobuf 中显式定义当前采集的全部字段和单位。
- [x] 上报每 CPU、每块设备、每个物理文件系统、每张网卡和每个温度传感器的可用数值指标。
- [x] 上报网络接口 MTU、协议栈和 filter/conntrack 数值指标。
- [x] 将成功采集器写入 Agent `capabilities`，格式为 `collector:<name>`。
- [x] 单项失败时继续采集，并按结果标记 `DEGRADED` 或 `UNAVAILABLE`。
- [x] 过滤 NaN 和 Infinity，避免非法浮点值进入 protobuf 批次。
- [x] 为扩展指标统一补齐采集时间，并按指标名和完整标签集去重。
- [x] 9 类主机级采集器并行执行，整轮设置 95ms 内部截止时间，为组装返回预留 5ms。
- [x] 所有保留采集器使用 gopsutil `WithContext` API，截止时取消仍在执行的读取。
- [x] 兼容摘要、速率、通用 points 和强类型字段由同一份采样快照生成，不重复读取系统。
- [x] 集成测试强制要求 `Sample` 在 100ms 内返回；当前开发环境约 10ms。
- [x] 生成并提交更新后的 Go protobuf 代码。
- [x] Ingest Gateway 消费 `points` 和 `collectors`，将每台主机的最新完整采集快照写入 Redis。
- [x] Ingest Gateway 校验点数、指标名、有限浮点、标签数量与长度、collector 数量和批次字符串总量。
- [x] Master API 提供按租户和主机查询最新完整采集快照的接口。
- [x] Dashboard 按主机与负载、CPU、内存、Swap、文件系统、磁盘 I/O、网络和温度分类展示全部当前指标。
- [x] Dashboard 展示采集器可用、降级、不可用、超时状态和本轮耗时。

## 服务端存储与展示

当前动态遥测采用“完整最新快照 + 固定摘要历史窗口”的两层模型：

- `gaoming:metrics:<host_uid>:snapshot` 保存最新一次全部 `points` 和 `collectors`，使用现有 Redis 指标窗口 TTL，默认 2 小时。每轮成功批次原子覆盖该键，不为设备、挂载点、网卡和传感器标签创建高基数 Redis 键。
- 原有 16 项 `AgentDigest` 继续写入独立 Redis 列表，为 Dashboard 性能趋势提供历史数据。
- `GET /master/api/v1/hosts/:hostUID/metrics/latest` 先按请求租户校验主机归属，再返回单台主机最新快照；没有快照时返回 `snapshot: null`。
- Dashboard 只在主机被选中以及该主机收到 SSE `host_upsert` 后拉取单主机快照，避免把租户内所有高维快照广播给每个页面连接，并保留原有摘要和趋势视图。

快照是面向当前状态的短期保存，不是长期时序归档。主机停止上报超过 TTL 后，完整动态快照会过期；PostgreSQL 中的主机状态摘要不受影响。

Ingest 当前限制为每批最多 4096 个点、每点最多 16 个标签、最多 64 个采集器、单个指标名 128 字节、标签键 64 字节、标签值 256 字节、指标与标签等字符串总量 1 MiB。超限或包含 NaN、Infinity、控制字符的批次返回 gRPC `InvalidArgument`。

## 当前采集边界

本阶段上报可在 100ms 预算内稳定完成的主机级数值遥测。设备名、挂载点、文件系统、网卡名和传感器名只作为指标维度上报。

### 因耗时舍弃

- 全量逐进程 CPU、内存、I/O、文件描述符和上下文切换：进程数量决定开销，无法保证 100ms。
- 全量逐线程 CPU 时间、CPU 亲和性和资源限制：本机有界并发后单轮仍约 1.9 秒，串行约 7 秒。
- 伪文件系统和所有挂载点扫描：改用 `disk.Partitions(false)`，避免网络挂载、容器伪挂载或异常文件系统阻塞采集。

保留 `host.process_count` 和调度器进程汇总，提供轻量的进程整体状态。

### 因安全舍弃

进程命令行、进程名、环境变量、工作目录、打开文件路径、用户名、网络连接地址和端口、主机 ID、MAC 地址、磁盘序列号不采集。这些内容不是必要的主机数值遥测，且会明显扩大凭据、隐私和资产信息泄露面。

## TODO

- [x] Ingest Gateway 校验指标名、标签数量、标签长度、每批点数和主要字符串体积。
- [ ] 设计批次分片字段和重组/确认语义；在服务端支持前，超大动态批次可能超过 gRPC 默认消息大小而被拒绝。
- [x] 服务端消费 `collectors`，记录 Agent 当前可用、降级和不可用的采集能力。
- [ ] Redis 历史窗口从固定摘要白名单扩展为动态 `name + labels` 时序。
- [ ] PostgreSQL 增加长期指标模型或对接专用时序数据库，并制定保留、压缩和降采样策略。
- [x] Master API 增加最新动态指标的单主机查询协议，并复用 SSE 主机版本事件触发按需刷新。
- [ ] Master API 增加动态指标目录和历史查询协议。
- [x] Dashboard 根据最新快照和采集器状态按能力展示，不把未采集误显示为零。
- [ ] 为挂载点、设备、网卡和传感器指标制定服务端配额与告警。
- [ ] 增加 Linux、Windows、macOS 和 BSD 的 Agent 构建与采集契约测试。
- [ ] 若未来需要进程监控，设计独立的低频、分页、可配置采集协议，不得加入默认 100ms 主机采集路径。
- [ ] 在明确授权模型后，单独设计敏感资产和连接明细协议，不与默认遥测批次混用。

## 安全与资源风险

动态设备指标仍会增加批次体积和标签基数。当前入口配额限制单批资源消耗，最新快照模型避免按标签展开 Redis 键；后续实现动态历史时必须继续增加每主机序列数、租户总量、写入速率和保留时间配额，否则恶意或异常 Agent 仍可能造成 Redis、网络和内存资源耗尽。

挂载点可能包含业务或租户信息。传输链路必须使用 TLS，服务端查询必须继续执行租户隔离；新增维度不得直接拼接到 SQL、Redis 命令或日志格式中。

当前主机查询和最新快照查询沿用匿名只读模式，依赖请求中的 `tenant` 参数隔离，租户编码本身不构成身份认证。生产环境若包含敏感挂载点或设备命名，应在反向代理或 Master API 为这些读取接口强制会话认证与租户授权，不能只依赖租户编码保密。
