# Project Layout

Yunshu uses a Go-first layout. The folders are organized by runtime ownership and dependency direction, not by the old  module shape.

```text
cmd/
  cc-edge               gateway and external OpenAPI process
  cc-console            merchant and operate management process
  cc-call               CTI and ESL realtime runtime process
  cc-worker             background worker process
  update-agents         local maintenance command
configs/                deployable defaults and examples
docs/                   architecture and migration notes
internal/
  app/                  service assembly, Gin engine, shared middleware
  contracts/            compatibility contracts: HTTP, Redis, MQ, DTOs, enums, errors
  domain/
    cti/                pure CTI business logic
    esl/                pure ESL business logic
  transport/
    http/
      call/             CTI and ESL Gin handlers
      # edge/console/worker handlers will be created when their first real endpoint lands.
  infra/
    config/             config loading
    db/                 GORM database wiring and future repositories
    fsregistry/         FreeSWITCH node registry and leases
    limit/              concurrency primitives
    logging/            shared structured log fields
    mq/                 RabbitMQ ports and adapters
    outbox/             durable publication contract
    redis/              Redis atomic ports and adapters
  observability/        request context and trace identity helpers
pkg/
  idempotency/          reusable idempotency primitive
  state/                reusable finite state machine
  telephony/            reusable telephony constructors
```

## Dependency Direction

Allowed direction:

```text
cmd -> internal/app -> internal/transport -> internal/domain
                              |              ^
                              v              |
                         internal/contracts  |
                              |              |
internal/infra ---------------+--------------+
```

Rules:

- `internal/domain/*` must not import Gin, GORM, Redis, RabbitMQ, or concrete infra packages unless the package is a small domain-neutral primitive such as `internal/infra/limit`.
- `internal/transport/*` translates Gin requests into domain structs and compatibility `Result` responses.
- `internal/infra/*` owns external systems and adapter-specific behavior.
- `internal/contracts` is the compatibility boundary. Changing it requires updating tests, docs, and `AGENTS.md` with `go run ./cmd/update-agents`.
- `cmd/*` must stay tiny. Service construction belongs in `internal/app` or future service-specific assembly packages.

## Where New Code Goes

- New HTTP endpoint: add contract in `internal/contracts`, handler in `internal/transport/http/<service-domain>`, business logic in `internal/domain/<domain>`.
- New MQ consumer: add queue contract in `internal/contracts`, consumer adapter in `internal/infra/mq` or `internal/transport/mq`, business handler in `internal/domain/<domain>`.
- New DB table: add schema note under `docs/`, GORM model/repository under `internal/infra/db` or a repository subpackage, and keep domain packages behind interfaces.
- New FreeSWITCH behavior: command/event model in `internal/contracts` if externally shared, lifecycle logic in `internal/domain/esl`, connection adapter under `internal/infra`.
- New CTI scheduling behavior: state machine or rule in `internal/domain/cti`, durable delivery through `internal/infra/outbox`.
