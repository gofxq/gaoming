GO ?= go
DOCKER_COMPOSE ?= docker compose
YARN ?= yarn
WEB_API_ORIGIN ?= https://gm-metric.gofxq.com

.PHONY: fmt build test check clean
.PHONY: master ingest core probe agent
.PHONY: proto-check proto-lint
.PHONY: compose-config up up-full down docker-up docker-up-full docker-down docker-logs docker-ps
.PHONY: smoke smoke-agent
.PHONY: deploy-agent-service install-agent-local-service
.PHONY: web-install web-dev web-local web-build web-typecheck web-preview

fmt:
	$(GO) fmt ./...

build:
	$(GO) build ./...

test:
	$(GO) test ./...

check: fmt test build proto-check

clean:
	rm -rf bin dist tmp .tmp coverage cover.out web/dist

master:
	$(GO) run ./services/master-api/cmd/server

ingest:
	$(GO) run ./services/ingest-gateway/cmd/server

core:
	$(GO) run ./services/core-worker/cmd/worker

probe:
	$(GO) run ./services/probe-worker/cmd/worker

agent:
	$(GO) run ./agent/daemon/cmd/agent

proto-check:
	protoc -I api/proto \
		--descriptor_set_out=/tmp/gaoming-proto.pb \
		api/proto/monitor/v1/common.proto \
		api/proto/monitor/v1/agent.proto \
		api/proto/monitor/v1/probe.proto \
		api/proto/monitor/v1/query.proto \
		api/proto/monitor/v1/ops.proto

proto-lint:
	buf lint api/proto

compose-config:
	$(DOCKER_COMPOSE) config >/dev/null

up:
	$(DOCKER_COMPOSE) --profile web up -d --build

up-full:
	$(DOCKER_COMPOSE) --profile container-agent --profile web up -d --build

down:
	$(DOCKER_COMPOSE) down --remove-orphans

docker-logs:
	$(DOCKER_COMPOSE) logs -f --tail=200

docker-ps:
	$(DOCKER_COMPOSE) ps

smoke:
	sh ./scripts/smoke-backend.sh

smoke-agent:
	sh ./scripts/smoke-agent.sh

deploy-agent-service:
	sh ./deployments/service-deploy-sh.sh

install-agent-local-service:
	mkdir -p .tmp
	$(GO) build -o .tmp/gaoming-agent ./agent/daemon/cmd/agent
	sh ./deployments/install-agent-local.sh --bin ./.tmp/gaoming-agent

web-install:
	cd web && $(YARN) install

web-dev: web-install
	cd web && VITE_PROXY_TARGET=$(WEB_API_ORIGIN) $(YARN) dev

web-local: web-install
	cd web && VITE_PROXY_TARGET=http://localhost:8080 $(YARN) dev

web-build:
	cd web && VITE_API_ORIGIN=$(WEB_API_ORIGIN) $(YARN) build

web-typecheck:
	cd web && $(YARN) typecheck

web-preview:
	cd web && $(YARN) preview
