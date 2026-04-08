# 仓库工程化补充

当前仓库已经补齐了基础工程文件：

- [`.gitignore`](../.gitignore)
- [`.editorconfig`](../.editorconfig)
- [`.dockerignore`](../.dockerignore)
- [`.gitattributes`](../.gitattributes)
- [`CONTRIBUTING.md`](../CONTRIBUTING.md)

## 作用

- `.gitignore`
  - 忽略编译产物、日志、覆盖率文件、本地环境文件和 IDE 垃圾文件
- `.editorconfig`
  - 统一缩进、换行和文件结尾
- `.dockerignore`
  - 控制镜像构建上下文
- `.gitattributes`
  - 统一文本文件换行策略
- `CONTRIBUTING.md`
  - 固定开发、校验和提交流程

## 当前仓库习惯

- Go 侧优先用 `make check`
- 涉及容器改动时补跑 `make compose-config`
- 涉及联调链路时补跑 `make smoke` 和 `make smoke-agent`
- Web 变更走 `make web-dev` / `make web-build`
- 设计和运行态说明继续沉淀在 `docs/`，前端专项说明放 `web/README.md`
