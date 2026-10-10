GO ?= go
DOCKER_COMPOSE ?= docker compose
YARN ?= yarn
AIR ?= air
WEB_API_ORIGIN ?= https://gm-metric.gofxq.com

.PHONY: fmt build test check clean air
.PHONY: master ingest core probe agent
.PHONY: proto-check proto-lint
.PHONY: compose-config up up-full down docker-up docker-up-full docker-down docker-logs docker-ps
.PHONY: smoke smoke-agent migrate schema-check migration-test fmt-check proto-generate proto-generated-check web-test web-e2e smoke-roadmap
.PHONY: deploy-agent-service install-agent-local-service
.PHONY: web-install web-dev web-local web-build web-typecheck web-preview

fmt:
	$(GO) fmt ./...

fmt-check:
	@test -z "$$(gofmt -l $$(git ls-files '*.go') $$(git ls-files --others --exclude-standard '*.go'))" || { echo 'Go files require gofmt'; exit 1; }

migrate:
	$(GO) run ./services/master-api/cmd/admin migrate

schema-check:
	$(GO) run ./services/master-api/cmd/admin schema-check

migration-test:
	REQUIRE_POSTGRES_TESTS=1 $(GO) test ./pkg/migrations -count=1 -v

build:
	$(GO) build ./...

test:
	$(GO) test ./...

check: fmt-check test build proto-check proto-generated-check web-typecheck web-test web-build

clean:
	rm -rf bin dist tmp .tmp coverage cover.out web/dist

air: web-install
	GO="$(GO)" AIR="$(AIR)" YARN="$(YARN)" sh ./scripts/dev-air.sh

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
		api/proto/monitor/v1/monitor.proto

proto-lint:
	buf lint api/proto

proto-generate:
	protoc -I api/proto --go_out=api/gen/go --go_opt=paths=source_relative --go-grpc_out=api/gen/go --go-grpc_opt=paths=source_relative api/proto/monitor/v1/monitor.proto

proto-generated-check:
	sh ./scripts/check-proto-generated.sh

compose-config:
	$(DOCKER_COMPOSE) config >/dev/null

up:
	$(DOCKER_COMPOSE) --profile web up -d --build

up-full:
	$(DOCKER_COMPOSE) --profile container-agent --profile web up -d --build

docker-up: up

docker-up-full: up-full

docker-down: down

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

web-test:
	cd web && $(YARN) test

web-e2e:
	cd web && $(YARN) test:e2e

smoke-roadmap:
	sh ./scripts/smoke-roadmap.sh

web-preview:
	cd web && $(YARN) preview
