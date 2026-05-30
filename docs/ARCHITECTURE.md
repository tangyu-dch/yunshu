# Yunshu Architecture

## Deployment Shape

Yunshu is designed as four independently deployable Go services. The code keeps CTI, ESL, API, merchant, and operate boundaries internally, but deployment starts smaller to reduce release, config, observability, and troubleshooting cost.

- `cc-edge`: request ingress, internal route protection, trace propagation, signed external API compatibility.
- `cc-console`: merchant management, operate management, dialpad workflows, FS/Kamailio/gateway/base-data control.
- `cc-call`: number selection, batch scheduling, WebSocket projection, CDR, billing, callbacks, Kamailio, FreeSWITCH nodes, ESL commands, event ownership, call lifecycle, recording, CDR task publication.
- `cc-worker`: outbox delivery, projection dispatch, import/export, model jobs, callback repair, recording repair, cleanup.

Every service exposes:

- `GET /healthz`
- `GET /contracts/routes`
- `GET /contracts/redis`
- `GET /contracts/mq`

## Multi FreeSWITCH Node Design

FreeSWITCH nodes are represented by `internal/infra/fsregistry.Node`. Production nodes are loaded from the -compatible database table `freeswitch`; config-file nodes are only a local fallback when no MySQL DSN is configured. ESL instances claim ownership through a lease before consuming events from a node. A node can be available for commands while event consumption remains owned by exactly one live ESL instance.

Ownership rules:

- Claim by `fsAddr`.
- Lease has TTL and must be renewed.
- If the owner expires, another ESL instance may claim.
- Command routing still records `fsAddr`, `uuid`, `callId`, leg role, and command id.
- Dynamic node management uses `/esl/freeswitch/reload`, `/esl/freeswitch/load/:id`, `/esl/freeswitch/remove/:id`, `/esl/freeswitch/list`, and `/esl/freeswitch/status`.
- API outbound node selection filters by destination `setid` and uses -compatible effective weight: when congestion control `cc` is enabled, `rweight` participates in selection; otherwise `weight` participates.

`cc-call` may run many replicas. Each replica can serve CTI HTTP requests, but FS event consumption is controlled per `fsAddr` by lease ownership, so multiple replicas do not duplicate the same FS event stream.

## High-Concurrency Design

The project uses explicit primitives that can be backed by Redis, DB, or local memory in tests:

- `pkg/idempotency.Store`: command and consumer dedupe.
- `internal/infra/limit.ShardedLimiter`: concurrency slots by merchant, gateway, caller, or task.
- `internal/infra/outbox`: durable event publication contract.
- `pkg/state.Machine`: auditable task and call state transitions.

For production, memory implementations must be replaced with Redis or DB-backed adapters without changing domain behavior.

## Go Stack

The HTTP layer uses Gin for routing, middleware, binding, and JSON responses. Domain code accepts `context.Context` plus plain request structs so the same logic can be used by HTTP handlers, RabbitMQ consumers, scheduled jobs, replay tests, and repair tools.

Database adapters use GORM in `internal/infra/db` and future repository packages. GORM models and transactions stay in infrastructure; CTI, ESL, merchant, operate, and API domain packages should depend on small interfaces instead of `*gorm.DB`.

## Code Layout

The code is split by dependency direction:

- `cmd/*` starts one process and delegates to `internal/app`.
- `internal/app` composes the service, Gin runtime, middleware, and transport registration.
- `internal/transport/http/*` contains Gin handlers and request/response binding.
- `internal/domain/*` contains pure business logic that can be reused by HTTP, MQ, scheduled jobs, repair tools, and replay tests.
- `internal/infra/*` contains external adapters and replaceable infrastructure implementations.
- `internal/contracts` stores compatibility contracts for -to-Go migration.

The detailed layout rules live in `docs/PROJECT_LAYOUT.md`.

## Async Reliability

Consumers ack only after durable side effects complete. Each queue contract in `internal/contracts/mq.go` defines:

- producer and consumer
- ack timing
- retry policy
- dead letter target
- idempotency key
- observability requirements

## Workflow And Events

Long-running CTI and ESL behavior should be modeled as workflows driven by events. The shared kernel lives in `pkg/workflow`, while domain definitions live under `internal/domain/*/workflows.go`.

Handlers, MQ consumers, and schedulers should apply workflow events instead of carrying large business-specific `if/else` branches.

Cross-service events should use `contracts.EventEnvelope` so event id, version, source service, idempotency key, aggregate identity, and trace identity stay consistent.

## Logging

Logging is centralized in `internal/infra/logging` and uses Go `log/slog`. HTTP middleware logs service, method, path, status, duration, request id, and trace id. Telephony code must also log call identity and runtime identity:

- `callId`
- `uuid`
- `fsAddr`
- `legRole`
- `commandId`
- failure reason

Raw customer phone numbers should be masked in normal logs.

See `docs/LOGGING.md` for the field catalog and rules.
