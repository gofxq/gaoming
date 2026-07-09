# Ionic React 移动端集成说明

`h5/` 独立手机端项目已经合并进 `web/`。仓库现在只保留一个前端应用：

- `web/` 承载桌面 Dashboard、管理页、登录页和移动端 PWA 页面。
- 移动端页面继续使用 Ionic React 组件。
- 移动端页面复用 `web` 的配置、租户、认证上下文、路由和实时数据逻辑。
- 不再维护独立 `h5` 构建、端口、Dockerfile 或 Compose 服务。

## 路由

移动端入口仍兼容原有路径：

- `/:tenantCode/pwa`
- `/:tenantCode/pwa/:hostUID`
- `/:tenantCode/mobile` 重定向到 `/:tenantCode/pwa`

桌面端窄屏或移动设备访问 `/:tenantCode` 时，会自动进入 `/:tenantCode/pwa`。

## 代码位置

移动端 UI：

```text
web/src/pages/mobile/MobileAgentPage.tsx
web/src/styles/mobile-ionic.css
web/src/styles/ionic-theme.css
```

复用的数据与格式化逻辑：

```text
web/src/pages/dashboard/dashboard.ts
web/src/pages/dashboard/useLiveHostsData.ts
```

应用级上下文：

```text
web/src/app/providers/AppConfigProvider.tsx
web/src/app/providers/TenantProvider.tsx
web/src/app/providers/AuthProvider.tsx
```

## 本地开发

只需要启动 `web`：

```bash
make web-local
```

或：

```bash
cd web
yarn dev
```

## 验证

移动端相关改动至少验证：

```bash
cd web
yarn typecheck
yarn build
```

浏览器验证：

- 打开 `/default/pwa`
- 打开 `/default/pwa/:hostUID`
- 验证无主机数据空状态
- 验证有主机数据时列表、详情、指标窗口和下拉刷新正常
- 验证 SSE 状态正常切换
