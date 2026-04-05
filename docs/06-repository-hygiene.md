# 仓库工程化补充

为了让项目更像一个可长期维护的仓库，而不是一次性样例，当前已经补上这些基础设施文件：

- [/.gitignore](/home/u/dev/github.com/gofxq/gaoming/.gitignore)
- [/.editorconfig](/home/u/dev/github.com/gofxq/gaoming/.editorconfig)
- [/.dockerignore](/home/u/dev/github.com/gofxq/gaoming/.dockerignore)
- [/.gitattributes](/home/u/dev/github.com/gofxq/gaoming/.gitattributes)
- [/CONTRIBUTING.md](/home/u/dev/github.com/gofxq/gaoming/CONTRIBUTING.md)

## 作用

- `.gitignore`：忽略编译产物、日志、覆盖率文件、本地环境文件和 IDE 垃圾文件。
- `.editorconfig`：统一缩进、换行和结尾换行策略。
- `.dockerignore`：减少 Docker 构建上下文，避免把无关文件送进镜像构建。
- `.gitattributes`：统一文本文件的 LF 换行。
- `CONTRIBUTING.md`：固定开发、校验和提交流程。

## 推荐习惯

- 日常开发优先执行 `make check`。
- 改了容器或启动链路时，补跑 `make compose-config` 和 `make smoke`。
- 新增详细设计时写进 `docs/`，不要把 README 再膨胀回大杂烩。
