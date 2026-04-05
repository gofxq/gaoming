GO ?= go
DOCKER_COMPOSE ?= docker compose

.PHONY: fmt build test check clean run-master run-ingest run-core run-probe run-agent proto-check proto-lint compose-config docker-up docker-up-full docker-down docker-logs docker-ps smoke smoke-agent

fmt:
	$(GO) fmt ./...

build:
	$(GO) build ./...

test:
	$(GO) test ./...

check: fmt test build proto-check

clean:
	rm -rf bin dist tmp .tmp coverage cover.out

run-master:
	$(GO) run ./services/master-api/cmd/server

run-ingest:
	$(GO) run ./services/ingest-gateway/cmd/server

run-core:
	$(GO) run ./services/core-worker/cmd/worker

run-probe:
	$(GO) run ./services/probe-worker/cmd/worker

run-agent:
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

docker-up:
	$(DOCKER_COMPOSE) up -d --build

docker-up-full:
	$(DOCKER_COMPOSE) --profile container-agent up -d --build

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
