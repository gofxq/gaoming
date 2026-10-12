GO ?= go
YARN ?= yarn
AIR ?= air
DOCKER_COMPOSE ?= docker compose
SERVER_CONFIG ?= config/server.yml
DEPS_DIR ?= .local-data
HTTP_PORT ?= 8080
GRPC_PORT ?= 8091
WEB_PORT ?= 5173
WEB_PROXY_TARGET ?= http://127.0.0.1:$(HTTP_PORT)
WEB_API_ORIGIN ?= https://gm-metric.gofxq.com
MASTER_URL ?= http://127.0.0.1:$(HTTP_PORT)/master
INGEST_URL ?= http://127.0.0.1:$(HTTP_PORT)/ingest
DOCKER_PROFILES ?= --profile web

LOCAL_DEPS = $(GO) run ./cmd/localdeps -config "$(SERVER_CONFIG)" -dir "$(DEPS_DIR)" -docker-compose "$(DOCKER_COMPOSE)"
export GO AIR SERVER_CONFIG HTTP_PORT GRPC_PORT WEB_PORT
LOCAL_ENV = GAOMING_HTTP_ADDR="0.0.0.0:$(HTTP_PORT)" GAOMING_GRPC_ADDR="0.0.0.0:$(GRPC_PORT)"

.PHONY: help dev-init deps-up deps-down backend dev air run-agent
.PHONY: fmt fmt-check build test test-all check clean
.PHONY: web-install web-dev web-build web-test web-e2e web-preview
.PHONY: proto-check proto-lint proto-generate
.PHONY: compose-config docker-up docker-down docker-logs docker-ps docker-smoke
.PHONY: smoke smoke-agent install-agent-local-service

dev-init: ## 生成缺失配置和随机数据库/Redis 密码与 setup 凭据，保留已有配置
	$(LOCAL_DEPS) init

deps-up: dev-init ## 通过 Docker 启动 PostgreSQL 和 Redis（后端仍在本机运行）
	$(LOCAL_DEPS) up

deps-down: ## 停止依赖容器，保留 PostgreSQL 数据卷
	$(LOCAL_DEPS) down

backend: ## 只运行统一后端（master + ingest + probe）
	$(LOCAL_ENV) $(GO) run ./server/cmd/server -config "$(SERVER_CONFIG)"

dev: web-install ## 启动统一后端和前端，Ctrl-C 停止
	mkdir -p .tmp/local/bin
	$(GO) build -o .tmp/local/bin/server ./server/cmd/server
	$(LOCAL_ENV) node ./scripts/dev-local.mjs

air: web-install ## 热更新统一后端和前端（需安装 Air）
	$(LOCAL_ENV) sh ./scripts/dev-air.sh

run-agent: ## 运行宿主机 Agent（读取 agent-config.yaml）
	$(GO) run ./agent/daemon/cmd/agent

##@ 测试与构建
fmt: ## 格式化 Go 代码
	$(GO) fmt ./...

fmt-check: ## 检查 Go 格式，不修改文件
	@test -z "$$(git ls-files --cached --others --exclude-standard '*.go' | while IFS= read -r file; do if [ -f "$$file" ]; then gofmt -l "$$file"; fi; done)" || { echo 'Go files require gofmt'; exit 1; }

build: ## 构建全部 Go 包
	$(GO) build ./...

test: ## 运行全部 Go 测试
	$(GO) test ./...

test-all: test web-test ## 运行 Go 和前端单元测试（先 make web-install）

check: fmt-check test-all build proto-check web-build ## 完整检查（需前端依赖、protoc 和生成插件）

clean: ## 清理构建产物，保留依赖状态目录和 PostgreSQL 数据卷
	$(LOCAL_DEPS) clean

##@ 链路验证
smoke: ## 验证统一后端健康状态和接入计数器
	MASTER_URL="$(MASTER_URL)" INGEST_URL="$(INGEST_URL)" sh ./scripts/smoke-backend.sh

smoke-agent: ## 验证 Agent 上报（需要 SESSION_COOKIE_FILE）
	MASTER_URL="$(MASTER_URL)" INGEST_URL="$(INGEST_URL)" sh ./scripts/smoke-agent.sh

##@ 前端
web-install: ## 安装前端依赖
	cd web && $(YARN) install --frozen-lockfile

web-dev: web-install ## 只启动前端，代理 WEB_PROXY_TARGET（默认本机统一后端）
	cd web && VITE_PROXY_TARGET="$(WEB_PROXY_TARGET)" $(YARN) dev --host 0.0.0.0 --port $(WEB_PORT) --strictPort

web-build: ## 类型检查并构建前端（API 地址由 WEB_API_ORIGIN 指定）
	cd web && VITE_API_ORIGIN="$(WEB_API_ORIGIN)" $(YARN) build

web-test: ## 运行前端单元测试
	cd web && $(YARN) test

web-e2e: ## 运行 Playwright 测试
	cd web && $(YARN) test:e2e

web-preview: ## 预览前端构建产物
	cd web && $(YARN) preview --host 0.0.0.0

##@ 协议
proto-check: ## 校验协议定义及生成代码是否一致
	sh ./scripts/check-proto-generated.sh

proto-lint: ## 使用 buf 检查协议风格
	buf lint api/proto

proto-generate: ## 生成 Go protobuf 和 gRPC 代码
	protoc -I api/proto --go_out=api/gen/go --go_opt=paths=source_relative --go-grpc_out=api/gen/go --go-grpc_opt=paths=source_relative api/proto/monitor/v1/monitor.proto

##@ Docker 全栈部署（与 deps-up 的依赖容器独立）
compose-config: ## 校验 Compose 配置
	$(DOCKER_COMPOSE) config >/dev/null

docker-up: compose-config ## 启动容器；DOCKER_PROFILES 可增加 container-agent profile
	$(DOCKER_COMPOSE) $(DOCKER_PROFILES) up -d --build

docker-down: ## 停止所有 profile 容器，保留数据库卷
	$(DOCKER_COMPOSE) --profile '*' down --remove-orphans

docker-logs: ## 持续查看容器日志
	$(DOCKER_COMPOSE) --profile '*' logs -f --tail=200

docker-ps: ## 查看全部 profile 容器状态
	$(DOCKER_COMPOSE) --profile '*' ps

docker-smoke: ## 独立 Docker 集成验证，结束后清理临时资源
	sh ./scripts/smoke-roadmap.sh

##@ Agent 服务安装
install-agent-local-service: ## 构建并安装本机 Agent 服务
	mkdir -p .tmp
	$(GO) build -o .tmp/gaoming-agent ./agent/daemon/cmd/agent
	sh ./deployments/install-agent-local.sh --bin ./.tmp/gaoming-agent
