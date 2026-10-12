# 后端目录与部署收拢

> 后续调整：用户要求移除全部离线管理员与 Go 迁移逻辑。当前只保留 `server/cmd/server`；数据库由用户直接管理，见 `docs/02-guides/database-setup.md`。


用户要求将后端彻底收拢为一个服务，以顶层 `server/` 替代 `services/`，同步更新 AGENTS.md。

唯一在线后端入口为 `server/cmd/server`，离线管理入口为 `server/cmd/admin`。统一 app/config 位于 `server/internal`；master、ingest、core、probe 是内部模块，不再保留独立 cmd/app 或 worker 配置加载器。保留现有认证、共享 PostgreSQL/Redis、HTTP `/master/*` 和 `/ingest/*`、gRPC 及后台任务行为。Agent 保持独立。

Docker Compose 使用一个 server 容器及 PostgreSQL/Redis、一次性迁移容器和可选 Agent/Web。配置示例置于 deployments，私有本机 config 不纳入版本管理。修正新源码及 AGENTS.md 的忽略规则。更新 Make、CI、开发/部署/smoke 脚本、现行文档；旧设计文件保留为历史记录。

保留工作区已有修改，不提交或覆盖用户本机配置。验证包含 Go 全量测试、构建、Compose 校验和隔离 Docker smoke；检查认证边界、配置挂载、容器权限及密钥泄露风险。
