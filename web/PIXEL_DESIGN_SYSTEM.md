# Gaoming Pixel UI Design System

本文档是 Gaoming 像素化前端界面的设计与实现规范。它描述当前已经落地的视觉语言，并约束后续 PC、H5 页面扩展时的颜色、字体、边框、组件、交互、可访问性和性能行为。

Pixel 是视觉皮肤，不是另一套业务应用。页面结构、API、状态、表单和 payload 必须与现代皮肤共享；只有 CSS 无法表达结构差异时，才允许增加轻量展示变体。

## 1. 设计目标

### 1.1 核心原则

1. **信息优先**：像素效果必须服务于主机状态、指标和异常识别，不能降低监控信息的扫描效率。
2. **硬朗而非花哨**：通过直角、实线、硬阴影、网格和有限的点阵纹理建立风格，不使用模糊光晕或复杂插画堆叠。
3. **语义一致**：现代皮肤与 Pixel 皮肤使用同一组语义 token；皮肤变化不得改变状态含义和操作结果。
4. **结构复用**：优先使用同一套 DOM 和组件，通过 `data-skin="pixel"` 覆写视觉表现。
5. **低频动效**：动效只表达 hover、focus、选中和展开，不使用持续闪烁、扫描线或呼吸动画。
6. **明暗完整**：任何新增组件必须同时支持 Pixel Light 和 Pixel Dark。

### 1.2 视觉关键词

- 12px 中文像素字
- 暖纸张色与棕红强调色
- 0 圆角、2px 结构边框
- 右下方向的无模糊硬阴影
- 24px 背景网格、虚线与条纹
- 终端式标题前缀和明确的键盘焦点

### 1.3 非目标

- 不将所有图标替换成自绘像素 SVG。
- 不复制 PC/H5 页面或业务组件来实现换肤。
- 不使用低分辨率位图模拟边框、背景或按钮。
- 不为了装饰引入持续动画、大图背景或第二套字体。

## 2. 主题模型

外观由两个互相独立的维度组成：

| 维度 | 值 | 职责 |
| --- | --- | --- |
| `data-skin` | `modern` / `pixel` | 形状、字体、材质和装饰语言 |
| `data-theme` | `light` / `dark` | 明暗色值 |

Pixel 选择器必须挂在根节点条件下：

```css
:root[data-skin="pixel"] .component { /* Pixel Light 与 Dark 共用 */ }
:root[data-skin="pixel"][data-theme="dark"] { /* 只覆盖深色 token */ }
```

禁止使用独立的 `.pixel-app` 容器或在 React 组件内通过三元表达式维护两套大段 DOM。皮肤与主题由 `AppearanceProvider` 管理，并在 React 挂载前恢复到 `<html>`，避免首屏皮肤闪烁。

## 3. Design tokens

token 的实现源位于 `src/shared/styles/themes/pixel/tokens.css`。组件只能引用语义 token，不直接判断 Light/Dark。

### 3.1 颜色

| Token | Pixel Light | Pixel Dark | 用途 |
| --- | --- | --- | --- |
| `--page` | `#f1e0c8` | `#1a0f0b` | 页面底色 |
| `--surface` | `#fff7e8` | `#2a1812` | 主面板、输入和控件表面 |
| `--surface-raised` | `#ead0ad` | `#40261b` | hover、次级表面 |
| `--nav` | `rgba(250,237,216,.97)` | `rgba(31,17,12,.97)` | 顶部导航 |
| `--ink` | `#3b251c` | `#ffe7c4` | 标题、核心数字 |
| `--ink-secondary` | `#6b4a38` | `#d2aa84` | 正文、次级信息 |
| `--ink-tertiary` | `#8c6951` | `#a77d5f` | 时间、占位和弱标签 |
| `--line` | `#bb9272` | `#704735` | 默认边框和分隔 |
| `--line-strong` | `#6d432e` | `#d18e5d` | 强边界和硬阴影 |
| `--accent` | `#b94f2c` | `#ff9258` | 当前项、焦点和主操作 |
| `--success` | `#557849` | `#9ac876` | 正常、在线 |
| `--warning` | `#a96600` | `#ffc65f` | 警告、需关注 |
| `--danger` | `#ad3f38` | `#ff7b70` | 错误、故障、离线 |
| `--purple` | `#76517f` | `#dda4e7` | 辅助图表序列 |
| `--chart-grid` | `rgba(109,67,46,.18)` | `rgba(210,170,132,.18)` | 背景和图表网格 |

语义色的弱背景使用对应 `--*-soft` token。临时透明色优先通过 `color-mix()` 从语义 token 派生，禁止另建只差少量透明度的固定色值。

### 3.2 圆角与边框

Pixel 皮肤将 `--radius-panel` 和 `--radius-control` 统一设置为 `0`。

| 层级 | 规则 |
| --- | --- |
| 内容分隔 | `1px solid var(--line)`，必要时使用 dashed |
| 输入、工具栏、面板 | `2px solid var(--line-strong)` 或组件已有语义边框 |
| 选中态 | `2px solid var(--accent)`，可配 `outline-offset: 3px` |
| 状态标签 | `1px solid currentColor`，必须同时保留状态文字 |

圆点状态在 Pixel 中应转为小方块。头像可以保留圆形，因为它表达人物身份而不是组件材质。

### 3.3 阴影

所有 Pixel 阴影均为右下方向、无模糊半径：

| 用途 | 推荐值 |
| --- | --- |
| 小状态/图标 | `2px 2px 0` |
| 输入和品牌标记 | `3px 3px 0` |
| 一级面板 | `5px 5px 0` |
| 卡片 hover | `7px 7px 0` |

阴影颜色从 `--line-strong`、`--accent` 或当前状态色派生。禁止同时使用硬阴影与大面积柔和阴影。

## 4. 字体与排版

### 4.1 字体栈

```css
font-family: "Fusion Pixel 12px Proportional SC", "SFMono-Regular",
  Consolas, "Liberation Mono", Menlo, "PingFang SC", monospace;
```

- 当前只加载 `Fusion Pixel 12px Proportional SC` 400 字重。
- 层级优先依靠字号、颜色、留白和边框建立，不得只依赖粗体。
- 数字继续使用 `font-variant-numeric: tabular-nums`，实时刷新不得造成列宽跳动。
- 正文默认不增加字距；品牌、导航、eyebrow 等短标签可使用 `0.06em`。
- 中文正文行高不得低于 `1.45`，12px 字号只用于标签和辅助信息。

### 4.2 标题语法

Pixel 标题允许使用下列终端式前缀：

| 场景 | 形式 | 示例 |
| --- | --- | --- |
| 页面/面板标题 | `> ` | `> 主机列表` |
| eyebrow / kicker | `[ 文本 ]` | `[ 基础设施 · 现在 ]` |

前缀通过伪元素生成，不写入业务文案，也不应被屏幕阅读器重复朗读。一个视觉区域只使用一种前缀，禁止叠加 `> [ 标题 ]`。

## 5. 网格、间距与材质

- 延续现代皮肤的 4px 基础间距系统，不建立“像素专属间距表”。
- 页面背景使用 24px × 24px、1px 线宽的静态网格。
- 内容布局和断点与现代皮肤保持一致；换肤不得改变信息顺序。
- 一级区域使用连续表面和明确分隔，不因 Pixel 风格增加卡片嵌套。
- 可使用低对比重复线性渐变表现纸张、终端或条纹，但单个组件最多两层背景纹理。
- 顶栏 Pixel 高度为 52px，使用不透明度较高的 `--nav`，关闭 `backdrop-filter`。

## 6. 组件规范

### 6.1 顶栏与品牌

- 品牌标记使用 2px 边框、直角和 3px 强调色硬阴影。
- 当前导航使用 4px 点阵式底线，不使用圆角胶囊。
- 工具区保持单行；移动端沿用 44px 最小触控区域。
- 皮肤、主题、刷新和退出按钮必须有 `aria-label`，桌面端提供 tooltip。

### 6.2 按钮和分段控件

- 所有按钮直角化，图标按钮不得因 Pixel 样式缩小触控面积。
- 分段控件容器使用 2px 边框；当前项使用 `--accent` 实色背景和 2px 硬阴影。
- hover 可以添加内描边；active 不得产生尺寸变化。
- 皮肤/主题按钮使用 `aria-pressed` 表达当前状态。

### 6.3 输入和搜索

- 输入框使用 2px 边框、0 圆角和 3px 硬阴影。
- `focus-within` 将硬阴影切换为 `--accent`，不能只依赖颜色很弱的背景变化。
- placeholder 不是标签；搜索输入仍需可访问名称。
- 错误态同时使用文字和 `--danger`，不能只改变边框颜色。

### 6.4 面板和卡片

- 一级面板使用 2px 边框和 5px 硬阴影。
- 主机卡 hover 可向左上移动 2px，并将阴影扩展到 7px；移动不能改变网格占位。
- 选中主机使用 `--accent` 边框和外置 outline，不能只靠阴影表达。
- 卡片背景纹理保持低对比，不得干扰主机名和指标数值。

### 6.5 状态标签

- 状态标签保持“方块标记 + 文本 + 语义色”三重表达。
- 状态点在 Pixel 中使用 0 圆角和 2px 硬阴影。
- success、warning、danger 的含义不得因主题变化互换。
- 实时状态更新不得触发整张卡片闪烁或重新进入动画。

### 6.6 摘要、指标和进度条

- 集群摘要保持连续分区，分隔线可加粗到 2px。
- 进度条和轨道使用 0 圆角；填充可添加 6px/2px 间隔的静态条纹。
- 指标单元允许右下角小面积强调色折角，但不得遮挡内容。
- 数字、单位和百分号保持同一基线。

### 6.7 图表

- 折线宽度为 3px，使用 `stroke-linecap: square`、`stroke-linejoin: miter`。
- SVG 折线与网格可使用 `shape-rendering: crispEdges`。
- 网格线使用 `3 5` 虚线，不增加发光或阴影滤镜。
- Pixel 只改变曲线表现，不改变采样、坐标、tooltip 和空状态逻辑。

### 6.8 表格与登录卡片

- 表头可使用低对比斜线纹理，数据行使用 dashed 分隔。
- meta pill、错误提示和 select 使用 2px 直角边框。
- 登录卡片可以添加右上角短点阵装饰，但装饰元素必须由 CSS 伪元素生成。
- 保存、加载、禁用和错误反馈仍使用明确文字。

## 7. 交互与动效

| 状态 | 行为 |
| --- | --- |
| Hover | 最多向左上移动 1-2px，并同步增加右下硬阴影 |
| Active | 保持组件尺寸，可减少位移或阴影模拟按下 |
| Selected | 强调色边框/背景，并保留文字或 `aria-pressed` |
| Focus visible | `2px dashed var(--accent)`，`outline-offset: 3px` |
| Loading | 稳定占位，不使用持续扫描线 |

- 颜色、边框和位移延续全局 `160ms ease-out` 节奏。
- 禁止 CRT 闪烁、无限扫描线、随机抖动和持续发光。
- `prefers-reduced-motion: reduce` 时取消非必要位移和动画。
- hover 只用于有指针设备；移动端不能依赖 hover 才显示必要操作。

## 8. 响应式与 H5

Pixel 不定义第二套响应式断点，继续遵守主设计系统的布局规则。PC 与 H5 共享 token、字体和基础 Pixel 材质，但组件覆写分别维护：

```text
src/
  pc/
    pages/                 # PC 页面和 Semi UI 布局
    components/pixel/      # PC 专属 Pixel 展示覆写
  h5/
    pages/                 # IonPage / IonContent 页面
    components/pixel/      # H5 专属 Pixel 展示覆写
  shared/
    features/              # API、状态、表单、payload
    styles/themes/pixel/   # 通用 Pixel token、字体和基础材质
```

H5 页面必须继续使用 `IonPage` / `IonContent` 骨架。不要把 Semi UI 组件或 PC Pixel 覆写直接导入 H5；也不要在 H5 Pixel 组件中复制请求和状态逻辑。

## 9. 实现约束

### 9.1 文件职责

| 文件/目录 | 职责 |
| --- | --- |
| `shared/styles/themes/pixel/tokens.css` | Light/Dark 颜色、圆角、阴影和字体 token |
| `shared/styles/themes/pixel/base.css` | 页面网格和通用字体应用 |
| `shared/styles/themes/pixel/index.css` | Pixel 字体与通用样式入口 |
| `pc/components/pixel/PcPixel.css` | PC 壳层、Semi UI、登录和用户页覆写 |
| `pc/components/pixel/PcDashboardPixel.css` | Dashboard 专属 Pixel 表现 |
| `shared/features/appearance/AppearanceProvider.tsx` | 皮肤/主题状态与持久化 |

### 9.2 新组件接入流程

1. 先完成语义正确、可访问、支持 Light/Dark 的基础组件。
2. 优先通过已有 token 自动适配 Pixel。
3. 确有必要时，在所属端的 `components/pixel` 中添加带根条件的覆写。
4. 只有 DOM 层级或交互模型无法共享时，才创建 `*Pixel.tsx` 展示变体。
5. 展示变体只接收 props，不发请求、不持有业务状态、不定义 payload。

所有 Pixel CSS 选择器必须以 `:root[data-skin="pixel"]` 开头。禁止无条件覆盖 Semi UI 或基础页面样式。

## 10. 可访问性

- 正文和关键数字需满足 WCAG AA；边框、焦点和选中态需保持至少 3:1 非文本对比。
- 像素字体不得成为唯一的信息表达手段；小字号中文需人工检查可读性。
- 图标按钮必须提供可访问名称，状态不得只依赖颜色。
- `::before` / `::after` 装饰不得承载业务信息。
- 键盘顺序、屏幕阅读器文本和图表摘要与现代皮肤保持一致。
- Light、Dark 都要检查 focus、disabled、error、empty 和 loading 状态。

## 11. 性能预算

当前 Pixel 字体 WOFF2 约 602 KB，是皮肤最大的首屏成本。后续实现必须遵守：

- 不再引入第二套中文像素字体。
- 新装饰优先使用 CSS，不新增大尺寸位图或动画资源。
- 字体优化优先考虑子集化、按需加载或只发布 WOFF2。
- 避免在长列表每一项叠加多层渐变、滤镜或大面积阴影。
- 禁止持续运行的装饰动画；实时数据更新只修改必要节点。
- Pixel CSS 增量应保持可审查，页面专属规则不得回流到通用 token 文件。

## 12. Do / Don't

| Do | Don't |
| --- | --- |
| 使用直角、2px 边框和无模糊硬阴影 | 使用大圆角、柔光和多层毛玻璃 |
| 使用语义 token 派生状态色 | 在组件中硬编码 Light/Dark 两套颜色 |
| 复用同一 DOM，通过 CSS 换肤 | 复制完整页面实现 Pixel 版本 |
| 使用静态网格和低对比条纹 | 使用持续扫描线和 CRT 闪烁 |
| 保留文字、形状和颜色三重状态提示 | 只靠颜色或像素图标表达状态 |
| 在 PC/H5 各自目录维护展示覆写 | 将 Semi UI 覆写放进 H5 或 shared 业务层 |

## 13. 视觉验收清单

- [ ] Pixel Light 与 Pixel Dark 均完成验证。
- [ ] 1440×900、768×1024、390×844 无页面级横向滚动。
- [ ] 标题、正文、标签和动态数字没有裁切或跳动。
- [ ] hover、active、selected、focus-visible 可明确区分。
- [ ] loading、empty、error、disabled 状态有文字反馈。
- [ ] 键盘可完成主要操作，焦点虚线完整可见。
- [ ] 状态不只依赖颜色，图标按钮具有 `aria-label`。
- [ ] `prefers-reduced-motion` 下无非必要位移和动画。
- [ ] 新 Pixel 规则位于正确的 shared、PC 或 H5 目录。
- [ ] `yarn typecheck` 与 `yarn build` 通过。
