# Web

这个目录用于承载 `Gaoming` 的同仓前端项目。

现代皮肤和全局组件规则见 [DESIGN_SYSTEM.md](./DESIGN_SYSTEM.md)，像素皮肤规范见 [PIXEL_DESIGN_SYSTEM.md](./PIXEL_DESIGN_SYSTEM.md)。

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

## Docker 联调

如果要直接看整套项目的启动效果，可以在仓库根目录执行：

```bash
make docker-up-full
```

它会启动后端服务、容器化 `agent` 和 `web` 开发容器。`web` 容器内部直接运行 `yarn dev`，并通过 Vite 代理把 `/master/*` 转发到 Compose 网络里的 `master-api`。启动完成后直接访问 `http://localhost:5173`。

## 推荐目录

```text
src/
  app/                         # 应用入口、路由和 Provider 装配
  pc/
    pages/                     # PC 页面和 Semi UI 壳层
    components/pixel/          # PC 专属 Pixel 展示覆写
  h5/
    pages/                     # IonPage / IonContent 页面
    components/pixel/          # H5 专属 Pixel 展示覆写
  shared/
    features/                  # API、状态、表单和 payload
    lib/                       # HTTP、Query Client 等基础设施
    styles/themes/pixel/       # Pixel token、字体和基础材质
  styles/global.css            # 现代皮肤 token 和全局样式
  main.tsx
```

## 目录职责

- `src/app`
  放应用入口、路由和全局 Provider 装配，不承载平台专属展示。
- `src/pc`、`src/h5`
  放各端页面、布局和必要的 Pixel 展示变体；不得复制共享业务状态。
- `src/shared/features`
  放鉴权、租户、配置、外观状态和主机实时数据等跨端能力。
- `src/shared/lib`
  放与具体页面无关的 HTTP 和数据查询基础设施。
- `src/shared/styles/themes/pixel`
  放跨端 Pixel 字体、语义 token 和基础材质。
- `src/styles`
  放现代皮肤 token 和当前应用的全局样式。

## 当前建议的落地方式

### 阶段 1：先把 Web 工程边界落地

- 用 `Vite + React + TypeScript` 起前端项目。
- 保持前端仍在当前仓库内，避免接口频繁变化时跨仓协作成本过高。
- 先把单页面应用跑起来，并逐步接入正式交付链路。

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
- 旧的 Go embed 页面已移除，后续只保留 React SPA 这一套前端入口。

## 为什么先同仓

- 当前前后端接口仍会快速变化。
- 鉴权、租户、配置能力会强耦合 API 演进。
- 同仓更适合在一个改动里同时修改 API、权限语义和前端页面。

## 后续建议

- API、状态和 payload 优先沉淀到 `src/shared/features`，避免 PC/H5 重复实现。
- 只有 CSS 无法表达展示差异时，才在各端增加 Pixel 组件变体。
