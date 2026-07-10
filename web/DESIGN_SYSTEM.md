# Gaoming Web Design System

本规范服务于高明主机监控、实时指标和租户管理界面。设计目标是让高频运维信息清晰、稳定、可快速扫描；毛玻璃用于表达层级，不作为装饰。

## 1. Design principles

1. **Data first**: 页面标题、健康状态、资源占用和异常主机优先于品牌表达。
2. **Flat inside, glass outside**: 顶栏和一级业务面板使用毛玻璃；面板内部用分隔线、浅色底和留白组织内容，禁止卡片嵌套卡片。
3. **Status has meaning**: 蓝色表示操作和选择，绿色表示正常，琥珀表示警告，红色表示故障，紫色只用于辅助数据系列。
4. **Stable density**: 控件和指标块使用固定高度、稳定网格，数据刷新不应引起布局跳动。
5. **Progressive disclosure**: 主机卡只呈现关键指标；完整指标与趋势在选中主机后展开。

## 2. Foundations

### Color tokens

| Token | Value | Usage |
| --- | --- | --- |
| `--canvas` | `#eef2f7` | 应用背景 |
| `--glass` | `rgba(255,255,255,.72)` | 一级玻璃面板 |
| `--surface` | `#ffffff` | 输入和实色表面 |
| `--ink` | `#111827` | 标题与主要数据 |
| `--muted` | `#667085` | 辅助信息 |
| `--line` | `rgba(15,23,42,.09)` | 分隔线与默认边框 |
| `--blue` | `#2563eb` | 主操作、当前选择 |
| `--green` | `#0f9f6e` | 正常、在线 |
| `--orange` | `#d97706` | 警告、需关注 |
| `--red` | `#dc2626` | 严重、离线、错误 |
| `--violet` | `#7c3aed` | 辅助数据系列 |

浅色语义背景使用相应的 `*-soft` token。正文与背景对比度应满足 WCAG AA；不得只依靠颜色传达状态，需同时提供圆点和文本。

### Typography

- 字体栈：`Inter, SF Pro Text, PingFang SC, Microsoft YaHei, sans-serif`。
- 页面标题：28px / 600；移动端 24px。
- 面板标题：16px / 600；分区标题 13px / 600。
- 主要数据：14-24px，使用等宽数字特性 `font-variant-numeric: tabular-nums`。
- 正文：13px；辅助文本 9-11px。
- 字距保持默认值 `0`；仅全大写的短标签可使用 `0.12em`。

### Spacing and shape

- 基础间距单位为 4px；常用间距为 8、12、16、18、24、30px。
- 一级面板圆角 8px，控件圆角 6px，状态标签使用胶囊圆角。
- 图标按钮为 36 x 36px；默认控件高度 36px；移动端触控区域不得小于 36px。
- 阴影只用于玻璃面板和悬浮状态，内部数据块不使用阴影。

## 3. Glass system

### Level 1: Application chrome

用于顶栏和一级业务面板：

```css
background: rgba(255, 255, 255, 0.72);
border: 1px solid rgba(255, 255, 255, 0.78);
backdrop-filter: blur(18px) saturate(135%);
box-shadow: 0 16px 48px rgba(15, 23, 42, 0.08);
```

### Level 2: Internal surface

列表项、指标网格和图表使用 `rgba(255,255,255,.6)` 或 `--surface-soft`。不再叠加 `backdrop-filter`，通过 1px 分隔线建立结构。

### Fallback

不支持 `backdrop-filter` 时，一级面板必须降级为 `rgba(255,255,255,.96)`，确保文字对比度不依赖背景模糊。

## 4. Components

### Top bar

- 桌面端吸顶，品牌、主导航、实时连接状态、筛选控件和用户身份保持单行。
- 820px 以下改为两行，560px 以下隐藏导航文字但保留图标和可访问名称。
- 图标按钮必须提供 tooltip 和 `aria-label`。

### Cluster summary

- 顺序固定为全部、正常、关注、离线、平均 CPU、平均内存。
- 统计值不使用圆形图；资源百分比使用水平进度，便于快速横向比较。
- 小屏幕使用两列，禁止横向滚动。

### Host card

- 最小桌面宽度 360px；移动端单列。
- 左侧 3px 状态线、状态图标和状态文字共同表达健康度。
- 卡片只显示 CPU、内存、磁盘、网络 RX，以及 Load、指标时间、心跳时间。
- Hover 可上移 2px；选中状态使用同一套边框和阴影，不改变尺寸。

### Host detail

- 详情在主机列表下方展开，不使用模态框，避免趋势图被小视口裁切。
- 元数据、指标和图表各自使用连续网格；内部单元格不做独立卡片。
- 图表缺少两个有效样本时显示明确空状态。

### Form and table

- 搜索框、选择器与按钮统一为 36px 高度。
- 用户表格在窄屏保留最小内容宽度并允许表格区域横向滚动，页面本身不横向滚动。
- 保存、加载、禁用和错误状态必须有文字反馈。

## 5. Responsive behavior

| Breakpoint | Behavior |
| --- | --- |
| `> 1280px` | 完整六列集群摘要、八列指标、四列趋势图 |
| `761-1280px` | 集群摘要 4+2、四列指标、两列趋势图 |
| `521-760px` | 两列摘要、两列指标、两列趋势图 |
| `<= 520px` | 单列主机和趋势图，主机关键指标保持两列 |

所有断点必须满足：无页面级横向滚动、文字不覆盖、交互目标不因动态数据改变尺寸。

## 6. Motion and accessibility

- Hover、focus 和展开反馈使用 150ms ease；不使用连续装饰动画。
- `prefers-reduced-motion: reduce` 时移除动画和滚动过渡。
- 所有键盘操作元素必须保留可见 focus ring。
- 状态、图表和按钮需提供可读文本或 `aria-label`；装饰性图形不得成为唯一信息来源。

## 7. Implementation rules

- 全局 token 和应用壳层位于 `src/styles/global.css`。
- Dashboard 专属样式位于 `src/pc/PcDashboardPage.css`。
- 新页面必须优先复用 token，不直接增加相近色值或新的玻璃层级。
- 添加组件前先判断是否可由分隔线、网格或现有 Semi UI 控件完成。
- 视觉改动至少验证 1440 x 900、768 x 1024 和 390 x 844 三个视口。
