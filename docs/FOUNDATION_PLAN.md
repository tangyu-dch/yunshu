# Foundation Plan

These are the areas that must be decided early. They are expensive to retrofit once services, consumers, and workflows grow.

## 1. Contract Governance

- Every HTTP route must be registered in `internal/contracts/routes.go`.
- Every externally visible error must be registered in `internal/contracts/errors.go`.
- Every Redis key must be registered in `internal/contracts/redis.go`.
- Every MQ queue or stream must be registered in `internal/contracts/mq.go`.
- Every durable event should use `contracts.EventEnvelope`.
- Event payload changes require a version bump or backward-compatible optional fields.

## 2. Tenant And Permission Context

Tenant identity must flow through HTTP, MQ, workflow events, scheduled jobs, and repair tools.

Rules:

- Use `contracts.TenantContext` in `context.Context`.
- Internal calls must explicitly mark `Internal=true`.
- Domain logic must not infer tenant scope from raw Gin headers or GORM sessions.
- Async jobs need stored tenant context because no HTTP request exists when they run.

## 3. Data And Migration

Plan schema work before implementing repositories:

- Create schema registry docs before adding GORM models.
- Preserve soft delete, tenant fields, indexes, defaults, enum values, and JSON fields.
- Migrations must be forward-only and rollback-aware.
- Critical state changes should prefer outbox publication.
- Dual-write or shadow-read migration must include reconciliation reports.

## 4. Observability

Before production traffic:

- Metrics names and labels must be standardized.
- Logs must use `internal/infra/logging`, and log messages must be Chinese while structured field names remain stable English identifiers.
- Traces must carry request id, trace id, merchant id, user id, call id, command id, fs addr, and workflow id.
- Dashboards should cover HTTP, MQ, Redis, DB, FS node health, workflow lag, CDR lag, recording upload lag, and callback retry.

## 5. Workflow And Eventing

Long-running business behavior should be workflow/event first:

- Define workflow before implementing multi-step behavior.
- Model failures as events, not side comments in handlers.
- Persist workflow state before relying on it across restarts.
- Handler side effects need idempotency keys.
- Production event transport should use Redis Stream or RabbitMQ consumer groups. Consumers ack only after handlers succeed.

## 6. Deployment And Runtime

Each service needs:

- config file and environment overrides
- health and readiness probes
- graceful shutdown
- per-service logger
- resource limits
- safe defaults for timeouts and retry intervals
- clear ownership of scheduled jobs

## 7. Testing Strategy

Testing should be layered:

- domain unit tests
- transport contract tests
- workflow replay tests
- infra adapter tests with local containers when adapters are added
- concurrency tests for idempotency, locks, counters, and FS ownership
- compatibility tests against  response envelopes and error codes

## 8. Security And Privacy

- Never log raw customer phones in normal logs.
- Mask phones in operational logs and metrics.
- Secrets must come from environment or secret manager, not YAML committed to git.
- OpenAPI signing rules need compatibility tests.
- Internal routes need service identity or network-level protection.

## 9. Grey Release And Rollback

Plan migration flags before business code depends on them:

- per tenant
- per service
- per call type
- per workflow
- shadow mode
- dual-write mode
- fallback to  route
