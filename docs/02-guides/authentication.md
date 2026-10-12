# 登录与管理员引导

本地用户名在空间内唯一，字母不区分大小写；密码要求 12～72 字节，bcrypt cost 12。用户属于一个空间；管理员和成员仍受租户权限边界约束。普通账号创建应使用独立强密码。

后端启动不创建账号，也不提供默认密码。初始管理员仅由下面的网页 setup 创建；已有账号保持不变。

## 首次安装

先在空数据库执行 `deployments/sql/init.sql`。`make dev-init` 为新配置自动生成随机 `setup_token`，已有配置不修改。由部署管理员通过秘密管理系统设置随机 `GAOMING_SETUP_TOKEN`（至少 32 字节），或在权限 0600 的 `config/server.yml`中设置 `setup_token`。不要将 token 放入 URL、命令行参数、日志或仓库。

访问 `/<tenantCode>/login`，初始化可用时可进入 `/<tenantCode>/setup`，输入引导凭据、用户名、显示名称和密码。仅当整个环境尚无用户、且引导未完成时允许初始化；并发初始化只能成功一次。数据库持久记录引导完成，用户删除或服务重启不会重新开放。完成后撤销配置中的引导秘密。

已有用户的环境通过页面管理账号，或由可信数据库管理员直接维护。密码字段必须保存 bcrypt 哈希，不能写入明文；租户、用户身份和会话关系须保持一致。

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
