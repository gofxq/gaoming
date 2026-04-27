GO ?= go
DOCKER_COMPOSE ?= docker compose
YARN ?= yarn
WEB_API_ORIGIN ?= https://gm-metric.gofxq.com

.PHONY: fmt build test check clean master ingest core probe agent proto-check proto-lint compose-config up docker-up docker-up-full docker-down docker-logs docker-ps smoke smoke-agent deploy-agent-service install-agent-local-service db-update web-install web-dev web-local web-build h5-install h5-dev h5-local h5-build

fmt:
	$(GO) fmt ./...

build:
	$(GO) build ./...

test:
	$(GO) test ./...

check: fmt test build proto-check

clean:
	rm -rf bin dist tmp .tmp coverage cover.out

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

docker-up:
	$(DOCKER_COMPOSE) --profile web up -d --build

docker-up-full:
	$(DOCKER_COMPOSE) --profile container-agent --profile web --profile h5 up -d --build

docker-down:
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
	sh ./deployments/deploy-agent-service.sh

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

h5-install:
	cd h5 && $(YARN) install

h5-dev: h5-install
	cd h5 && VITE_PROXY_TARGET=$(WEB_API_ORIGIN) $(YARN) dev

h5-local: h5-install
	cd h5 && VITE_PROXY_TARGET=http://localhost:8080 $(YARN) dev

h5-build:
	cd h5 && VITE_API_ORIGIN=$(WEB_API_ORIGIN) $(YARN) build
