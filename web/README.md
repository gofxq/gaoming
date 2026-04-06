# Web

这个目录用于承载 `Gaoming` 的同仓前端项目。

当前目标不是立刻把所有页面重写完，而是先把一个独立、可演进的 React 工程边界落下来，让后续的鉴权、分租户、前端配置能力有稳定落点。

## 本地开发

默认测试接口指向 `http://localhost:8080/`。

开发模式默认通过 Vite 代理把 `/master/*` 转发到这个地址，因此浏览器侧仍然走同源请求，不需要远端额外放开 CORS。即使直接运行 `yarn dev`，也会默认代理到这个地址；如果要切到本地或其他环境，再覆盖代理目标。

```bash
make web-install
make web-dev
```

如果要切换到其他接口地址，可以覆盖 `WEB_API_ORIGIN`：

```bash
make web-dev WEB_API_ORIGIN=http://127.0.0.1:8080
```

当前默认包管理器为 `yarn`。

## 推荐目录

```text
web/
  index.html
  package.json
  tsconfig.json
  tsconfig.node.json
  vite.config.ts
  src/
    app/
      App.tsx
      router.tsx
      providers/
        AppProviders.tsx
        AuthProvider.tsx
        TenantProvider.tsx
        AppConfigProvider.tsx
    components/
      layout/
        Shell.tsx
    pages/
      dashboard/
        DashboardPage.tsx
        dashboard.ts
    styles/
      global.css
    main.tsx
```

## 目录职责

- `src/app`
  放应用级入口能力，包括路由、全局 provider、应用壳层装配。
- `src/app/providers`
  放全局上下文，不把鉴权、租户、前端配置直接散落到页面中。
- `src/pages`
  放路由页面和页面级数据模型。当前 dashboard 已经迁入 React，并在同目录维护页面状态和格式化工具。
- `src/components`
  放跨页面复用的视图组件。先从布局组件开始，不要一开始就抽太细。
- `src/styles`
  放全局样式和设计 token。

## 当前建议的落地方式

### 阶段 1：先把 Web 工程边界落地

- 用 `Vite + React + TypeScript` 起前端项目。
- 保持前端仍在当前仓库内，避免接口频繁变化时跨仓协作成本过高。
- 先把单页面应用跑起来，再逐步替换现有 Go embed 页面。

### 阶段 2：先重写展示层，不改后端边界

- 第一批已经接入 dashboard 展示页面，聚焦核心指标和窗口信息。
- 直接复用现有 `master-api` 接口：
  - `GET /master/api/v1/hosts`
  - `GET /master/api/v1/hosts/{host_uid}`
  - `GET /master/api/v1/stream/hosts`
- 当前实现已经把实时看板迁到 React SPA，并保留 SSE 增量刷新。

### 阶段 3：再接鉴权、租户和配置

- `AuthProvider` 承载登录态、权限判断、401/403 处理、请求头注入。
- `TenantProvider` 承载当前租户、租户切换、tenant scope 下的数据刷新。
- `AppConfigProvider` 承载前端配置、用户偏好、表格列显示、默认筛选条件等。

### 阶段 4：逐步替换旧页面入口

- 当 React 版 dashboard 稳定后，再考虑把 `/` 指向新的构建产物。
- 在替换之前，保留当前 embed 页作为回退方案。

## 为什么先同仓

- 当前前后端接口仍会快速变化。
- 鉴权、租户、配置能力会强耦合 API 演进。
- 同仓更适合在一个改动里同时修改 API、权限语义和前端页面。

## 后续建议

- 后续增加 `src/lib/http.ts` 统一封装 API client。
- 当页面数变多时，再补 `entities/`、`features/` 等更细分层，不要一开始过度设计。
