# Yunshu CallCenter Go Rewrite

Yunshu is the Go rewrite workspace for Yunshu CallCenter. It keeps the public contracts of the  system while introducing explicit domain boundaries for Gateway, OpenAPI, merchant console, operate admin, CTI, ESL, workers, contracts, infra, and observability.

## Services

- `cc-edge`: gateway plus external OpenAPI surface.
- `cc-console`: merchant console plus operate admin surface.
- `cc-call`: CTI and ESL realtime call runtime.
- `cc-worker`: import/export, callback repair, recording compensation, cleanup jobs.

Run one service:

```bash
go run ./cmd/cc-call -addr :8085
```

Verify:

```bash
go test ./...
go vet ./...
```

## Layout

The repository follows a Go-first dependency layout:

- `cmd/*`: tiny process entrypoints.
- `internal/app`: service assembly and shared Gin runtime.
- `internal/domain/*`: pure business logic for CTI, ESL, and future domains.
- `internal/transport/*`: Gin/MQ transport adapters.
- `internal/infra/*`: GORM, Redis, MQ, FS registry, outbox, config, logging, and other external adapters.
- `internal/contracts`: compatibility contracts that must remain stable during migration.

See [docs/PROJECT_LAYOUT.md](docs/PROJECT_LAYOUT.md) for the full rules.
