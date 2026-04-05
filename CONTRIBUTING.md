# Contributing

## Development Loop

Use the repository Make targets instead of ad hoc commands where possible:

```bash
make fmt
make test
make build
make check
```

For the full local stack:

```bash
make docker-up
make smoke
make docker-logs
make docker-down
```

## Repository Conventions

- Keep the project ASCII-first unless an existing file already uses Unicode.
- Use `go fmt` on all Go changes.
- Prefer extending the existing `pkg/` and service structure instead of inventing parallel layouts.
- Keep README high level and move detailed design or runbook content into `docs/`.
- Update `deployments/sql/init.sql` when schema changes affect the bootstrap flow.

## Before Opening a PR

Run:

```bash
make check
make compose-config
```

If your change affects the local stack behavior, also run:

```bash
make docker-up
make smoke
make docker-down
```
