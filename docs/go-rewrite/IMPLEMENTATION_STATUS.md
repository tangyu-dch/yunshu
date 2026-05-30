# Go Rewrite Implementation Status

This repository now contains the initial runnable Go rewrite workspace.

Implemented baseline:

- Four deployable service commands under `cmd/`: `cc-edge`, `cc-console`, `cc-call`, and `cc-worker`.
- Compatibility `Result` response envelope.
- Route registry for all known  controller surfaces from the inventory.
- Redis key registry with owner, schema, TTL, reader/writer/delete/idempotency notes.
- MQ queue registry with ack, retry, dead-letter, idempotency, and observability notes.
- Telephony command/event model carrying `callId`, `uuid`, `fsAddr`, leg role, and command id.
- In-memory idempotency store for tests and local development.
- Generic finite state machine used by CTI and ESL slices.
- CTI number selection business path, including failure as a first-class result.
- ESL event ownership lease model and lifecycle reducer.
- Health and route discovery endpoints for every service.
- Go-first project layout with `internal/domain`, `internal/transport`, and `internal/infra` boundaries.
- Gin transport tests for CTI and ESL compatibility responses.
- Config loading from YAML with environment overrides.
- GORM MySQL wiring for future repository adapters.
- CTI allocation service combining idempotency, selection, concurrency slots, and outbox publication.
- ESL command service combining validation, idempotency, structured logs, and executor abstraction.
- ESL call session service with FS event state transitions and CDR outbox publication at `CHANNEL_HANGUP_COMPLETE`.
- GORM-backed durable `message_outbox` store and destination-based outbox dispatcher for retryable projection/callback/WebSocket delivery nodes, including worker lease claiming for multi-instance `cc-worker` deployments.
- Batch customer callback delivery through `cti_batch_callback`: completion events write callback outbox entries, and `cc-worker` posts them to `CALLBACK_URL` with idempotency headers and optional HMAC signature.
- CDR finalization handler for `call_center_cdr_queue`: `cc-worker` idempotently persists Go-native `call_cdr_record` rows as the durable settlement point for billing, recording, report, and downstream CDR push nodes.
- CDR persisted fanout now creates explicit outbox nodes for billing, recording upload, report projection, and downstream CDR push. CDR payload carries merchant/user/batch/recording context to avoid hot-path repair queries.
- `cti_cdr_billing` now persists `call_billing_ledger` idempotently as a pending billing workflow node and can write a default-rate `rated` estimate when `worker.billing.defaultRatePerMin` or `WORKER_BILLING_DEFAULT_RATE_PER_MIN` is configured. It now also emits `cti_billing_settlement`, which persists `call_billing_settlement_job` and performs the first balance mutation step when the merchant billing overview row exists.
- `cti_cdr_recording` now persists `call_recording_job`, records missing `recordFilePath` as `skipped`, and can submit uploads to `RECORDING_UPLOAD_URL` with uploaded/failed state tracking.
- `cti_cdr_report_projection` now persists `call_report_projection` for query-friendly CDR views.
- `cti_cdr_downstream_push` now persists `call_downstream_push_job` and can deliver to `DOWNSTREAM_CDR_URL`, marking jobs delivered/failed while preserving outbox retry semantics.
- cc-worker now initializes an outbox dispatcher loop with configurable interval, batch size, and retry delay; batch tel/task projection destinations write Redis runtime projection hashes and publish lightweight WebSocket refresh events when Redis is configured.
- cc-call now exposes `/cti/ws` WebSocket fanout: it subscribes to Redis refresh events, reads the referenced projection hash, requires merchant-scoped subscriptions, and broadcasts projection payloads only to matching clients.
- In-process event bus, workflow runner, workflow instance store, and cc-call event consumers for API outbound and FS event workflows.
- Redis Stream event bus adapter with XADD publish, consumer group read, handler dispatch, and success-only XACK semantics.
- FreeSWITCH ESL connection pool using -compatible database node configuration from the `freeswitch` table, with dynamic load/remove/reload/status HTTP operations.
- cc-console now implements the first operate management slice for `/operate/freeswitch`: list, detail, save, enable, disable, and logical delete against the -compatible `freeswitch` registry, with local YAML/memory fallback when no MySQL DSN is configured.
- cc-console now implements -compatible extension management for `/operate/extension` and merchant phone-group management for `/merchant/phone-group`, including page, detail, save, delete, enable/disable where applicable, plus number and skill-group binding on phone groups.
- cc-console now implements channel, pool, pool-phone, and skill-group management slices with -compatible `channel`, `pool`, `pool_phone`, `pool_phone_skill_group`, `skill_group`, and `user_skill_group` tables so operators can configure the full number-selection chain instead of only reading it at runtime.
- cc-console now implements merchant lifecycle management for `/operate/merchant` against the -compatible `merchant` table, with page, detail, add, update, delete, enable, and disable endpoints.
- cc-console now implements -compatible fee-rate management for `/operate/rate` against `call_rate`, with referenced-delete protection against `gateway.rate_id` and `call_rate_merchant`; merchant save flows can now persist `rateId` bindings through `call_rate_merchant`.
- cc-console now implements -compatible blacklist management for `/operate/blacklist` against `blacklist` and `blacklist_gateway`, including page, detail, add, update, and delete of blacklist libraries plus gateway ignore mappings.
- cc-console now implements -compatible whitelist management for `/operate/whitelist` against `whitelist_data` and `whitelist_data_merchant`, including page, batch add, detail, update, and delete of white-number data plus merchant bindings.
- cc-console now implements -compatible merchant billing management for `/operate/billing`, including billing-overview page/query, payment-mode and credit-limit save, manual recharge, and recharge-record page against `merchant_billing_overview` and `merchant_billing_recharge`.
- cc-console merchant save/detail flows now preserve the  `merchant.whitelist_domains` field alongside rate bindings, so merchant access allow-lists stay editable through the same CRUD surface.
- `cc-call` now marks selection candidates with whitelist/blacklist runtime flags before selector execution so `WhitelistHit` can raise selection priority and blacklist hits can fail closed without moving routing logic into HTTP handlers.
- `cc-call` selection marking now also consumes -compatible `riskId` / merchant risk bindings for blacklist-level blocking, so risk-driven blacklist rejection no longer stops at the transport DTO layer.
- `cc-call` selection marking now also consumes -compatible `risk_control` data for blind-area and callee-frequency blocking, while still letting whitelist hits bypass these risk-only filters and keeping physical availability/concurrency checks intact.
- `cc-call` selection marking now also consumes -compatible `channel.blind_area` and `channel.config` for channel blind-spot and channel-frequency blocking, so non-whitelisted candidates still respect physical routing constraints before runtime claim.
- `cc-call` selection marker now keeps hot lookup results in an in-process TTL cache to avoid re-querying the same whitelist, risk, and channel data on repeated selection attempts, while still treating MySQL/Redis as the source of truth.
- cc-console now implements lightweight management auth for `/operate/auth` and `/merchant/auth` with Redis-backed token issuance, token lookup, and logout when Redis is configured, with in-memory fallback for local development.
- cc-console now injects tenant context from management tokens back into request context so downstream handlers can read `contracts.TenantContext` consistently.
- cc-console now enforces token-based access for protected `/operate/*` and `/merchant/*` management routes while keeping login, logout, token lookup, health checks, and contract discovery open.
- cc-console now carries route-level functional permissions in the login token, with wildcard-aware permission checks and fail-closed route mapping for operate/merchant actions. When MySQL is configured, login permissions and route-permission mappings are read from Go-native `console_permission`, `console_role_permission`, and `console_route_permission` tables, with static permission rules kept as migration fallback.
- cc-console now implements the first gateway management slice for `/operate/gateway`: -style add, update, delete, page, detail, and codec list endpoints backed by -compatible `gateway` and `pool.gateway_id` mappings, with explicit runtime sync-required results.
- cc-console can dispatch gateway runtime sync to cc-call when `CC_CALL_BASE_URL`/`console.callBaseURL` is configured, matching  operate-to-ESL Feign behavior.
- cc-console now implements Kamailio dispatcher management for `/operate/kamailio/dispatcher` with page, detail, add, update, delete, and reload endpoints backed by a Go-native `kamailio_dispatcher` table.
- cc-console now implements the remaining merchant management surface for `/merchant/batch-call-task`, `/merchant/batch-call-dialpad`, `/merchant/call-record`, and `/merchant/ai-model-flow`, including list/detail/save/enable/disable, dialpad control, CDR query, precheck, and publish operations with Chinese logs and focused route tests; AI flow now has a Go-native database repository instead of only an in-memory fallback.
- `cc-call` now implements Kamailio subscriber management for `/cti/kamailio/subscriber` with page, detail, add, update, delete, and reload endpoints backed by a Go-native `kamailio_subscriber` table and Redis auth-cache invalidation.
- cc-call now exposes -compatible `/esl/gateway` create/update/delete sync entrypoints that resolve `gateway` rows, enumerate enabled FreeSWITCH targets, and call each node's -compatible FS cmd HTTP path.
- API outbound originate planning now follows  `AGENT_FIRST` semantics: the first originate targets the agent extension resolved from the -compatible `extension` table and carries masked caller display variables.
- ESL API outbound now includes -compatible database guard checks for merchant user, merchant status/expiry, and prepaid billing balance.
- ESL API outbound guard now reads -compatible Redis hash `extension:status` and rejects offline, pre-ring, ringing, and talking extensions before originate.
- API and batch originate plans now carry supplement ring / broadcast-time metadata into ESL workflow variables, and `CHANNEL_PROGRESS_MEDIA` is modeled as an explicit ringback/early-media stage instead of a hidden event side note. API outbound bridge execution now waits for both agent answer and customer `CHANNEL_PROGRESS_MEDIA`/`CHANNEL_ANSWER`; `CHANNEL_PROGRESS` only advances ring state and never triggers direct bridge.
- FreeSWITCH ESL event adapter maps raw ESL headers into `TelephonyEvent` so session reducers and workflow consumers can process FS events from real connections.
- `cc-call` now claims a durable FS event lease before registering listeners and renews it while the ESL connection is alive, so multi-instance deployments skip nodes already owned by another consumer instead of double-consuming events.
- Batch outbound migration has started with -compatible `BatchCallReq`, `/cti/batch-call-task/dispatch`, `/esl/batch/call/start`, CUSTOMER_FIRST originate planning, session creation, request/command events, FS terminal-event advancement, tel-completed/task-completed events, next-number dispatch, durable outbox projection nodes, and retryable outbox dispatch.
- Number selection redesign has started with a runtime allocator port and Redis Lua claim/release implementation for idempotent high-concurrency number allocation.
- Runtime selection now falls back across eligible candidates when a single candidate's concurrency is exhausted, so high-load calls do not fail the whole allocation as long as another candidate is still available.
- CTI selection now has a -compatible candidate source adapter for `gateway -> pool -> pool_phone -> pool_phone_skill_group -> skill_group -> user_skill_group`, and `/cti/select/number/rule` can return real gateway metadata when MySQL is configured.
- `/cti/select-number` now also reuses the same candidate source when `userId` is present and explicit candidates are absent.
- Batch outbound database mapping has started for `merchant_batch_call_task` and `merchant_batch_call_task_list`;号码清单已提供事务 CAS 占用、失败释放、完成态写入和任务统计收口，CTI 调度器可派发下一个号码，并在无待拨/拨打中号码时完成任务。更完整的重试语义仍是后续 slice。
- Event envelope, error contract registry, and tenant context contract.
- Foundation planning docs for contracts, tenant context, data migration, metrics, deployment, testing, security, and rollback.

Next implementation slices should replace the remaining in-memory adapters with DB, Redis, RabbitMQ, OSS, Elasticsearch, Nacos, Kamailio, and WebSocket adapters while preserving the contracts in `internal/contracts`.
