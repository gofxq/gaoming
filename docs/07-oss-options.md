# 可以简化当前代码的开源选项

当前仓库的现状是：

- 主机指标采集已经改为 `gopsutil`
- 后端实时推送仍然使用原生 SSE
- 前端实时订阅仍然使用原生 `EventSource`
- 时间序列图仍然使用内联 SVG

这套组合的好处是运行依赖少、链路清晰；坏处是图表和前端状态管理代码会慢慢变长。所以如果后面继续扩展，下面几个开源方案值得考虑。

## 当前已经采用的库

### `gopsutil`

官方仓库：

- https://github.com/shirou/gopsutil

当前项目已经实际使用它来替换手写 `/proc` 采集逻辑，原因是：

- 更适合宿主机 agent
- 支持 CPU、内存、磁盘容量、磁盘 IO、网络等统一采集
- 对未来的 Windows/macOS/Linux 兼容更友好

## 1. htmx SSE Extension

官方地址：

- https://htmx.org/extensions/sse/

适合场景：

- 页面以服务端渲染为主
- 想减少手写 `EventSource`、事件绑定和局部 DOM 更新代码
- 更偏向“HTML 片段推送”而不是前端状态管理

对当前项目的价值：

- 如果后面状态表和卡片都改成服务端渲染片段，`htmx + SSE extension` 能明显减少前端事件处理样板代码。
- 对“复杂时序图”帮助不大，因为图表本身仍然要自己维护。

## 2. uPlot

官方仓库：

- https://github.com/leeoniya/uPlot

适合场景：

- 重点是实时折线、时间序列、窗口切换
- 想保持体积小、性能高
- 不想引入很重的图表框架

对当前项目的价值：

- 很适合替换当前自绘 SVG 负载图。
- 如果后面要加 CPU、内存、网络多曲线图，`uPlot` 会比继续手写 SVG 更省代码。
- 对当前这个监控面板来说，它是最贴近需求的图表库选项。

## 3. Apache ECharts

官方文档：

- https://echarts.apache.org/handbook/en/get-started/

适合场景：

- 后面要做更多面板类型，比如堆叠图、饼图、拓扑图、告警趋势图
- 想快速堆出更丰富的可视化能力

对当前项目的价值：

- 如果未来要做完整监控大盘，ECharts 会比现在这套原生实现省很多图表代码。
- 代价是体积和复杂度明显高于当前零依赖实现，也高于 `uPlot`。

## 当前建议

如果只继续增强当前状态页：

- 继续保留 `gopsutil + SSE` 的后端组合
- 图表层优先评估 `uPlot`

如果要把页面进一步改成“服务端返回 HTML 片段 + 实时局部刷新”：

- 可以评估 `htmx SSE extension`

如果目标是完整监控大盘：

- 直接评估 `Apache ECharts`
