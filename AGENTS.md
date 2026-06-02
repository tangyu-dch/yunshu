# AGENTS.md - Yunshu Go Rewrite Guide

## Mission

This repository is the Go rewrite workspace for Yunshu CallCenter. Build the whole system, not only ESL or CTI. Every implementation must preserve existing external contracts until a deliberate migration contract replaces them.

- **项目中文名称统一**：项目的中文名称叫 **“云枢”**（不是“云舒”），所有对外和对内的中文注释、日志、文档说明、控制台提示词、管理文案中必须统一使用**“云枢”**，请勿使用错误的同音词。

## Reference Source Rules

- Use the  Yunshu CallCenter implementation as the authoritative business reference for FS connection management, CTI/ESL orchestration, event handling, database models, and production behavior.
- Do not treat `yunshu-cmd` as a complete or authoritative implementation. It may only be used as historical context after  code has already been checked.
- When  and `yunshu-cmd` differ, follow  unless the user explicitly approves a new Go-native design.
- FreeSWITCH production node configuration must come from the -compatible database table `freeswitch`; YAML node entries are only a local development fallback when no MySQL DSN is configured.
- API outbound agent extension resolution must come from the unified database table `cc_res_extension` (old `extension` and `kamailio_subscriber` are completely deprecated and merged); request `extra` may only be used as a local fallback when database configuration is absent.
- ESL internal API outbound must keep  `OutboundRequestGuard` semantics: validate merchant user, merchant status and expiry, prepaid balance, and extension availability before sending originate.
- Extension online/busy state must use the -compatible Redis hash `extension:status`; missing/offline, pre-ring, ringing, and talking states must reject API outbound before originate.
- 管理端权限不能只停留在代码常量里。`cc-console` 有数据库时必须优先读取 `console_permission`、`console_role_permission` 和 `console_route_permission`，静态 `PermissionRules` 只能作为迁移期兜底；新增运营管理路由必须同步落权限码 and 数据库种子。

## Database Schema and Table Rules

### 核心设计与废弃表声明 (Core Design & Deprecated Tables)
- **统一分机表模式**：废弃并物理删除独立的 `kamailio_subscriber` 与老 `extension` 表。
- 所有分机鉴权、SIP 参数、密码、商户与用户绑定统一存放在 `cc_res_extension` 中。
- Go 业务侧在修改密码或分机信息时，必须在业务代码中**闭环**自动计算 `ha1` 和 `ha1b` 并写入 `cc_res_extension`，绝对不能依赖外部或 Kamailio 自行计算。
- Kamailio 采用 `use_domain=1` 并通过 `ha1b` 进行域绑定鉴权，物理隔离多租户同名分机冲突。

### 数据库版本控制 (Schema Versioning)
- `version` 表是 Kamailio 启动和鉴权模块的强制依赖，必须播种且包含记录：
  * `('cc_res_extension', 7)` - 对应统一分机表在 Kamailio `auth_db` 中的标准版本校验。
  * `('cc_res_location', 9)` - 对应 Usrloc 动态位置注册表。
  * `('cc_res_freeswitch', 4)` - 对应信令网关负载均衡探测表。
  * `('cc_res_rtpengine', 1)` - 对应媒体代理节点配置表。

### 数据表名及 GORM 模型规范 (Table Names & GORM Models)
进行 Go 重写 and 数据操作时，**必须严格遵循以下物理表名和规则**，严禁私自变更或引入旧表：

1. **统一分机表 `cc_res_extension`**
   - 对应 Go 模型：`internal/infra/resource.ExtensionModel` (表名 `cc_res_extension`)
   - 关键字段：`id`, `extension_number` (分机号/SIP用户名), `password` (明文备份), `sip_domain` (SIP注册域), `ha1` (标准MD5), `ha1b` (带域绑定的MD5), `merchant_id` (商户ID), `user_id` (绑定坐席用户ID), `enable`, `del_flag`
   - 唯一索引：`idx_extension_merchant` (`extension_number`, `merchant_id`)
   - 作用：API 外呼通过 `user_id` 匹配此表以获取分机；Kamailio 注册通过此表进行 HA1b 鉴权。

2. **信令网关探测表 `cc_res_freeswitch`**
   - 对应 Go 模型：`cc_res_freeswitch` 物理表 (由 `internal/infra/telephony/freeswitch.go` 的 `AfterSave` 级联维护，以前叫 `kamailio_dispatcher`)
   - 字段：`id`, `set_id`, `destination` (格式 `sip:host:port`), `flags`, `priority`, `attrs`, `description`, `enable`, `del_flag`
   - 作用：Kamailio 从该表加载媒体节点并做负载均衡和心跳探测。

3. **媒体代理配置表 `cc_res_rtpengine`**
   - 对应 Go 模型：`internal/infra/telephony.RtpengineModel` (表名 `cc_res_rtpengine`，以前叫 `kamailio_rtpengine`)
   - 字段：`id`, `set_id`, `rtpengine_sock` (格式 `udp:host:port`), `disabled`, `weight`, `description`, `del_flag`
   - 作用：Kamailio 从该表加载 RTP 代理地址。

4. **媒体节点配置表 `cc_res_freeswitch_node`**
   - 对应 Go 模型：`internal/infra/telephony.FreeswitchModel` (表名 `cc_res_freeswitch_node`，以前叫 `freeswitch`)
   - 字段：`id`, `address`, `local_address`, `esl_port`, `sip_port`, `password`, `setid`, `weight`, `canary`, `enable`, `del_flag`
   - 作用：Go 后端 CTI 加载此表配置以通过 ESL 控制 FreeSWITCH。

5. **分机动态注册位置表 `cc_res_location`**
   - 对应 Go 模型：无，由 Kamailio `usrloc` 自动读写，旧称 `location`
   - 关键字段：`ruid`, `username`, `domain`, `contact`, `expires`, `user_agent`
   - 作用：保存坐席当前分机动态注册的 NAT 穿透 IP 与端口路由映射。

6. **媒体事件租约表 `cc_res_fs_lease`**
   - 对应 Go 模型：`internal/infra/telephony.FreeswitchEventLeaseModel` (表名 `cc_res_fs_lease`，以前叫 `freeswitch_event_lease`)
   - 字段：`fs_addr` (主键), `owner` (实例持有者), `lease_expiry` (租约过期时间)
   - 作用：`cc-call` 多实例高可用消费 FS 事件的租约表，防止重复消费。

6. **控制台账号与权限表 (Console Account & Permissions)**
   - 账号表：`console_account` (唯一登录与账号控制，支持 `merchant_id` 物理隔离)
   - 权限定义表：`console_permission`
   - 角色权限表：`console_role_permission`
   - 路由权限表：`console_route_permission`

7. **商户与计费表 (Merchant & Billing)**
   - 商户表：`merchant` (包含 `whitelist_domains` 域名白名单以做防刷/鉴权)
   - 坐席用户表：`merchant_user`
   - 计费总览表：`merchant_billing_overview`
   - 充值流水表：`merchant_billing_recharge`

8. **业务选号与策略表 (Number Selection & Routing)**
   - 号码池表：`pool_phone`
   - 号码技能组关系表：`pool_phone_skill_group`
   - 技能组表：`skill_group`
   - 坐席技能组绑定表：`user_skill_group`
   - 通道网关配置表：`channel`, `gateway`, `pool`

9. **黑白名单表 (Blacklist & Whitelist)**
   - 黑名单库：`blacklist`，网关映射：`blacklist_gateway`
   - 白名单库：`whitelist_data`，商户映射：`whitelist_data_merchant`

10. **分布式与异步流程持久化表 (Durable Workflow & Async Outbox)**
    - 事务型信箱表：`message_outbox` (实现高并发推送、WebSocket、回调、CDR 发布等幂等可靠投递)
    - 呼叫话单记录表：`call_cdr_record` (ESL 挂机后可靠落盘的话单事实)
    - 计费账单表：`call_billing_ledger`
    - 结算任务表：`call_billing_settlement_job`
    - 录音上传任务表：`call_recording_job`
    - 话单投影表：`call_report_projection`
    - 下游推送任务表：`call_downstream_push_job`

## Migration Thinking Rules

- Do not migrate by mechanically translating  classes, packages, or controller names. Before implementing a slice, write down the business closure, production risk, Go-native boundary, compatibility requirement, and required tests.
- Prefer business capability slices over  module slices. A slice should be something production can reason about end to end, such as API outbound, batch outbound scheduling, CDR finalization, recording upload, WebSocket projection, gateway sync, or billing.
- For every  feature, decide explicitly whether to: keep contract-compatible behavior, redesign internally with Go boundaries, defer behind a port, or remove as obsolete. Document the decision in `docs/go-rewrite/MIGRATION_DECISIONS.md` or the relevant design doc.
- Avoid copying  layering mistakes. Keep orchestration in domain/application services, external systems in `internal/infra`, Gin binding in `internal/transport`, and compatibility DTOs in `internal/contracts`.
- When a feature depends on missing infrastructure, add a small port and a logged fallback only if local development needs it; production behavior must fail closed for billing, authorization, FS connection, customer privacy, and call-state correctness.
- Each migration slice must include Chinese logs, Chinese comments for business semantics, focused tests, and documentation updates before handoff.
- -compatible schemas are not automatically final schemas. If a table name or table structure is unreasonable for Go production needs, high concurrency, operational safety, or long-term maintenance, design a replacement or supplemental schema and document the migration path before implementation.
- New Go-native table names are allowed when they improve clarity, ownership, scalability, or operational safety. When renaming, document compatibility views/adapters, backfill, dual-write or sync strategy, rollback, and the final cutover plan.
- Table name optimization is encouraged during redesign. Prefer names that clearly express bounded context, aggregate ownership, and lifecycle purpose instead of blindly preserving legacy  names such as overly long, ambiguous, or module-leaking table names.
- Before using direct database queries in a hot path, explicitly evaluate whether DB reads/writes can satisfy expected concurrency, latency, lock contention, and failure-isolation requirements. If not, design Redis/cache/materialized views/sharding/outbox/async refresh instead of forcing synchronous DB access.
- Number selection must be redesigned as a high-concurrency capability, not copied as ad hoc DB query logic. The design must consider candidate preloading, Redis atomic allocation, concurrency counters, gateway/node health, blacklist/risk filters, weighted selection, fallback/release semantics, idempotency, and auditability.
- For number selection and other hot allocation paths, database tables should be treated as source-of-truth/configuration where appropriate; runtime selection should prefer precomputed/cache-backed structures with atomic updates unless documented load testing proves direct DB access is sufficient.

## Architecture Rules

- Keep deployable service boundaries explicit: `cc-edge`, `cc-console`, `cc-call`, and `cc-worker`.
- Keep code domain boundaries explicit even when deployment is consolidated: API, merchant, operate, CTI, ESL, and worker logic must stay in separate domain/transport packages.
- Keep directory boundaries explicit: `cmd` for process entrypoints, `internal/app` for assembly, `internal/domain` for pure business logic, `internal/transport` for Gin/MQ handlers, `internal/infra` for external adapters, and `internal/contracts` for compatibility contracts.
- Put shared DTOs, event payloads, Redis keys, MQ names, enums, and error codes in `internal/contracts`.
- Use `contracts.EventEnvelope` for durable events and cross-service workflow events.
- Register externally visible error codes in `internal/contracts/errors.go`.
- Keep CTI responsible for business scheduling, number selection, task state, WebSocket projection, CDR persistence, billing, callbacks, and Kamailio integration.
- Keep ESL responsible for FreeSWITCH command execution, event adaptation, call/channel lifecycle, bridge relations, recordings, terminal events, and CDR publication.
- Management surfaces are first-class. Merchant and operate operations that affect runtime behavior must refresh or publish the correct cache/update event.
- Number selection source tables are not read-only artifacts. `channel`, `gateway`, `pool`, `pool_phone`, `skill_group`, `pool_phone_skill_group`, and `user_skill_group` all need real management surfaces, not just runtime readers, so operators can configure the full selection chain without raw SQL.
- 多租户分机鉴权必须支持多域物理隔离。分机表 `cc_res_extension` 与商户 `merchant` 表均必须包含 `sip_domain` 属性。在 SIP 注册鉴权时，Kamailio 默认启用 **HA1b** 密文鉴权方案，避免在多租户下不同商户因为 `1001` 等同名分机引发冲突。
- **HA1** 与 **HA1b** 计算规则必须在 Go 业务层代码中闭环：当分机密码、分机号或所属商户发生变化时，Go 服务端必须自动重新计算哈希并写入数据库：
  * **HA1** = `MD5(username:realm:password)`
  * **HA1b** = `MD5(username@domain:realm:password)`，以支持域绑定验证。
  * 严禁明文密码被外部直接读取，统一采用 `ha1b` 校验方案来与 Kamailio `auth_db` 对接。

## Multi-Instance and High-Concurrency Rules

- Every state-changing command needs an idempotency key or command id.
- Every async consumer must document ack timing, retry, dead-letter or repair behavior, and observability.
- Do not use Redis Pub/Sub as final business truth. Critical state needs DB, durable queue, stream, outbox, or replayable audit.
- 多实例 worker 不能直接裸扫 outbox 表后投递。生产投递必须先通过 `ClaimDue` 或等价机制领取租约，写入 `processing`、`locked_by`、`locked_until`，并在下游确认后才标记 `published`；租约过期后允许重领以恢复崩溃 worker。
- WebSocket、回调和报表投影必须携带租户/商户作用域。WebSocket fanout 在写入 socket 前必须按 `merchantId`、`taskId` 等订阅条件过滤，不能做跨商户全量广播；本地调试的无过滤订阅不能作为生产认证方案。
- CDR 是计费、录音、报表和外部推送的收口事实。ESL 只能在最终事件写 `call_center_cdr_queue` outbox，worker 必须先幂等持久化 `call_cdr_record`，后续节点再基于 CDR 事件继续编排。
- CDR payload 必须携带后续流程需要的业务上下文，包括 `merchantId`、`userId`、批量任务/号码 ID 和录音路径；不要让计费、报表或录音修复节点在高并发下回查热会话补上下文。
- 计费必须拆成独立 workflow 节点。`cti_cdr_billing` 只能先幂等写入计费流水和状态；费率计算、套餐抵扣、余额扣减、发票/结算通知必须各自有事件、幂等键、补偿和审计记录，不能藏在 CDR 消费器里一次性扣费。
- 计费默认费率也必须来自配置，不要写死在代码里。`worker.billing.defaultRatePerMin` 和 `WORKER_BILLING_DEFAULT_RATE_PER_MIN` 只是默认估算值，生产环境如果未配置应有明显中文告警，且只能生成审计用的 `rated` 估算，不允许直接当作最终扣费结果。
- `cti_billing_settlement` 是计费后半段的独立节点，必须先写结算 job，再做余额扣减；如果 `merchant_billing_overview` 不存在，要显式记录为 no-op 结算，而不是把它当成成功扣费。
- 录音处理必须拆成独立 workflow 节点。`cti_cdr_recording` 应先幂等写入录音任务；缺少录音路径要记录为可修复状态，不能只打一行日志后丢失。
- 配置 `RECORDING_UPLOAD_URL` 后，录音上传只能在上传端确认后标记 `uploaded`；失败要写入 `failed/last_error` 并让 outbox 重试，不能吞掉异常或提前认为录音已处理。
- 报表和下游 CDR 推送必须从 CDR 持久化事实继续编排。`cti_cdr_report_projection` 负责查询投影，`cti_cdr_downstream_push` 负责下游投递任务；不要让控制台查询扫 outbox 热表，也不要在 CDR consumer 中直接同步推送所有下游。
- 下游 CDR 推送必须有任务状态和确认语义。配置 `DOWNSTREAM_CDR_URL` 后，worker 只能在下游确认后标记 `delivered`；失败要写入 `failed/last_error` 并让 outbox 重试，不允许吞掉异常。
- FreeSWITCH event consumption must have node ownership or shard ownership. Do not allow two Go ESL instances to consume the same FS node stream without a lease.
- `cc-call` 在注册 FS 事件监听前必须先通过注册表领取事件租约，在连接存活期间持续续约，断开或停机时释放；如果租约已被其他实例持有，当前实例必须跳过该节点而不是重复消费。续约失败时要 fail-closed，不能继续假装自己仍然是 owner。
- 振铃音和早期媒体也必须进入流程编排。`CHANNEL_PROGRESS_MEDIA` 不能只当作事件日志，它要作为 ESL workflow 的显式 ringback/early-media 阶段处理，且 originate 计划必须透传 `supplementRing`、`supplementRingFile`、`broadcastTime` 和 `broadcastTimeFlag` 等元数据，保证后续桥接、停止播放和排障都能从 workflow 变量中追踪。`CHANNEL_PROGRESS` 只表示振铃，不得直接桥接；API 外呼只有在坐席腿已应答且客户腿进入 `CHANNEL_PROGRESS_MEDIA` 或 `CHANNEL_ANSWER` 后才允许执行桥接。
- Concurrency counters, locks, and resource allocation must use atomic storage semantics when backed by Redis or DB.
- CTI 运行时选号在高并发下必须逐个尝试经过规则链的候选号码；单个候选号并发满了不代表整单失败，只有所有候选都失败后才能返回 `ErrNoAvailableNumber`。
- Logs for telephony operations must include `callId`, `uuid`, `fsAddr`, leg role, command id, and failure reason. Never log raw customer phone numbers unless a business audit explicitly requires it.
- Use `internal/infra/logging` helpers for HTTP and telephony logs. Do not invent new field names for request id, trace id, call id, FS address, command id, or event id.

## Implementation Rules

- Prefer small domain packages over a large `common` package.
- Use Gin for HTTP transport and GORM for production database adapters when they speed up delivery, but keep domain packages independent from transport and ORM types.
- Use `context.Context` on all I/O and domain entry points that can be called by HTTP, MQ, scheduled jobs, or event consumers.
- Prefer workflow definitions and event transitions for multi-step business behavior. Avoid growing controller, consumer, or lifecycle `if/else` branches when a workflow event can express the path.
- 所有多步骤业务必须按“流程节点 + 事件推进”建模，包括入口校验、资源占用、选号、ESL 起呼、FS 事件、桥接、录音、CDR、计费、回调、WebSocket/消息推送、批量下一号码调度、失败补偿和人工修复。HTTP handler、定时任务、MQ/Stream consumer 只能触发节点或发布事件，不能私自承载完整业务流程。
- 每个流程节点必须有稳定的事件名、状态迁移、幂等语义、失败语义、重试/补偿策略和中文日志。新增节点时同步更新 `docs/WORKFLOWS.md`，说明上游事件、下游事件、外部副作用和可观测字段。
- 最后的消息推送也必须流程化：WebSocket、客户回调、报表投影、通知和第三方 webhook 都应由事件消费者或 workflow step 触发，并通过 outbox/Redis Stream 等可重放机制保证失败可查、可重试、可补偿。
- Return compatibility `Result` envelopes from HTTP handlers while the  contract is preserved.
- Keep Chinese business error messages when they are externally visible.
- 中文注释和中文日志是强制要求。新增或修改代码时，包级注释、导出类型/接口/函数注释、关键状态流转、幂等、租约、重试、补偿、外部副作用说明必须使用中文。
- 日志消息必须使用中文，结构化字段名保持英文稳定字段，例如 `requestId`、`traceId`、`callId`、`fsAddr`、`commandId`。不要在不同模块里发明同义字段。
- 关键路径必须有完整日志：入口、参数/上下文识别、幂等命中、状态迁移、资源占用/释放、外部系统调用、重试、补偿、失败原因和最终结果都要可检索。
- 修改现有代码时也要补齐中文注释和中文日志，不能只要求新文件满足规范。
- 注释要解释业务意图、边界和失败语义，不要写“给变量赋值”这类机械注释。
- Add focused tests for state machines, idempotency, ownership, route contracts, and replay reducers.
- Update docs when adding new public contracts, Redis keys, MQ queues, or data schemas.
- Before adding repositories or migrations, update `docs/DATA_MIGRATION.md` or a schema registry document.

## Verification

Run before handoff:

```bash
gofmt -w .
go test ./...
go vet ./...
```

## Automatic Updates

When HTTP, Redis, or MQ contracts change, run:

```bash
go run ./cmd/update-agents
```

The command updates the generated summary below from `internal/contracts`.

<!-- BEGIN AUTO-GENERATED CONTRACT SUMMARY -->

## Auto-Generated Contract Summary

This section is generated from `internal/contracts`. Run `go run ./cmd/update-agents` after contract changes.

- HTTP route contracts: 36
- Redis contracts: 17
- MQ contracts: 7

Service route counts:

- `cc-edge`: 5
- `cc-console`: 22
- `cc-call`: 9
- `cc-worker`: 0

<!-- END AUTO-GENERATED CONTRACT SUMMARY -->
