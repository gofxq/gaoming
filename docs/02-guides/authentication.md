# 登录与管理员引导

本地用户名在空间内唯一，字母不区分大小写；密码要求 12～72 字节，bcrypt cost 12。用户属于一个空间；管理员和成员仍受租户权限边界约束。普通账号创建应使用独立强密码。

统一后端会在用户表为空时自动初始化管理员；已有账号保持不变。

## 启动初始化

统一入口 `server/cmd/server` 在开始监听前检查整个用户表。若为空，创建用户名 `admin`、角色 `admin`、租户 `default` 的账号；密码读取 `GAOMING_ADMIN_PASSWORD`，未设置或为空时使用 `G@oming`。日志不打印密码，数据库只存 bcrypt 哈希。

已有任意用户时不创建、不重置账号；并发启动只创建一次。删除所有用户后，下次启动会重新初始化，即使网页 setup 完成记录仍存在。启动初始化单独允许 1～72 字节密码以兼容指定的短默认值；普通网页创建的 12～72 字节规则保持不变。

首次初始化会同时写入 `auth_bootstrap_state`，之后网页 setup 关闭。原有网页协议和 `setup_token` 配置保留；当前统一服务的登录入口为 `/<tenantCode>/login`。环境变量改变不能修改已有账号密码。

默认密码公开且较短，在可被其他设备访问的部署中，应在首次启动前设置 `GAOMING_ADMIN_PASSWORD` 为独立强密码。不要将真实密码写入命令行参数、日志、URL 或仓库。

## HTTPS、代理与会话

```yaml
session_cookie_name: gaoming_session
session_cookie_secure: true
session_ttl_hours: 168
allowed_origins:
  - https://monitor.example.com
```

Cookie 为 HttpOnly、SameSite=Lax，默认 Secure；本地纯 HTTP 联调必须显式设置 `session_cookie_secure: false`，生产保持 true。TLS 可在可信反代终止，但 `allowed_origins` 必须填浏览器访问的准确 HTTPS origin，不接受路径、通配符、凭据或 `null`。未配置列表时仅允许直连请求本身的 scheme+Host，忽略 Forwarded/X-Forwarded-*，不自动扩大代理信任。

会话 token 为 32 字节随机值，数据库只存 SHA-256；登录替换当前浏览器原会话，退出删除会话，过期（含到期时刻）、禁用用户或停用租户不能继续使用。`session_secret` 是兼容旧配置的未使用字段，不表示已开启签名会话。

所有会话写操作必须携带合法 Origin 和从 `GET /master/api/v1/auth/csrf` 获取的 `X-CSRF-Token`；登录/初始化必须携带合法 Origin 和 JSON body。CSRF 由会话秘密派生，不暴露原始会话 token。客户端凭据不可缓存到 localStorage；认证响应 `no-store`。

登录/初始化每个直接 TCP 对端每分钟最多 20 次，最多 4 个并行认证请求，最多保留 1024 个活跃限流身份。超额返回 429 和 Retry-After；不使用任意转发头作为限流身份。反代后的请求会共享代理对端额度，后续 P0-06 将补充可信代理与多实例配额策略。

## 验收与剩余安全工作

Go 回归覆盖一次性并发 setup、错误账号统一拒绝、禁用/到期/退出、密码字节边界、Origin/CSRF/Cookie 和认证限流。前端包含安全回跳回归：外部地址、反斜线、编码歧义、跨空间目标和登录/初始化循环均回退到自身空间。

T03 已接通私有读取、受限分享及运维租户绑定，规则见[访问与分享](./access-and-sharing.md)。Agent/worker 鉴权及完整接入/TLS 边界仍待 T04，完成前禁止将环境作为安全私有多租户服务发布。TLS 与完整接入安全将在 P0-07 联合验收。
