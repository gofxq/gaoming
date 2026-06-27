# H5 手机端 PWA 前端计划

这份文档描述后续如何在仓库中新建独立 `h5/` 手机端前端项目，并使用 Ionic React 构建 PWA。本文档只定义实施计划，不代表已经开始实现。

## 背景

当前仓库已有 `web/` 前端：

- `web/` 是 `Vite + React + TypeScript` SPA。
- `web/` 同时包含桌面 Dashboard 和现有移动端 PWA 风格页面。
- 现有移动端页面主要靠自写 JSX 和 `global.css` 模拟手机界面。
- `web/` 已接入主机列表、主机详情、SSE 实时数据和指标窗口。

新的方向是：不再把 Ionic React 迁移进现有 `web/`，而是新建独立 `h5/` 目录，作为手机端专用前端项目。`web/` 继续承载桌面端和现有页面，`h5/` 专注移动端和 PWA。

## 目标

- 新建 `h5/` 手机端专用前端项目。
- 使用 `Vite + React + TypeScript + Ionic React`。
- 支持 PWA 安装、离线壳层、manifest、service worker 和移动端图标。
- 复用当前后端接口和 SSE 数据流。
- 保持租户路径和 host 详情路径可表达。
- 不改后端接口、不改 agent 和服务运行链路。
- 不在本阶段改造 `web/` 既有桌面端实现。

## 非目标

- 不把 `web/` 迁移到 Ionic React。
- 不删除 `web/src/pages/mobile`。
- 不合并桌面端和手机端页面。
- 不引入新的后端 API。
- 不实现完整登录、权限和多租户管理后台。
- 不做原生 App 打包，当前只做浏览器 H5/PWA。

## 目标目录结构

建议新增目录：

```text
h5/
  index.html
  package.json
  tsconfig.json
  tsconfig.node.json
  vite.config.ts
  public/
    manifest.webmanifest
    service-worker.js
    icons/
      icon-192.png
      icon-512.png
      apple-touch-icon.png
  src/
    main.tsx
    app/
      App.tsx
      router.tsx
      providers/
        AppConfigProvider.tsx
        TenantProvider.tsx
    pages/
      HostListPage.tsx
      HostDetailPage.tsx
    components/
      HostCard.tsx
      MetricCard.tsx
      MetricSparkline.tsx
      StatusBadge.tsx
    data/
      dashboard.ts
      useLiveHostsData.ts
    styles/
      ionic-theme.css
      global.css
```

说明：

- `h5/src/data/dashboard.ts` 可以先复制并精简 `web/src/pages/dashboard/dashboard.ts` 中的纯逻辑。
- `h5/src/data/useLiveHostsData.ts` 可以先复用现有 SSE 和 hosts fetch 逻辑。
- `h5/` 拥有自己的依赖、构建脚本和 PWA 资源。
- 后续如果 `web/` 和 `h5/` 需要共享逻辑，再抽到共享包；首版先避免过度抽象。

## 依赖计划

`h5/package.json` 建议使用：

- `react`
- `react-dom`
- `@ionic/react`
- `@ionic/react-router`
- `ionicons`
- `react-router`
- `react-router-dom`
- `@vitejs/plugin-react`
- `typescript`
- `vite`

Node 版本：

- 使用仓库统一 Node 24。
- `h5/package.json` 声明 `engines.node: "24.x"`。

包管理：

- 与现有前端保持一致，默认使用 `yarn@1.22.22`。
- `h5/` 生成独立 `yarn.lock`，不与 `web/yarn.lock` 混用。

## 路由设计

手机端首版保留最小路由：

- `/`
  - 重定向到 `/default`
- `/:tenantCode`
  - 手机端主机列表
- `/:tenantCode/hosts/:hostUID`
  - 手机端单主机详情

兼容入口：

- `/:tenantCode/pwa`
  - 重定向到 `/:tenantCode`
- `/:tenantCode/pwa/:hostUID`
  - 重定向到 `/:tenantCode/hosts/:hostUID`

这样可以兼容当前移动端路径，同时让新 `h5/` 项目拥有更清晰的手机端 URL。

## PWA 设计

`h5/` 需要包含：

- `public/manifest.webmanifest`
  - `name`: `Gaoming H5`
  - `short_name`: `Gaoming`
  - `display`: `standalone`
  - `start_url`: `/default`
  - `scope`: `/`
  - `theme_color`: 与 Ionic 主题一致
  - `background_color`: 与页面背景一致
- `public/service-worker.js`
  - 缓存应用壳层资源。
  - 网络优先加载 API 和 SSE，不缓存实时接口响应。
  - 新版本激活时清理旧缓存。
- `index.html`
  - 引入 manifest。
  - 引入 apple touch icon。
  - 设置移动端 viewport。
- `src/main.tsx`
  - 注册 service worker。
  - 使用 `IonApp` 包裹应用。

## 页面能力

### 主机列表页

使用 Ionic 组件组织：

- `IonPage`
- `IonHeader`
- `IonToolbar`
- `IonTitle`
- `IonContent`
- `IonRefresher`
- `IonList`
- `IonCard`
- `IonBadge`
- `IonChip`

展示内容：

- 当前租户。
- SSE 连接状态。
- 主机总数、在线数、平均 CPU、平均内存。
- 主机列表卡片。
- 每台主机展示 hostname、IP、状态、CPU、内存、磁盘、最后心跳。

### 主机详情页

使用 Ionic 组件组织：

- `IonBackButton`
- `IonSegment`
- `IonCard`
- `IonGrid`
- `IonProgressBar`

展示内容：

- 主机名称、IP、状态。
- 最近心跳、最近指标时间、版本。
- CPU、内存、磁盘、负载、网络吞吐、磁盘吞吐。
- 1 分钟 / 5 分钟 / 15 分钟 / 1 小时窗口切换。
- 轻量 sparkline 趋势图。

## 数据层策略

保留当前后端契约：

- `GET /master/api/v1/hosts?tenant={tenantCode}`
- `GET /master/api/v1/stream/hosts?tenant={tenantCode}`

`h5/` 的配置读取：

- 默认 API base：`/master/api/v1`
- 默认 stream path：`/master/api/v1/stream/hosts`
- 支持 `VITE_API_ORIGIN`
- 支持 `VITE_API_BASE_URL`
- 支持 `VITE_STREAM_PATH`

实时数据策略：

- 首次进入页面先 fetch 当前主机列表。
- 随后通过 SSE 接收 `sync`、`host_upsert`、`host_delete`。
- SSE 断开时展示“重连中”状态。
- 数据逻辑先保留在 `h5/src/data`，避免直接依赖 `web/src`。

## 与现有 `web/` 的关系

首版不修改 `web/`：

- `web/` 继续保留现有桌面 Dashboard。
- `web/` 的现有移动页暂不删除。
- `h5/` 独立构建、独立开发、独立 PWA。

后续稳定后再评估：

- 是否让移动端入口统一指向 `h5/`。
- 是否删除 `web/src/pages/mobile`。
- 是否把 `dashboard.ts` 和 `useLiveHostsData` 抽到共享包。

## 本地开发

建议新增命令：

```bash
cd h5
yarn install
yarn dev
```

建议端口：

- `h5` 开发服务器使用 `5174`。
- `web` 保持 `5173`。

Vite 代理：

- 将 `/master/*` 代理到后端。
- 默认代理目标与 `web/` 保持一致：`http://localhost:8080/`。

## Docker 与部署

首版建议新增独立 Dockerfile：

```text
deployments/docker/h5.Dockerfile
```

职责：

- 使用 Node 24 镜像。
- 安装 `h5/` 依赖。
- 运行 `yarn dev` 用于本地 Compose 联调，或运行 `yarn build` 用于静态产物部署。

Compose 后续可新增 `h5` 服务：

- 端口 `5174:5174`
- 代理 `/master/*` 到 `master-api`

## 验证清单

实现后至少验证：

```bash
cd h5
yarn typecheck
yarn build
```

浏览器验证：

- 打开 `/default`
- 打开 `/default/hosts/:hostUID`
- 打开兼容路径 `/default/pwa`
- 打开兼容路径 `/default/pwa/:hostUID`
- 验证无主机数据空状态。
- 验证有主机数据时列表、详情、指标窗口正常。
- 验证 SSE 状态从连接中到实时推送中。
- 验证断网或后端不可用时页面不崩溃。
- 验证 Lighthouse PWA 基础项。
- 验证添加到主屏幕后以 standalone 模式打开。

## 实施顺序

1. 新建 `h5/` Vite React TypeScript 项目。
2. 安装 Ionic React、Ionic Router、Ionicons 和 PWA 所需依赖。
3. 配置 Ionic 基础 CSS、主题变量和 `IonApp`。
4. 配置 H5 路由和兼容重定向。
5. 复制并精简主机数据模型、格式化函数和 SSE hook。
6. 实现主机列表页。
7. 实现主机详情页。
8. 增加 manifest、service worker、图标和注册逻辑。
9. 配置 Vite 代理和构建脚本。
10. 运行 typecheck、build 和浏览器 PWA 验证。

## 完成定义

完成后应满足：

- 仓库存在独立 `h5/` 手机端项目。
- `h5/` 使用 Ionic React 作为主要 UI 组件体系。
- `h5/` 支持 PWA manifest 和 service worker。
- `h5/` 可通过现有后端 API 和 SSE 展示实时主机数据。
- `web/` 不因本次工作被迁移或破坏。
- `h5` 的 `typecheck` 和 `build` 通过。
