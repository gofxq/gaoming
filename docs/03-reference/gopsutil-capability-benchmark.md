# gopsutil 采集能力与性能报告

## 结论

本仓库固定使用 `github.com/shirou/gopsutil/v4 v4.24.12`。本报告只对该版本和本次 Linux 宿主机实测负责；截至 2026-08-05，gopsutil 已发布 `v4.26.6`，升级后需要重新生成能力清单并复测。

“gopsutil 一共有多少数据”没有一个跨机器固定的点数：CPU、磁盘、挂载点、网卡、协议、连接、传感器、容器、进程和线程都会动态展开。应使用三个口径：

- 当前 Linux 构建的 9 个功能包直接声明 277 个导出结果字段，其中 222 个是直接数值字段。这是静态能力下限，不含嵌入结构和 `map[string]int64` 等动态字段。
- Agent 当前宿主机一次完整批次实际包含 192 个唯一指标名、692 个指标点。
- Agent 9 个默认 gopsutil 采集器本身产生 180 个唯一指标名、679 个指标点；完整批次另含兼容摘要和 runtime 指标，并按名称和标签去重。

当前默认 9 组并行采集 150 次，没有超时，三轮均值为 **10.36-10.51ms/轮**，满足 100ms 外部目标和 95ms 内部截止时间。完整 protobuf 批次为 **56,346 bytes**，编码均值约 **0.38ms**。

不应把“API 能读取”直接等价为“每轮全部上报”。本机全进程清单约 387ms，逐进程数值采集约 1.47s，网络连接虽约 38ms，但包含地址、端口和 PID。此类数据同时违反 100ms 预算、低基数或最小披露原则，应该继续放在默认采集路径之外。

## 能力数量口径

以下清单由当前 `linux/amd64` 构建的 `go doc -all` 统计。函数和方法数量包含无 context 包装、`String`、构造函数和进程控制方法，只用于描述 API 表面积，不代表可直接上报的指标数。

| 功能包 | 包级导出函数 | 导出方法 | 结果结构体 | 直接数值字段 | 全部导出字段 |
| --- | ---: | ---: | ---: | ---: | ---: |
| `cpu` | 8 | 3 | 2 | 15 | 24 |
| `disk` | 10 | 3 | 3 | 19 | 28 |
| `docker` | 16 | 2 | 3 | 32 | 38 |
| `host` | 18 | 2 | 2 | 4 | 17 |
| `load` | 4 | 2 | 2 | 8 | 8 |
| `mem` | 7 | 6 | 5 | 55 | 56 |
| `net` | 32 | 11 | 9 | 36 | 49 |
| `process` | 9 | 100 | 10 | 47 | 49 |
| `sensors` | 3 | 2 | 3 | 6 | 8 |
| **合计** | **107** | **131** | **39** | **222** | **277** |

跨平台源码还有第 10 个功能域 `winservices`，它只在 Windows 构建中可用，因此未计入本次 Linux 字段和性能统计。不同操作系统的同名结构也可能有不同字段；`mem` 和 `sensors` 的 `Ex` API 明确属于平台扩展能力。

222 不是理论上限，主要原因如下：

- `net.ProtoCountersStat.Stats` 的键来自内核协议计数器，不能从结构体字段数静态确定。
- 嵌入结构没有重复计数，例如 Docker CPU 统计嵌入 CPU time 字段。
- 每 CPU、设备、挂载点、网卡、传感器、容器、进程和线程会把同一个字段展开为多个点。
- 字符串、列表和对象也属于 gopsutil 可读取数据，但不属于数值指标；其中很多是敏感资产信息。

当前仓库版本与上游能力说明参考：

- [gopsutil v4 文档](https://pkg.go.dev/github.com/shirou/gopsutil/v4)
- [gopsutil 源码仓库](https://github.com/shirou/gopsutil)

## 测试环境

测试时间：2026-08-05 UTC。

| 项目 | 值 |
| --- | --- |
| gopsutil | `v4.24.12` |
| Go | `go1.25.8 linux/amd64` |
| Kernel | Linux `6.8.0-136-generic` x86_64 |
| CPU | Intel Core i7-10700K，测试进程可用 12 个逻辑 CPU |
| Memory | 15,777,411,072 bytes |
| 运行身份 | 非 root，UID 1000；可读 Docker socket，不可读 DMI product UUID |
| 动态规模 | 约 11,904 个进程、57 条分区记录、5 个磁盘 I/O 设备、26 张网络接口、170 条连接、0 个可读温度传感器、17-18 个容器 |

这台机器的进程数明显偏高，适合暴露 `/proc` 全目录扫描和逐进程采集的扩展性问题。结果不能替代 Linux、Windows、macOS、BSD 以及低配机器上的独立测试。

## 默认采集器测试

每个分组执行 `50 ops x 3`，表中耗时是三次 `ns/op` 均值的范围，不是单次调用的 p95。点数和名称数在本次环境中稳定。内存是每次操作的 Go 分配量近似值。

| Agent 分组 | 主要 gopsutil API | 唯一名称/点 | 三轮均值范围 | 分配/轮 | 状态 |
| --- | --- | ---: | ---: | ---: | --- |
| `host` | `host.InfoWithContext` | 3 / 3 | 7.79-8.22ms | 0.97MiB | 可用 |
| `cpu` | counts、percent、times、info | 15 / 158 | 0.829-0.930ms | 357KiB | 可用 |
| `memory` | `mem.VirtualMemoryWithContext` | 38 / 38 | 0.0369-0.0400ms | 23KiB | 可用 |
| `swap` | `mem.SwapMemoryWithContext` | 10 / 10 | 0.0404-0.0456ms | 26KiB | 可用 |
| `load` | `load.AvgWithContext`、`load.MiscWithContext` | 8 / 8 | 7.59-8.05ms | 0.97MiB | 可用 |
| `filesystem` | physical partitions + usage | 8 / 8 | 0.187-0.215ms | 57KiB | 可用 |
| `disk_io` | `disk.IOCountersWithContext` | 11 / 55 | 0.183-0.192ms | 109KiB | 可用 |
| `network` | I/O、interface、protocol、filter、conntrack | 87 / 399 | 1.10-1.17ms | 949KiB | 降级，1 个子调用失败 |
| `sensors` | `sensors.TemperaturesWithContext` | 0 / 0 | 0.0189-0.0201ms | 2.5KiB | 无可读传感器 |
| **9 组并行** | 95ms context | **180 / 679** | **10.36-10.51ms** | **3.58MiB** | 0 超时，1 降级，1 不可用 |
| **Agent Sample** | 并行采集 + 摘要计算 | **180 / 679 扩展点** | **10.30-10.38ms** | **3.55MiB** | 满足预算 |

一次真实 Agent 快照中的采集器耗时为：`host` 11.461ms、`cpu` 1.846ms、`memory` 0.385ms、`swap` 0.651ms、`load` 11.338ms、`filesystem` 0.740ms、`disk_io` 1.264ms、`network` 2.189ms、`sensors` 0.130ms。它说明单轮现场值会高于热基准均值，但仍明显低于 95ms。

网络降级的具体原因是 `/proc/net/stat/nf_conntrack` 在本机不存在；`nf_conntrack_count` 和 `nf_conntrack_max` 可读，因此 filter 汇总仍能上报。传感器调用没有报错，但返回 0 个实体，Agent 正确标记为不可用而不是把温度显示为零。

## 批次与资源成本

| 项目 | 结果 |
| --- | ---: |
| 完整批次点数 | 692 |
| 唯一指标名 | 192 |
| 带标签点数 | 616 |
| protobuf 大小 | 56,346 bytes |
| protobuf 编码 | 0.370-0.441ms，复核值 0.380ms |
| 最新快照 JSON | 88,386 bytes |
| Ingest 点数限制 | 4,096 |
| Ingest 字符串体积限制 | 1MiB |

当前机器的点数约占入口点数限制 16.9%，protobuf 大小约占 1MiB 的 5.4%。这不是永久容量保证：接口、协议、CPU 数、虚拟网卡和设备数量增长都会线性增加点数和标签体积。

每轮约 3.6MiB 的临时分配比网络 payload 大得多，值得优先优化。`host.InfoWithContext` 和 `load.MiscWithContext` 都会调用进程计数，并分别扫描一次 `/proc`；在约 1.19 万进程时，两组各需要约 8ms 和 1MiB 分配。`host.InfoWithContext` 还读取 hostname、平台、虚拟化和 Host ID，Agent 最终没有上报这些字符串，但“没有上报”不等于“没有读取”。

另外，95ms context 能让聚合器按时返回，但 gopsutil Linux 的进程计数内部使用 `Readdirnames(-1)`，该系统目录读取本身不会在每个条目检查 context。极端情况下超时采集 goroutine 可能在聚合器返回后继续短暂运行；若采集周期过短，会形成资源叠加。

## 非默认能力测试

以下是单次筛查结果，用于判断数量级、权限和风险，不作为 SLA。基准代码只输出耗时、实体数和错误数，不输出读取到的敏感值。

| 能力 | 耗时 | 实体 | 调用错误 | 分配 | 结论 |
| --- | ---: | ---: | ---: | ---: | --- |
| 登录用户 | 0.057ms | 1 | 0 | 12KiB | 快，但含用户、终端和来源主机，默认不采集 |
| Linux 扩展内存 | 0.058ms | 1 | 0 | 10KiB | 轻量，可补 5 个平台字段 |
| 逐 swap 设备 | 0.041ms | 1 | 0 | 4.6KiB | 轻量，可补每设备 used/free |
| 全部分区枚举 | 0.305ms | 57 | 0 | 53KiB | 含伪文件系统，不直接上报 |
| 全挂载点 usage | 0.883ms | 21 成功 | 34 | 62KiB | 本机快，但大量不可用；网络挂载仍可能阻塞 |
| 磁盘 label/serial | 0.304ms | 5 设备 | 5 | 113KiB | 资产标识敏感，默认不采集 |
| 全系统网络连接 | 38.4ms | 170 | 0 | 3.86MiB | 接近预算且含地址、端口、PID，默认不采集 |
| 全进程清单 | 387ms | 11,904 | 0 | 128MiB | 超预算 |
| 逐进程数值，串行 | 1.47s | 11,904 | 23,703 | 564MiB | 超预算，进程退出/权限导致约 28% 子调用失败 |
| 逐进程数值，12 worker | 639ms | 11,903 | 23,713 | 564MiB | 仍超预算，并发不能解决基数问题 |
| 全进程线程 times | 812ms | 13,097 线程 | 0 | 263MiB | 超预算且基数更高 |
| Linux 扩展温度 | 0.070ms | 0 | 0 | 11KiB | 当前无实体；有传感器时可补 min/lowest/highest |
| Docker inventory | 13.8ms | 18 | 0 | 35KiB | 含容器 ID、名称、镜像和状态，需独立授权模型 |
| Docker cgroup CPU/memory | 13.6ms | 17 | 34 | 72KiB | 当前 cgroup v2 环境下旧版 cgroup v1 API 全部失败 |

逐进程数值测试只读取 times、memory、I/O、context switches、FD 数、线程数和 page faults；没有读取命令行、环境变量、工作目录、打开文件名或用户名。即便如此，耗时、分配、权限错误和时序基数都已不适合默认 100ms 主机采集路径。

## 覆盖缺口与建议

### 应补充的轻量数值

以下数据实测开销小，且不需要新增高风险身份字段，适合下一轮协议和客户端补齐：

1. `mem.ExLinux` 的 active file、inactive file、active anon、inactive anon、unevictable，共 5 个 Linux 字段。
2. `mem.SwapDevicesWithContext` 的每设备 used/free，共 2 个数值字段乘 swap 设备数；设备名沿用现有受控标签规则。
3. `sensors.ExLinux` 的 min、lowest、highest，共 3 个字段乘温度传感器数。
4. 已经读取但未上报的 `cpu.InfoStat.Cores` 和 `Stepping`；需要先确认跨平台语义和 protobuf 命名。

补充前仍需在 Linux、Windows、macOS 和 BSD 上确认 API 存在性，并继续使用运行时 capability/status 表达不可用，而不是补零。

### 应优化的默认路径

1. 将 `host.InfoWithContext` 改为 uptime、boot time 和一次共享进程计数的窄读取，避免读取不使用的 Host ID 等字符串。
2. 让 `host` 和 `load` 复用同一次进程计数，避免每轮两次扫描 `/proc`。
3. 缓存 boot time 等进程生命周期内不变的数据，只在注册或低频刷新时读取。
4. 增加每采集器耗时、超时和失败原因的本地统计；当前协议只有状态和本轮耗时，没有错误类别。
5. 在不同进程数、接口数、挂载点数和低配 CPU 下执行同一基准，验收 p95/p99 和最大值，而不只看本机均值。
6. 评估从 `v4.24.12` 升级到当前 gopsutil 版本；升级必须单独审查字段变化、cgroup v2 支持、性能和跨平台构建。

### 继续排除的能力

- 用户会话、Host ID、MAC、地址/端口、磁盘序列号和容器身份属于敏感资产数据。
- 进程、线程、连接和全挂载点属于高基数或不受控延迟数据。
- 若产品需要这些能力，应使用低频、分页、限额、显式授权的独立协议，不得加入默认每轮主机快照。

## 复现命令

默认 9 组和整轮基准：

```bash
go test ./agent/daemon/internal/service \
  -run '^$' \
  -bench '^BenchmarkSystemMetricCollectors/' \
  -benchtime=50x -count=3 -benchmem -timeout=3m
```

完整批次编码：

```bash
go test ./agent/daemon/internal/service \
  -run '^$' \
  -bench '^BenchmarkSystemMetricBatchEncoding$' \
  -benchtime=100x -count=3 -benchmem
```

Linux 非默认能力筛查：

```bash
go test ./agent/daemon/internal/service \
  -run '^$' \
  -bench '^BenchmarkGopsutilOptionalGroups/' \
  -benchtime=1x -count=1 -benchmem -timeout=3m
```

最后一组会在本地读取连接、用户、磁盘标识、容器和进程数据，虽然测试不会输出其值，也只应在获得授权的测试主机上运行。
