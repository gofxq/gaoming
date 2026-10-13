# Server + Web Container Plan

用户要求将启动入口统一到 server/main.go，并将 Web 与 Server 打包到同一 Docker 容器以简化 Compose。

- [x] 更新 Make、Air、Agent 构建/安装、CI 和 Docker 入口至 ./server 与 ./agent，保留用户已有入口移动。
- [x] 为 Go HTTP mux 增加可选静态目录（web_dir / GAOMING_WEB_DIR），为 SPA 页面提供 index fallback，API 优先且未知静态资源 404；限制目录穿越、隐藏文件及目录列表。
- [x] 多阶段 Docker 构建 Web dist 和 Go server，运行时仅保留 Go、静态资源及非 root 用户；Agent 使用不含 Web 的独立 target。
- [x] Compose 删除 Web 服务和 web profile/5173 映射；默认仅 PostgreSQL、Redis、Server。
- [x] 同步现行文档与 smoke，运行 Go、Web 构建、Compose 校验及隔离 Docker 联调，独立审阅安全边界。

当前分支已恢复启动管理员初始化，保持该行为；原 setup 联调脚本需同步为启动账号登录。历史设计文档不作为当前部署入口说明。

验证：静态服务边界测试先因缺失实现失败，实现后通过。make test/build 在普通环境和真实隔离 PostgreSQL/Redis 环境通过；Web 类型/生产构建、Compose 校验与 shell 语法检查通过。合并容器用缓存 Go 依赖和本机构建 Web 产物完成运行验证：主页/深层路由/JS/CSS、404/隐藏文件、匿名拒绝、登录、Agent 上报及 probe。标准多阶段构建的 Node 镜像下载过慢，主动取消本次隔离构建并由脚本清理；生产 Dockerfile 未增加环境专用配置。独立审阅发现 Agent target 与旧 WEB_PORT 说明，均已修正。
