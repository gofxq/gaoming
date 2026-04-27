# Web 最小迁移方案（分 3 个 PR）

本文档定义前端“最小可落地”迁移路径，目标是在不重写页面的前提下，尽快降低重复代码和后续维护成本。

## 范围与目标

- 当前问题：
  - 数据请求与状态管理分散在 `useEffect + useState`，重复处理 loading/error。
  - 图表组件大量手写 SVG，业务代码和绘图细节耦合。
  - 样式体量集中在单个 `global.css`，后续新增页面容易继续膨胀。
- 目标：
  - 先处理“代码体量最大、收益最高”的层：数据流和图表。
  - 样式层采取渐进策略，避免一次性重写。

## PR1：引入 React Query 并迁移用户管理页（先做）

### 变更内容

- 增加依赖：`@tanstack/react-query`。
- 在应用 Provider 层接入 `QueryClientProvider`。
- 新增轻量 HTTP 工具（统一 JSON 解析、错误处理）。
- 将 `UsersPage` 从 `useEffect + useState` 迁移到 `useQuery + useMutation`。

### 验收标准

- 用户列表加载逻辑不再使用页面级 `useEffect`。
- 更新用户角色/状态后，列表数据能正确刷新。
- 页面交互和现有 UI 保持一致。

### 回滚策略

- 仅撤销 `UsersPage` 和 Query Provider 改动即可，影响面小。

## PR2：替换手写图表为图表库

### 变更内容

- 选型建议：`recharts`（轻量、React 生态成熟）。
- 优先替换：
  - Dashboard: `MetricChart`
  - Mobile: `InsetTrend`、`DualInsetTrend`、`DonutGauge`、`BalanceDonut`
- 保留现有业务指标计算逻辑，仅替换渲染层。

### 验收标准

- 图表代码总行数显著下降。
- 图表展示和现有视觉语义一致（颜色、趋势方向、空态）。
- 不引入明显性能回退。

### 回滚策略

- 每个图表组件独立提交，出现问题可按组件粒度回退。

## PR3：样式体系渐进收敛（Tailwind 可选）

### 方案选择

- 推荐最小路径：
  - 保留现有语义类命名和 design token。
  - 先抽取可复用 UI primitives（按钮、卡片、状态标签）。
- Tailwind 作为可选：
  - 若确认团队希望使用 utility-first，再在此 PR 引入。
  - 引入时建议配套 `clsx` 和 `class-variance-authority`，避免 className 失控。

### 验收标准

- `global.css` 不再继续单文件膨胀。
- 新增页面优先复用 primitives，而非新增大段样式。
- 若启用 Tailwind，需保持 token 和品牌风格一致。

### 回滚策略

- Tailwind 仅作为样式层改造，不和业务逻辑提交混合，便于独立回退。

## 实施顺序

1. PR1（本轮开始执行）
2. PR2（图表替换）
3. PR3（样式层收敛）

## 建议提交信息

- PR1：`refactor(web): adopt react-query for users page and shared http client`
- PR2：`refactor(web): replace custom svg charts with recharts`
- PR3：`refactor(web): consolidate styling primitives and optional tailwind baseline`
