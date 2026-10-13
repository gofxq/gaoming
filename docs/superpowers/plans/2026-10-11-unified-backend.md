# Unified Backend Implementation Plan

> 历史记录：目录和独立入口保留策略已被 [2026-10-12 后端收拢设计](../specs/2026-10-12-server-consolidation.md) 替代；当前入口为 `server/cmd/server`。


> **For agentic workers:** Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Run all four business backends in one native process and make local development independent of Docker.
**Architecture:** Public per-service runtime adapters compose existing internal modules against shared PostgreSQL/Redis. The unified server owns listeners, workers and bounded shutdown; Make targets and local scripts own frontend process supervision.
**Tech Stack:** Existing Go, Gin, gRPC, pgx/GORM, Redis and Vite; no new application dependencies.
**Spec:** docs/superpowers/specs/2026-10-11-unified-backend.md

## Global Constraints

- Preserve existing APIs, independent binaries, authentication and tenant boundaries.
- PostgreSQL and Redis remain native external dependencies; Agent remains separate.
- Listen on 0.0.0.0, retain database data and avoid default login passwords.
- Existing pending Makefile/documentation changes are part of the continuing task; do not discard them.

## Review Focus

- Partial listener startup must close acquired listeners and shared storage.
- Persistent gRPC streams and SSE requests must not block shutdown.
- Local startup must fail when a child fails, and stop only owned processes.
- Custom ports must reach server, proxy, probes and smoke together.
- Missing native dependencies/configuration must produce actionable failures.

### Task 1: Shared composition and unified server

Files: create services/{master-api,ingest-gateway,core-worker,probe-worker}/runtime/runtime.go; create services/server/{cmd/server,internal/config,internal/app}.
Interfaces: master runtime exposes Config/LoadConfig and NewHandler; ingest runtime exposes Module with HTTP, GRPC and RunReconciler; worker adapters expose Run(ctx). Unified config Load(path) supplies HTTP, gRPC, worker intervals and storage/auth config; App.New(ctx,cfg), Run(ctx) own shared resource lifecycle.
- [x] Write config and lifecycle tests, run and observe missing implementation failures.
- [x] Implement runtime adapters, config and app with shared storage and bounded shutdown.
- [x] Verify defaults, bad addresses/intervals, route composition, cancellation and bind cleanup.

### Task 2: Native development workflow

Files: Makefile, scripts/dev-local.sh, scripts/dev-air.sh, deployments/server-config.local.yaml, docs/02-guides/local-development.md, README.md.
Interfaces: SERVER_CONFIG points to config/server.yml; HTTP_PORT/GRPC_PORT override native listeners. Existing Docker port variables remain scoped to Docker. Native smoke uses same HTTP port for both paths.
- [x] Implement native init/migration/backend/dev/hot reload and explicit Docker targets.
- [x] Verify Make dry runs and shell syntax; verify child failure and cleanup by running local launcher.
- [x] Update native dependency/setup/login instructions; remove stale default-Docker guidance.

### Task 3: Native integration and review

Files: services/server/internal/app/app_integration_test.go, scripts/smoke-local.sh if needed.
Interfaces: TEST_POSTGRES_DSN and TEST_REDIS_ADDR supply native fixtures; tests own temporary schemas/cache namespaces.
- [x] Write integration assertions for both HTTP modules, gRPC metrics, authenticated reads, probe counters and bounded cancellation.
- [x] Run native integration, make test, make build, make web-build and diff checks.
- [x] Request a fresh whole-change review, fix material findings and record validation and security limitations.
