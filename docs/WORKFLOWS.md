# Workflow And Event Orchestration

Yunshu should model long-running callcenter behavior as workflows driven by events. This avoids scattering business-specific `if/else` blocks across HTTP handlers, consumers, and lifecycle reducers.

## Kernel

The reusable kernel lives in `pkg/workflow`.

It provides:

- workflow definitions
- initial states
- event transitions
- optional step handlers
- workflow instances with variables

## Domain Usage

Current domain definitions:

- CTI: `internal/domain/cti/workflows.go`
- ESL: `internal/domain/esl/workflows.go`
- FS event adapter: `internal/infra/fsesl/event_adapter.go`
- cc-call workflow consumers: `internal/domain/callflow/consumer.go`

API outbound currently starts with -compatible `AGENT_FIRST` originate planning. The first FS command creates the agent leg, FS events are adapted into `TelephonyEvent`, and workflow consumers advance CTI/ESL instances through Redis Stream or the in-memory bus. `CHANNEL_PROGRESS` only means the leg is ringing; bridge execution must wait until the customer leg reaches `CHANNEL_PROGRESS_MEDIA` or `CHANNEL_ANSWER` and the agent leg has already answered.

API and batch outbound now also carry supplement ring metadata from gateway/candidate origin options into the ESL originate payload. `CHANNEL_PROGRESS_MEDIA` is treated as an explicit ringback/early-media phase in the ESL workflow, and the session payload forwards `playbackFile`, `supplementRing`, `supplementRingFile`, and broadcast-time metadata so later consumers can reason about ringback instead of treating it as a hidden originate parameter.

Batch outbound now starts from a scheduler node: runnable task -> CAS claim tel -> `cti.batch_call.requested` -> CTI workflow -> ESL CUSTOMER_FIRST command -> FS event workflow. When `CHANNEL_HANGUP_COMPLETE` is applied, a batch terminal consumer advances CTI with `terminal_event`, marks the list row complete, emits `cti.batch_call.tel_completed`, and attempts the next-number dispatch. If no more numbers are available, the scheduler runs a drained-task decision, completes the task, and emits `cti.batch_call.task.completed`. Tel/task completion events are projected through outbox entries (`cti_batch_tel_projection`, `cti_batch_task_projection`) and callback entries (`cti_batch_callback`) so WebSocket, callback, report, and notification delivery remain retryable workflow nodes.

Outbox delivery is a workflow node too. `OutboxDispatcher` claims durable entries with a worker lease, routes them by destination handler, marks success as `published`, and marks retryable failures as `failed` with `next_attempt_at`. Multi-instance `cc-worker` deployments must use the lease-aware `ClaimDue` path; direct naked scans are only acceptable for local fallback stores.

`cc-worker` starts the outbox dispatcher loop while keeping HTTP health and contract endpoints available. Current projection destinations are `cti_batch_tel_projection` and `cti_batch_task_projection`; customer callbacks use `cti_batch_callback` and are delivered with HTTP POST when `worker.callback.url` or `CALLBACK_URL` is configured. Handlers are intentionally isolated so report, billing, and notification adapters can be added without changing upstream workflow nodes.

CDR finalization is also an outbox workflow node. ESL session final hangup writes `call_center_cdr_queue` with call identifiers plus business context (`merchantId`, `userId`, batch ids, recording path when present); `cc-worker` consumes that destination and idempotently persists `call_cdr_record`. Billing, recording upload, report projection, ODS/internal CDR push, and repair jobs should continue from this durable CDR node.

After `call_cdr_record` is persisted, worker writes the next fanout nodes as outbox entries: `cti_cdr_billing`, `cti_cdr_recording`, `cti_cdr_report_projection`, and `cti_cdr_downstream_push`. Each node owns one side effect and can be retried, repaired, or replaced independently.

`cti_cdr_billing` first persists `call_billing_ledger` idempotently with `pending` status, then uses the worker-side default minute rate to write a `rated` estimate when `worker.billing.defaultRatePerMin` or `WORKER_BILLING_DEFAULT_RATE_PER_MIN` is configured. It then writes `cti_billing_settlement` as the next workflow node. Package deduction, invoice publication, and settlement policy refinement must still remain separate follow-up billing workflow nodes instead of being hidden inside the CDR consumer.

`cti_billing_settlement` persists a durable settlement job and performs the first balance mutation step when a merchant billing overview row exists. Missing balance rows are treated as no-op settlements so the workflow remains durable; later package and invoice nodes can still continue from the same job boundary.

`cti_cdr_recording` persists `call_recording_job` idempotently. When `recordFilePath` is missing it records `skipped`, which keeps the missing recording path visible to repair jobs instead of hiding it in logs. When `RECORDING_UPLOAD_URL` is configured, worker submits the upload request, marks `uploaded` after acknowledgement, and marks `failed` plus returns an error on retryable failures so outbox retry remains active.

`cti_cdr_report_projection` persists `call_report_projection` idempotently so console/report queries do not scan session or outbox hot paths. Wider merchant/hour/day aggregates should be follow-up projection nodes fed from this same CDR fact.

`cti_cdr_downstream_push` persists `call_downstream_push_job` idempotently. When `DOWNSTREAM_CDR_URL` is configured the worker posts the CDR payload, marks the job `delivered` after acknowledgement, and marks `failed` plus returns an error on retryable failures so outbox retry remains active. Internal CDR, ODS, open API, or merchant-specific delivery adapters can replace the HTTP adapter behind the same job boundary.

The first real projection adapter writes batch runtime views to Redis hashes: `batch:{taskId}:tel:{telId}` for number completion and `batch:{taskId}:summary` for task completion. After the hash write succeeds, worker publishes a lightweight refresh event to `cti_websocket_push_event`; WebSocket fanout should read the projection key instead of trusting the Pub/Sub payload as final truth.

`/cti/ws` is the first WebSocket fanout node. It subscribes to `cti_websocket_push_event`, reads the Redis hash named by `projectionKey`, and broadcasts `{type, merchantId, userId, taskId, telId, projectionKey, projection}` to matching clients. Clients must subscribe with `merchantId` and may narrow with `taskId`; fanout nodes must filter by these scopes before writing to a socket. Push events without `merchantId` are not broadcast. It does not decide task state and does not read hot SQL tables.

Future APIs, merchant workflows, operate workflows, import/export jobs, CDR repair, recording upload, callback retry, and billing must use the same model.

FreeSWITCH event consumption also uses the same lease model. Each `cc-call` instance must claim the FS node lease before registering event listeners, renew the lease while the ESL connection is alive, and release it on disconnect or shutdown. If a lease is already held by another instance, the node should be skipped instead of creating duplicate event consumers. If renewal fails, the connection should fail closed so another owner can take over without double-consuming events.

## Mandatory Flow Nodes

Every production call-center process should be broken into explicit workflow nodes. Typical nodes are:

- request_received: API/HTTP/MQ/Stream 入口只做解析、认证上下文和事件触发。
- validate_context: 校验商户、用户、余额、权限、任务状态和幂等。
- acquire_resource: 占用号码、网关、FS 节点、坐席、并发槽位或任务清单行。
- select_route: 选号、选网关、选 FS 节点、选坐席或机器人。
- dispatch_command: 下发 ESL、MQ、Redis Stream、第三方 HTTP 或内部命令。
- apply_external_event: 消费 FreeSWITCH、回调、文件上传、计费结果等外部事件。
- project_message: WebSocket 推送、报表投影、通知、第三方 webhook 和客户回调。
- compensate_or_retry: 失败重试、释放资源、补偿状态、死信修复。
- finalize: CDR、录音、计费、任务统计和审计收口。

## Rules

- New multi-step business behavior should start as a workflow definition.
- HTTP handlers and MQ consumers should publish or apply events; they should not own long business branches.
- Selection failure, no-answer, bridge failure, hangup, recording failure, callback retry, and repair should be modeled as events.
- Final message push, WebSocket projection, callbacks, billing notification, CDR publication, and report projection are workflow/event consumers, not inline side effects hidden in controllers.
- Workflow handlers must be idempotent when they perform side effects.
- Workflow instance state must become durable before production traffic relies on it.
