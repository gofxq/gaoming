# Gaoming Web Design System

本规范服务于 Gaoming 主机监控、实时指标与租户管理界面。视觉语言强调高对比排版、克制留白、精细材质和内容驱动的阅读节奏。Gaoming 是高频使用的运维工具：信息密度、状态识别与操作效率优先于展示性效果。

本文描述现代皮肤及全局组件规则。像素皮肤的 token、字体、组件形态和实现边界见 [PIXEL_DESIGN_SYSTEM.md](./PIXEL_DESIGN_SYSTEM.md)。

## 1. Design direction

### 1.1 Core principles

1. **Clarity before decoration**：先建立标题、关键指标、异常与操作的阅读顺序，再考虑材质与动效。
2. **Quiet confidence**：大面积使用中性色，品牌蓝只标记选择与关键操作；界面不依赖渐变、光斑或厚重阴影营造质感。
3. **Editorial hierarchy**：页面标题可以有明确尺度差，控制台内部标题保持紧凑；数字与说明之间形成稳定节奏。
4. **One surface, clear sections**：同一业务上下文尽量放在一个连续表面中，通过留白和分隔线分组，禁止卡片嵌套卡片。
5. **Motion explains change**：动效只用于状态切换、内容进入和层级变化，不使用持续漂浮、呼吸或纯装饰动画。
6. **Density with restraint**：关键数据保持可扫描，次要信息按需展开；刷新数据不得造成布局跳动。

### 1.2 Visual characteristics

- 使用系统字体、黑白高对比、宽松页面节奏、精细 1px 分隔、半透明全局导航和直接而简短的文案。
- 品牌资产只使用 Gaoming 自有内容，视觉表现服务于产品信息，不模仿其他品牌的标识、产品图或文案。
- Gaoming 的产品特征必须在首屏明确出现：主机状态、资源指标、实时性和异常处理，而不是营销型空白大图。

## 2. Foundations

### 2.1 Color tokens

浅色主题是默认主题；深色主题需通过相同语义 token 切换，组件内不得硬编码主题颜色。

| Token | Light | Dark | Usage |
| --- | --- | --- | --- |
| `--page` | `#f5f5f7` | `#000000` | 页面背景 |
| `--surface` | `#ffffff` | `#1c1c1e` | 一级内容表面 |
| `--surface-raised` | `#fbfbfd` | `#2c2c2e` | 控件、悬浮与次级表面 |
| `--nav` | `rgba(250,250,252,.82)` | `rgba(22,22,23,.82)` | 吸顶导航材质 |
| `--ink` | `#1d1d1f` | `#f5f5f7` | 标题、关键数字 |
| `--ink-secondary` | `#6e6e73` | `#a1a1a6` | 正文、辅助信息 |
| `--ink-tertiary` | `#86868b` | `#86868b` | 时间、占位与弱标签 |
| `--line` | `rgba(0,0,0,.10)` | `rgba(255,255,255,.14)` | 分隔线和默认边框 |
| `--line-strong` | `rgba(0,0,0,.18)` | `rgba(255,255,255,.24)` | 强调边界 |
| `--accent` | `#0071e3` | `#2997ff` | 链接、选择、主操作 |
| `--success` | `#248a3d` | `#30d158` | 正常、在线 |
| `--warning` | `#b25000` | `#ff9f0a` | 警告、需关注 |
| `--danger` | `#d70015` | `#ff453a` | 故障、离线、错误 |
| `--purple` | `#8944ab` | `#bf5af2` | 辅助数据系列，限图表 |

状态浅色背景使用语义色的 8%-12% 透明度。状态必须同时包含文本和形状标记，不得只靠颜色区分。正文对比度满足 WCAG AA，关键操作与焦点状态满足 3:1 非文本对比度。

### 2.2 Typography

```css
font-family: system-ui, "Segoe UI", "PingFang SC", "Helvetica Neue",
  Arial, sans-serif;
font-variant-numeric: tabular-nums;
letter-spacing: 0;
```

| Role | Desktop | Mobile | Weight |
| --- | --- | --- | --- |
| Page display | 48/1.08 | 34/1.12 | 600 |
| Page title | 32/1.16 | 28/1.18 | 600 |
| Section title | 22/1.25 | 20/1.25 | 600 |
| Panel title | 15/1.35 | 15/1.35 | 600 |
| Primary metric | 28/1.05 | 24/1.08 | 600 |
| Body | 14/1.55 | 14/1.5 | 400 |
| Label | 12/1.35 | 12/1.35 | 500 |
| Caption | 11/1.35 | 11/1.35 | 400 |

- 页面 display 只用于页面开场，不得放入卡片、工具栏或侧栏。
- 中英文混排优先系统字体，不额外下载或分发专有字体文件。
- 数字使用等宽数字特性；百分号、单位与数字保持同一基线。
- 字距保持 `0`。全大写标签不是默认样式，必要时也不增加字距。

### 2.3 Spacing and layout

- 基础单位：4px；常用间距：8、12、16、20、24、32、48、64px。
- 页面最大宽度：1440px；水平安全边距：桌面 40px、平板 24px、移动端 16px。
- 页面分区间距：桌面 48-64px，移动端 32-40px。
- 信息网格优先使用 `minmax(0, 1fr)` 和明确的 `min-width`，防止长主机名撑开页面。
- 固定格式区域需定义稳定高度或 `aspect-ratio`，加载、刷新和 hover 不得改变整体尺寸。

### 2.4 Shape and elevation

| Element | Radius | Elevation |
| --- | --- | --- |
| 一级面板、重复项 | 8px | 默认无阴影 |
| 输入、按钮、分段控件 | 6px | 默认无阴影 |
| 状态点、头像 | 50% | 无阴影 |
| 状态标签 | 999px | 无阴影 |
| 导航浮层、菜单 | 8px | `0 12px 36px rgba(0,0,0,.12)` |

- 主要结构依靠背景对比、边框与留白，而非阴影。
- Hover 阴影最多使用 `0 8px 24px rgba(0,0,0,.08)`，且不得改变元素尺寸。
- 禁止装饰性渐变、彩色光斑、拟物高光和多层毛玻璃叠加。

## 3. Materials

### 3.1 Global navigation

只有吸顶全局导航可默认使用半透明材质：

```css
background: var(--nav);
border-bottom: 1px solid var(--line);
-webkit-backdrop-filter: saturate(180%) blur(20px);
backdrop-filter: saturate(180%) blur(20px);
```

不支持 `backdrop-filter` 时退化为不透明 `--surface`。导航高度固定，滚动过程中不得缩放品牌或改变布局。

### 3.2 Content surfaces

- 一级业务区使用 `--surface`，1px `--line` 边框；同一表面内通过分隔线组织摘要、列表与图表。
- 次级单元使用 `--surface-raised`，不得再次添加 blur 或强阴影。
- 页面背景保持纯色。数据图表可使用语义色折线和淡色网格，不使用面积渐变。

## 4. Components

### 4.1 Global bar

- 固定高度 48px，品牌、主导航、实时状态和账户操作保持单行。
- 当前导航使用文字颜色或 2px 底线表达，不使用大面积彩色胶囊。
- 820px 以下隐藏次要导航，保留品牌、实时状态与必要操作。
- 图标按钮使用 Semi Icons 或项目现有图标库，提供 tooltip 与 `aria-label`。

### 4.2 Page introduction

- 使用一句短 eyebrow、明确标题和一行支持文案；监控页面 display 标题不超过两行。
- 右侧只放同一层级的关键动作或同步时间，不能堆叠统计卡片。
- 首屏下缘应露出核心数据区，避免开场区域占满整个视口。

### 4.3 Segmented control and filters

- 时间范围、视图模式等互斥选项使用分段控件；选中项使用实色表面和轻边框。
- 二元设置使用 switch 或 checkbox；搜索使用带搜索图标的输入框。
- 主按钮只用于当前页面唯一的关键动作；普通筛选不得全部使用品牌蓝。
- 默认控件高 36px，紧凑工具栏可用 32px；触控区域至少 44px。

### 4.4 Cluster summary

- 顺序固定：全部、正常、关注、离线、平均 CPU、平均内存。
- 关键值使用 24-28px 半粗体，标签 12px；单元之间使用连续分隔线。
- 资源百分比使用水平进度条，不使用圆形仪表盘。
- 小屏幕使用两列，禁止页面级横向滚动。

### 4.5 Host row/card

- 桌面端优先 3 列网格；高密度场景可切换为表格视图。移动端单列。
- 单项只显示主机身份、状态、CPU、内存、磁盘、网络和更新时间。
- 状态由 8px 圆点、文本与语义色共同表达。
- 选中态使用 `--accent` 边框和浅色背景；hover 最多上移 1px。
- 卡片内部指标是连续网格，不得再包裹独立卡片或添加阴影。

### 4.6 Host detail and charts

- 详情在主机列表后原位展开，不使用模态框承载长趋势图。
- 摘要、指标和趋势属于同一连续表面；标题与图表之间使用 24px 间距。
- 折线宽 2px，网格线 1px，数据点默认隐藏；hover/focus 时显示数据点和 tooltip。
- 少于两个有效样本时显示明确空状态。图例不得遮挡曲线或缩小绘图区。

### 4.7 Forms and tables

- 输入、选择器与按钮默认 36px 高，标签始终可见；不要只依赖 placeholder。
- 表格表头固定、行高 44-48px，数字右对齐，身份与状态左对齐。
- 窄屏允许表格容器横向滚动，但页面本身不得横向滚动。
- 保存、加载、禁用、空数据和错误状态必须有文字反馈。

## 5. Responsive behavior

| Breakpoint | Behavior |
| --- | --- |
| `>= 1280px` | 3 列主机、6 列摘要、4 列趋势图，页面边距 40px |
| `768-1279px` | 2 列主机、3+3 摘要、2 列趋势图，页面边距 24px |
| `480-767px` | 单列主机、2 列摘要、单列趋势图，收起次要导航 |
| `< 480px` | 页面边距 16px，工具栏分行，主操作保持全宽或图标化 |

所有断点必须满足：无页面级横向滚动、文字不覆盖、最长主机名可省略、动态数字不改变列宽、交互目标不小于规定尺寸。

## 6. Motion and interaction

- 颜色、边框、透明度：`160ms ease-out`；位置与展开：`240ms cubic-bezier(.2,.8,.2,1)`。
- 页面加载仅允许内容淡入并上移 8px，最多分两批，总时长不超过 500ms。
- 实时刷新不得让整块内容闪烁；仅更新数字与时间。
- `prefers-reduced-motion: reduce` 时取消位移、平滑滚动与非必要过渡。
- 可点击区域必须有 hover、active 和 `:focus-visible` 状态；focus ring 使用 3px 半透明品牌蓝。

## 7. Content and accessibility

- 文案简短、直接、以任务为中心，例如“3 台主机需要关注”，避免营销式夸张表达。
- 日期、单位和状态命名在全站保持一致；不使用只有内部团队理解的缩写。
- 图标不能成为唯一标签；装饰图形设置 `aria-hidden="true"`。
- 图表提供文本摘要或可访问表格；状态变化通过 `aria-live="polite"` 通知。
- 主题切换后仍需保持 WCAG AA 对比度，且尊重系统 `prefers-color-scheme`。

## 8. Implementation rules

- 现代皮肤的全局 token 位于 `src/styles/global.css`；页面专属样式留在对应页面样式文件。
- Pixel 通用 token/字体位于 `src/shared/styles/themes/pixel`，PC/H5 展示覆写分别位于各端的 `components/pixel`。
- 组件优先复用 Semi UI 与 `@douyinfe/semi-icons`，不得为常见图标手绘 SVG。
- 新页面先复用语义 token，不创建只差轻微色值的新 token。
- 添加容器前先判断能否用留白、分隔线或网格完成，禁止卡片嵌套卡片。
- 生产实现不依赖 demo 代码；`tmp` 下 demo 只用于视觉评审与交互验证。
- 视觉变更至少验证 1440 x 900、768 x 1024、390 x 844，并检查浅色、深色、键盘焦点和减少动效模式。

## 9. Reference demo

单文件交互示例位于 [`tmp/design-system-demo.html`](tmp/design-system-demo.html)。它用于验证本规范的排版、颜色、响应式和交互，不是生产组件源码。
