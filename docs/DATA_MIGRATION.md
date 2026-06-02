# Data And Migration Plan

The Go rewrite must keep  data contracts until a migration contract replaces them.

## Schema Registry

Before implementing a repository for an entity, document:

- table name
- primary key
- tenant fields
- soft delete fields
- indexes
- enum fields
- JSON fields
- default values
- time fields and timezone rules
-  entity name
- Go model name

## Migration Rules

- Prefer additive schema changes.
- Backfills must be idempotent and resumable.
- Writes that publish messages should use outbox.
- Data repair commands need dry-run mode.
- Every migration needs a verification query.

## FreeSWITCH Node Table

The Go rewrite must reuse the  `freeswitch` table for production FS node configuration.

- table name: `freeswitch`
-  entity: `com.dolphin.datasource.model.entity.FreeswitchDO`
- Go model: `internal/infra/fsregistry.FreeswitchModel`
- primary key: `id`
- soft delete field: `del_flag`
- enable field: `enable`
- address fields: `address`, `local_address`, `esl_port`, `sip_port`, `cmd_port`
- credential field: `password`
- routing fields: `setid`, `weight`, `rweight`, `cc`, `canary`
- time fields: `created_time`, `updated_time`

Production `cc-call` loads enabled, non-deleted nodes from this table. YAML node configuration is only a local development fallback when `mysql.dsn` is empty.

## FreeSWITCH Event Lease Table

The Go rewrite adds a supplemental lease table so multiple `cc-call` instances do not consume the same FS event stream.

- table name: `freeswitch_event_lease`
-  entity: none, Go-native supplemental table
- Go model: `internal/infra/fsregistry.FreeswitchEventLeaseModel`
- primary key: `fs_addr`
- ownership fields: `owner`, `lease_expiry`
- time fields: `created_time`, `updated_time`

`cc-call` must claim this lease before registering an ESL event listener for a node. If the active lease is held by another owner, the instance skips that node. Disconnect and shutdown paths must release the lease for the current owner only.

## Extension Table

API outbound must reuse the unified `cc_res_extension` table to resolve the agent extension by `user_id`. The same table now also has a cc-console management surface so operators can create, update, enable, disable, and delete extensions without raw SQL.

- table name: `cc_res_extension`
-  entity: `com.dolphin.datasource.model.entity.ExtensionDO`
- Go model: `internal/infra/resource.ExtensionModel`
- primary key: `id`
- soft delete field: `del_flag`
- enable field: `enable`
- lookup field: `user_id`
- business fields: `extension_number`, `password`, `merchant_id`, `bind_type` (1 for Manual, 2 for Dynamic), `sip_domain`, `ha1`, `ha1b`
- time fields: `created_time`, `updated_time`

Production API outbound reads enabled, non-deleted extension rows before building the AGENT_FIRST originate plan. Request `extra.extension` is only a local fallback when no MySQL DSN is configured.

To support secure multi-tenant authentication in Kamailio without exposing plaintext passwords, the table stores pre-calculated `ha1` and `ha1b` digests. These are automatically generated or updated on the Go side when a password changes.

## Phone Group Table

Merchant phone groups use the  `merchant_phone_group` table and its relation tables so operators can bind caller numbers and skill groups through the Go console instead of only through runtime readers.

- phone group table: `merchant_phone_group`
- phone group  entity: `com.dolphin.datasource.model.entity.PhoneGroupDO`
- phone group Go model: `internal/infra/directory.PhoneGroupModel`
- phone-group / phone relation table: `merchant_phone_group_pool_phone_ref`
- phone-group / skill-group relation table: `merchant_phone_group_skill_group_ref`

## Gateway And Pool Tables

Operate gateway management must reuse the  `gateway` table and  `pool.gateway_id` binding until a dedicated Go runtime projection replaces direct table reads.

- channel table: `channel`
- channel  entity: `com.dolphin.datasource.model.entity.ChannelDO`
- channel Go model: `internal/infra/directory.ChannelModel`
- channel fields: `name`, `config`, `blind_area`, `remark`, `enable`, `del_flag`
- gateway table: `gateway`
- gateway  entity: `com.dolphin.datasource.model.entity.GatewayDO`
- gateway Go model: `internal/infra/directory.GatewayModel`
- primary key: `id`
- soft delete field: `del_flag`
- enable field: `enable`
- routing fields: `name`, `realm`, `port`, `model`, `priority`, `concurrency`
- rewrite and prefix fields: `caller_prefix`, `callee_prefix`, `caller_rewrite_rule`, `callee_rewrite_rule`
- media fields: `codec_prefs`, `supplement_ring`, `supplement_ring_file`, `broadcast_time`, `broadcast_time_flag`
- billing field: `rate_id`
- pool table: `pool`
- pool  entity: `com.dolphin.datasource.model.entity.PoolDO`
- pool Go model: `internal/infra/directory.PoolModel`
- pool binding field: `gateway_id`

The Go operate management path now writes `channel`, `gateway`, and `pool` on the low-QPS management surface. CTI number selection remains a hot path and must use cache/materialized runtime structures before production load; direct `channel`, `gateway`, and `pool` reads are only acceptable for admin, repair, and migration checks.

## Call Rate Tables

 operate fee-rate management remains the source of truth for gateway and merchant billing bindings.

- call-rate table: `call_rate`
- call-rate  entity: `com.dolphin.datasource.model.entity.CallRateDO`
- call-rate Go model: `internal/infra/directory.CallRateModel`
- primary key: `id`
- soft delete field: `del_flag`
- fields: `rate_name`, `billing_price`, `billing_cycle`, `remark`
- merchant-rate relation table: `call_rate_merchant`
- merchant-rate  entity: `com.dolphin.datasource.model.entity.CallRateMerchantDO`
- merchant-rate Go model: `internal/infra/directory.CallRateMerchantModel`
- relation fields: `merchant_id`, `rate_id`

`cc-console` now exposes `/operate/rate` against these tables and writes merchant fee-rate bindings through `call_rate_merchant` when `/operate/merchant/add` or `/operate/merchant/update` carries `rateId`. Deleting a rate must fail closed when the rate is still referenced by any active `gateway.rate_id` row or any `call_rate_merchant` binding.

## Blacklist Tables

 operate blacklist management remains the source of truth for system blacklist libraries and gateway ignore relations.

- blacklist table: `blacklist`
- blacklist  entity: `com.dolphin.datasource.model.entity.BlacklistDO`
- blacklist Go model: `internal/infra/directory.BlacklistModel`
- primary key: `id`
- soft delete field: `del_flag`
- fields: `name`, `verification_channel`, `remark`
- blacklist-gateway relation table: `blacklist_gateway`
- blacklist-gateway  entity: `com.dolphin.datasource.model.entity.BlacklistGatewayDO`
- blacklist-gateway Go model: `internal/infra/directory.BlacklistGatewayModel`
- relation fields: `blacklist_id`, `gateway_id`

`cc-console` now exposes `/operate/blacklist` against these tables for page, detail, add, update, and delete. This slice only manages blacklist libraries and gateway ignore mappings; `blacklist_data` runtime filtering and merchant risk-binding flow should be connected in the next CTI selection slice rather than hidden inside management handlers.

`cc-call` now also marks selection candidates from these tables before running the selector, so `WhitelistHit` and `BlacklistHit` can influence actual call routing while keeping the management surface separate.

## Whitelist Tables

 operate whitelist management remains the source of truth for phone whitelist data and merchant bindings.

- whitelist-data table: `whitelist_data`
- whitelist-data  entity: `com.dolphin.datasource.model.entity.WhitelistDataDO`
- whitelist-data Go model: `internal/infra/directory.WhitelistDataModel`
- primary key: `id`
- soft delete field: `del_flag`
- fields: `phone`, `number_type`
- whitelist-merchant relation table: `whitelist_data_merchant`
- whitelist-merchant  entity: `com.dolphin.datasource.model.entity.WhitelistDataMerchantDO`
- whitelist-merchant Go model: `internal/infra/directory.WhitelistDataMerchantModel`
- relation fields: `white_id`, `merchant_id`

`cc-console` now exposes `/operate/whitelist` against these tables for page, batch add, detail, update, and delete. This slice only maintains whitelist master data and merchant bindings; runtime `WhitelistHit` candidate marking and merchant `whitelist_domains` exposure remain separate follow-up slices.

`cc-call` now marks selection candidates from these tables before running the selector, so whitelist priority can influence actual call routing without mixing it into the management handler.

## Merchant Whitelist Domain Field

The merchant table also carries a domain/IP allow-list used by access and integration checks.

- merchant field: `whitelist_domains`
-  entity field: `com.dolphin.datasource.model.entity.MerchantDO.whitelistDomains`
- Go model field: `internal/infra/directory.MerchantModel.whitelistDomains`

`cc-console` merchant save/detail routes now preserve this field alongside name, account, rate, and enable state. It is intentionally separate from `whitelist_data` and should continue to be treated as merchant access configuration rather than numbered whitelist data.

## Merchant Billing Tables

 operate billing management remains the source of truth for merchant billing overview and manual recharge records.

- billing-overview table: `merchant_billing_overview`
- billing-overview  entity: `com.dolphin.datasource.model.entity.MerchantBillingOverviewDO`
- billing-overview Go model: `internal/infra/directory.MerchantBillingOverviewModel`
- primary key: `id`
- fields: `merchant_id`, `payment_mode`, `current_balance`, `fee_date`, `daily_total_amount`, `fee_month`, `monthly_total_amount`, `credit_limit`
- recharge table: `merchant_billing_recharge`
- recharge  entity: `com.dolphin.datasource.model.entity.MerchantBillingRechargeDO`
- recharge Go model: `internal/infra/directory.MerchantBillingRechargeModel`
- fields: `merchant_id`, `amount`, `remark`, `operator`

`cc-console` now exposes `/operate/billing/overview/page`, `/operate/billing/overview/save`, `/operate/billing/recharge`, and `/operate/billing/recharge-records` against these tables. This slice only serves low-QPS management read/write and manual balance adjustment; worker-side `cti_cdr_billing` and `cti_billing_settlement` remain the production charging workflow truth.

## Number Selection Resource Tables

The migration-period CTI candidate source follows  `PhoneResourceMapper`.

- number table: `pool_phone`
- number  entity: `com.dolphin.datasource.model.entity.PoolPhoneDO`
- number Go model: `internal/infra/directory.PoolPhoneModel`
- number fields: `pool_id`, `phone`, `province`, `city`, `concurrency`, `call_limit`, `enable`, `del_flag`
- number-skill relation table: `pool_phone_skill_group`
- number-skill  entity: `com.dolphin.datasource.model.entity.PoolPhoneSkillGroupDO`
- number-skill Go model: `internal/infra/directory.PoolPhoneSkillGroupModel`
- skill table: `skill_group`
- skill  entity: `com.dolphin.datasource.model.entity.SkillGroupDO`
- skill Go model: `internal/infra/directory.SkillGroupModel`
- user-skill relation table: `user_skill_group`
- user-skill  entity: `com.dolphin.datasource.model.entity.UserSkillGroupDO`
- user-skill Go model: `internal/infra/directory.UserSkillGroupModel`

Go management routes now expose CRUD and relation maintenance for `channel`, `pool`, `pool_phone`, and `skill_group`, so operators can configure the full selection chain without touching SQL directly.

`cc-call` can read this relation for compatibility validation, but direct SQL joins must not become the final high-concurrency selection design. Production selection should move the joined candidate set into Redis/materialized projections and use atomic allocation for caller/gateway counters.

## Kamailio Dispatcher Table

The Go rewrite adds a Go-native dispatcher management table for `cc-console` operational control.

- table name: `kamailio_dispatcher`
-  entity: none, Go-native supplemental table
- Go model: `internal/infra/directory.DispatcherModel`
- primary key: `id`
- soft delete field: `del_flag`
- enable field: `enable`
- routing fields: `set_id`, `destination`, `flags`, `priority`, `attrs`
- description field: `description`
- time fields: `created_time`, `updated_time`

This table is intended for low-QPS management operations and Kamailio reload workflows. It should not become the final SIP routing truth if a production Kamailio sync projection or external control plane is introduced later.
## Kamailio Subscriber Table (DEPRECATED & MERGED)

The Go rewrite has completely deprecated and removed the standalone `kamailio_subscriber` table. 

To simplify database schemas, eliminate redundant tables, and align with security requirements, the Kamailio SIP authentication and subscriber management have been fully merged into the unified extension table `cc_res_extension`. 

Kamailio connects directly to the `cc_res_extension` table using `auth_db` module parameters, reading `ha1`/`ha1b` pre-calculated digests for SIP Digest authentication. `cc-call` invalidates `kamailio:auth:*` keys in Redis after extension configuration updates to prevent the cache from drifting.

## Merchant AI Flow Table

The Go rewrite adds a Go-native table for merchant-side AI flow management.

- table name: `merchant_ai_model_flow`
-  entity: none, Go-native supplemental table
- Go model: `internal/infra/directory.AIModelFlowModel`
- primary key: `id`
- soft delete field: `del_flag`
- business fields: `name`, `prompt`, `description`, `published`, `prechecked`
- time fields: `created_time`, `updated_time`

This table is only used by `cc-console` management workflows. It should stay low-QPS and should not be repurposed as a runtime call-processing dependency.

## Console Auth Session Redis Key

The Go rewrite uses a Redis session key for management auth so `cc-console` can share login state across instances.

- Redis key: `console:auth:session:*`
- Redis type: string
- value: JSON-encoded `internal/domain/auth.AuthTicket`
- TTL: 12h
- writers: `cc-console`
- readers: `cc-console`

The token store is an authorization dependency, so Redis availability should be treated as required when management auth is configured for production. Local development may fall back to the in-memory store when Redis is not configured.

## Console Account And Permission Tables

The Go rewrite uses a Go-native account and permission design. `console_account` is the single login and account-management truth.

- account table: `console_account`
- Go model: `internal/infra/directory.ConsoleAccountModel`
- primary key: `id`
- unique key: `username`
- fields: `username`, `password_hash`, `merchant_id`, `user_id`, `role_id`, `account_type`, `data_scope`, `enable`, `del_flag`, `created_by`, `updated_by`, `created_time`, `updated_time`, `deleted_time`
- account types: `super_admin`, `operate_user`, `merchant_admin`, `merchant_user`
- data scopes: `global`, `merchant`

Default seeded accounts:

- `admin` / `admin123` with `super_admin`
- `operator` / `operator123` with `operate_lead`
- `merchant` / `merchant123` with `merchant_admin`

Business rules:

- `super_admin` can see and maintain all data.
- `operate_user` can create or maintain merchants only when role permissions grant the corresponding operate permission.
- Merchant-scoped accounts must fail login when their merchant is disabled, deleted, missing, or expired.
- One merchant can have only one enabled `merchant_admin`; a second enabled merchant admin must be rejected.
- `merchant_admin` can maintain only `merchant_user` accounts in the same merchant.
- `merchant_user` is a real usage account and cannot maintain platform or merchant account structure by default.

Permission tables are still Go-native so route permissions, role bindings, and operation permissions can be managed by operators instead of being hard-coded only in Go constants.

- permission catalog table: `console_permission`
- Go model: `internal/infra/directory.ConsolePermissionModel`
- primary key: `code`
- fields: `name`, `module`, `description`, `enable`, `del_flag`, `created_time`, `updated_time`
- role binding table: `console_role_permission`
- Go model: `internal/infra/directory.ConsoleRolePermissionModel`
- fields: `role_id`, `permission_code`, `enable`, `del_flag`, `created_time`, `updated_time`
- route binding table: `console_route_permission`
- Go model: `internal/infra/directory.ConsoleRoutePermissionModel`
- fields: `path_prefix`, `path_suffix`, `method`, `permission_code`, `sort`, `enable`, `del_flag`, `created_time`, `updated_time`

`cc-console` reads login permissions and route permission mappings from these tables when MySQL is configured. Local development may use the in-memory account repository when MySQL is not configured, but production should seed and maintain `console_account`, `console_role`, `console_permission`, `console_role_permission`, and `console_route_permission`.

Gateway runtime sync uses permission code `operate:gateway:sync` and route `/operate/gateway/sync/:id`. This route is intended for manual operational resync after gateway configuration has already been saved.

## API Outbound Guard Tables

ESL internal API outbound must keep  `OutboundRequestGuard` database checks.

- user table: `merchant_user`
- user  entity: `com.dolphin.datasource.model.entity.MerchantUserDO`
- user Go model: `internal/infra/directory.MerchantUserModel`
- merchant table: `merchant`
- merchant  entity: `com.dolphin.datasource.model.entity.MerchantDO`
- merchant Go model: `internal/infra/directory.MerchantModel`
- merchant fields: `name`, `account`, `expired_time`, `enable`, `rate_id`, `whitelist_domains`, `app_key`, `app_secret`, `max_agents`
- billing table: `merchant_billing_overview`
- billing  entity: `com.dolphin.datasource.model.entity.MerchantBillingOverviewDO`
- billing Go model: `internal/infra/directory.MerchantBillingOverviewModel`

The guard validates enabled user, enabled and unexpired merchant, and prepaid balance plus credit limit.

## Extension Status Redis Hash

SIP registration and busy-state checks reuse  `ExtensionStatusUtils`.

- Redis key: `extension:status`
- Redis type: hash
- hash field: extension number
- hash value:  `ExtensionStatus.status`
- status values: `-1` offline, `0` busy, `1` idle, `2` pre-ring, `3` ringing, `4` talking

API outbound rejects missing/offline status, pre-ring, ringing, and talking before sending originate.

## Message Outbox Table

The Go rewrite uses a Go-native durable outbox table for retryable workflow side effects.

- table name: `message_outbox`
-  entity: none, Go-native supplemental table
- Go model: `internal/infra/outbox.MessageOutboxModel`
- primary key: `id`
- unique idempotency field: `idempotency_key`
- aggregate fields: `aggregate_type`, `aggregate_id`
- routing field: `destination`
- payload field: `payload` JSON
- retry fields: `status`, `attempts`, `next_attempt_at`
- worker lease fields: `locked_by`, `locked_until`
- time fields: `created_at`, `updated_at`

This table is required for production WebSocket projection, callback, report projection, CDR publication, billing notification, and other final message delivery nodes. HTTP handlers and terminal consumers must not directly perform final push side effects; they should write or emit events that create outbox entries. Worker delivery must mark rows as `published` only after downstream acknowledgement, and mark rows as `failed` with a future `next_attempt_at` for retryable failures.

`cc-worker` scans this table with `worker.outbox.interval`, `worker.outbox.batchSize`, `worker.outbox.retryDelay`, `worker.outbox.lease`, and `worker.outbox.workerId`. Production workers must call `ClaimDue`, which writes `processing`, `locked_by`, and `locked_until` inside a transaction before delivery. MySQL 8 deployments should use `SELECT ... FOR UPDATE SKIP LOCKED` semantics through the GORM adapter so multiple worker instances do not deliver the same row concurrently. If a worker crashes, rows whose `locked_until` has expired become claimable again.

Batch customer callbacks use destination `cti_batch_callback`. The worker posts callback payloads to `worker.callback.url` / `CALLBACK_URL` with `X-Outbox-Id`, `X-Idempotency-Key`, and optional `X-Signature-SHA256`. Non-2xx responses remain retryable through the same outbox lease and retry fields.

## CDR Record Table

The Go rewrite adds a Go-native CDR finalization table as the first durable CTI settlement point after ESL hangup complete.

- table name: `call_cdr_record`
-  entity: migration-period supplemental table, final  compatibility mapping still needs review
- Go model: `internal/infra/cdr.RecordModel`
- primary key: `call_id`
- unique fields: `event_id`, `outbox_id`
- call fields: `uuid`, `fs_addr`, `profile`, `merchant_id`, `user_id`, `batch_task_id`, `batch_call_tel_id`, `hangup_cause`, `final_state`, `record_file_path`, `completed_at`
- raw JSON field: `raw_payload`
- time fields: `created_at`, `updated_at`

ESL writes `call_center_cdr_queue` outbox entries at final hangup complete and includes session metadata such as `merchantId`, `userId`, `batchTaskId`, `batchCallTelId`, and recording path when available. `cc-worker` consumes that destination and writes `call_cdr_record` idempotently. Billing, recording upload, report projection, ODS/internal CDR push, and customer callbacks should use this durable CDR node instead of re-reading hot session state.

## CDR Billing Ledger Table

The Go rewrite adds a Go-native billing ledger as the first durable billing workflow node after CDR persistence.

- table name: `call_billing_ledger`
-  entity: migration-period supplemental table, final  billing integration still needs rate-rule mapping
- Go model: `internal/infra/billing.LedgerModel`
- primary key: `id`
- unique field: `call_id`
- tenant fields: `merchant_id`, `user_id`
- billing fields: `profile`, `duration_sec`, `amount`, `currency`, `status`, `billed_at`
- trace fields: `source_outbox_id`, `raw_payload`
- time fields: `created_at`, `updated_at`

`cti_cdr_billing` writes this table idempotently. The initial status is `pending` and amount is `0`; when `worker.billing.defaultRatePerMin` or `WORKER_BILLING_DEFAULT_RATE_PER_MIN` is configured, the worker also writes a `rated` estimate using the default minute rate. -compatible rate, package, and settlement rules still need separate workflow nodes. This avoids losing billing work while preventing premature irreversible balance mutation.

## CDR Settlement Job Table

The Go rewrite adds a Go-native billing settlement workflow table after rate estimation.

- table name: `call_billing_settlement_job`
-  entity: migration-period supplemental table, final  billing settlement mapping still needs review
- Go model: `internal/infra/settlement.SettlementJobModel`
- primary key: `id`
- unique field: `call_id`
- tenant fields: `merchant_id`, `user_id`
- billing fields: `amount`, `rate_per_min`, `status`, `last_error`
- balance fields: `balance_before`, `balance_after`, `settled_at`
- trace fields: `source_outbox_id`, `raw_payload`
- time fields: `created_at`, `updated_at`

`cti_billing_settlement` writes this table idempotently and can apply a balance debit against `merchant_billing_overview` when the row exists. Missing balance rows are treated as no-op settlements so the workflow remains durable while package and invoice nodes are migrated separately.

## Recording Job Table

The Go rewrite adds a Go-native recording workflow table after CDR persistence.

- table name: `call_recording_job`
-  entity: migration-period supplemental table, final  recording/OSS mapping still needs review
- Go model: `internal/infra/recording.JobModel`
- primary key: `id`
- unique field: `call_id`
- tenant fields: `merchant_id`, `user_id`
- recording fields: `record_file_path`, `status`
- delivery fields: `attempts`, `last_error`, `uploaded_at`
- trace fields: `source_outbox_id`, `raw_payload`
- time fields: `created_at`, `updated_at`

`cti_cdr_recording` writes this table idempotently. If the CDR payload does not include `recordFilePath`, the job is saved as `skipped` rather than silently disappearing; repair tooling can later inspect missing recording paths. When `worker.recording.url` / `RECORDING_UPLOAD_URL` is configured, worker submits the upload request with `X-Outbox-Id`, `X-Idempotency-Key`, and optional `X-Signature-SHA256`. Success marks the job `uploaded`; retryable failure marks `failed` and returns an error so outbox retry remains active.

## Call Report Projection Table

The Go rewrite adds a single-call report projection table after CDR persistence.

- table name: `call_report_projection`
-  entity: migration-period supplemental table, final report aggregation mapping still needs review
- Go model: `internal/infra/reporting.ProjectionModel`
- primary key: `call_id`
- tenant fields: `merchant_id`, `user_id`
- batch fields: `batch_task_id`, `batch_call_tel_id`
- report fields: `profile`, `final_state`, `hangup_cause`, `duration_sec`, `completed_at`
- trace fields: `source_outbox_id`, `raw_payload`
- time fields: `created_at`, `updated_at`

`cti_cdr_report_projection` writes this table idempotently. Console/report APIs should query report projections or aggregates rather than scanning `message_outbox` or hot call session state.

## Downstream CDR Push Job Table

The Go rewrite adds a CDR downstream push job table after CDR persistence.

- table name: `call_downstream_push_job`
-  entity: migration-period supplemental table, final `internal_cdr_queue` and `ods_cdr_queue` compatibility mapping still needs review
- Go model: `internal/infra/downstream.JobModel`
- primary key: `id`
- indexed fields: `call_id`, `merchant_id`, `target`, `status`
- delivery fields: `attempts`, `last_error`, `delivered_at`
- trace fields: `source_outbox_id`, `raw_payload`
- time fields: `created_at`, `updated_at`

`cti_cdr_downstream_push` writes this table idempotently. When `worker.downstream.url` / `DOWNSTREAM_CDR_URL` is configured, worker posts the CDR payload with `X-Outbox-Id`, `X-Idempotency-Key`, `X-Downstream-Target`, and optional `X-Signature-SHA256`. A successful downstream acknowledgement marks the job `delivered`; retryable failures mark the job `failed` and return an error so the durable outbox retry policy remains active. Concrete MQ adapters for internal CDR or ODS can replace the HTTP adapter behind this same job boundary.

## Batch Runtime Projection Redis Keys

Batch runtime projections are generated by `cc-worker` from `message_outbox` entries.

- tel key: `batch:{taskId}:tel:{telId}`
- summary key: `batch:{taskId}:summary`
- Redis type: hash
- TTL: 7 days by default
- writer: `cc-worker`
- readers: `cc-console`, `cc-call`, future WebSocket projection services
- durable truth: `merchant_batch_call_task`, `merchant_batch_call_task_list`, and `message_outbox`

These keys are read-optimized projections for UI refresh, WebSocket fanout, and repair diagnostics. They must not become the only copy of task or call-list state.

After a projection hash is written, `cc-worker` publishes a lightweight refresh message to Redis Pub/Sub topic `cti_websocket_push_event`. The message includes `type`, `merchantId`, `userId`, `taskId`, optional `telId`, `projectionKey`, and `outboxId`. This Pub/Sub message is a wake-up signal only; WebSocket nodes must read the projection key or durable store before pushing user-visible state.

`cc-call` WebSocket nodes read `projectionKey` from the Pub/Sub message and broadcast the hash content only to matching `merchantId`/`taskId` subscribers. They must not query `merchant_batch_call_task_list` or other hot SQL tables on every fanout event.
