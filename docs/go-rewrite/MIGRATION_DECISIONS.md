# Migration Decisions

Go 迁移不是逐个翻译  类。每个功能块开工前必须先判断业务闭环、生产风险、Go 边界和兼容要求。

## Decision Matrix

| Capability |  reference | Go decision | Why | Next slice |
| --- | --- | --- | --- | --- |
| API outbound | `ApiCallService`, `OutboundRequestGuard`, `OriginatePlanBuilder` | Keep external contract, Go-native domain + infra ports | 已是最短生产主链路，必须先跑通 FS | Continue hardening CDR and callback |
| Batch outbound | `BatchCallController`, `BatchCallTaskServiceImpl`, merchant state machine | Keep contract, redesign as workflow + scheduler ports |  分散在 merchant/CTI/ESL，多 if/else，Go 需要事件编排 | Add retry/release, richer statistics, and real projection delivery workers |
| FS node management | `FreeswitchNodeService`, `FreeswitchRegistry` | Keep DB schema and operational behavior, Go connection pool plus operate management facade | 多 FS 节点和动态管理是生产前提；管理端修改配置真相，呼叫端刷新运行时连接池 | Add MQ/cache refresh event and health auto-disable later |
| FS event handling | `FsDomainEventAdapter`, channel event consumers | Keep event meaning, Go adapter + reducers | 事件是流程编排核心，不能散在控制器 | Add bridge/recording/CDR consumers |
| CDR finalization | `ChannelHangupCompleteEventConsumer`, CTI CDR consumers | Keep final data semantics, Go outbox + DB repositories | 计费、回调、报表依赖 CDR | Implement call_record model and CDR publisher |
| Recording upload | recording services, OSS config | Port first, implementation later | 依赖对象存储和文件路径策略 | Define recording job contract |
| WebSocket projection | CTI websocket handlers and cluster push | Redesign as Redis projection + lightweight fanout | Go 服务合并后仍需多实例推送；推送不能承载最终真相 | Add auth, tenant filtering, and connection sharding |
| Kamailio sync | dispatcher/subscriber services | Keep operational contracts, isolate infra | 影响 SIP 注册和路由 | Dispatcher management slice implemented; subscriber slice implemented; next add real Kamailio reload/sync port and event-driven subscriber propagation |
| Gateway/number selection | CTI select rule services, operate gateway advice | Keep DB contracts, add operate gateway management, cc-console to cc-call sync client, -compatible `/esl/gateway` FS cmd sync, and -compatible candidate source adapter; redesign runtime selection separately | 影响接通率和线路成本；管理面可以写  表，呼叫热路径必须走缓存/原子投影 | Replace placeholder selector with gateway/pool projection |
| Rate management | `RateManageController`, `RateManageServiceImpl`, `CallRateDO`, `CallRateMerchantDO` | Keep  `call_rate` and `call_rate_merchant` contracts on the operate surface; defer runtime rating switch to billing workflow nodes | 网关 `rate_id` 和商户费率绑定已经是现网配置真相，先补齐主数据和绑定链，再迁移正式计费结算 | Next add merchant billing overview management and replace default-rate estimate with rate lookup |
| Blacklist management | `BlacklistController`, `BlacklistManageServiceImpl`, `BlacklistDO`, `BlacklistGatewayDO` | Keep  `blacklist` and `blacklist_gateway` contracts on the operate surface; defer `blacklist_data` runtime filtering integration to the next selection slice | 运营端需要先能维护黑名单库与网关忽略关系，运行时被叫过滤再接入选择链 | Next add `blacklist_data` / whitelist runtime lookup and merchant risk binding |
| Whitelist management | `WhiteListDataController`, `WhiteListDataManageServiceImpl`, `WhitelistDataDO`, `WhitelistDataMerchantDO` | Keep  `whitelist_data` and `whitelist_data_merchant` contracts on the operate surface; defer runtime `WhitelistHit` projection to the next selection slice | 运营端需要先能维护号码白名单及商户绑定，后续才能把白名单优先级接回选号链 | Next add merchant whitelist-domain field exposure and runtime whitelist candidate marking |
| Merchant whitelist domains | `MerchantManageAdvice`, `MerchantCreateDTO.whitelistDomains`, `MerchantVO.whitelistDomains` | Keep  `whitelist_domains` as a merchant configuration field exposed through Go merchant CRUD | 它是商户接入安全的域名/IP 加白配置，不是号码白名单；应该随着商户主体管理一起读写 | Next add dedicated validation and audit log detail for whitelist-domain changes |
| Selection runtime markers | `RiskWhitelistSelectionRule`, `BlacklistSelectionRule`, `SelectionRuleTypeMatrixTest` | Keep  selection semantics at the Go selector boundary by marking candidates before rule execution | 黑白名单必须真正影响选号结果，而不是停留在管理表；但必须保留号码可用性和并发占用等物理约束 | Next refine gateway-specific blacklist scope and candidate cache projection |
| Risk-id propagation | `SelectRuleReq.riskId`, `RiskBlacklistSelectionRule`, `RiskControlDO` | Keep  `riskId` contract visible in Go runtime selection and use it to resolve merchant blacklist blocking | 风控 ID 不能只停留在 HTTP DTO；Go 运行时必须能感知它，哪怕先只接黑名单等级这一段 | Next connect blind-area and callee-frequency risk branches behind the same selection marker |
| Runtime selection cache | `RedisCandidateSource`, `RuntimeSelectionMarker` | Keep marker results cached in-process with TTL while source-of-truth stays in MySQL/Redis | 选择前的标记查询很热，缓存只做加速，不改变最终事实；失效后必须可回源重建 | Next decide whether to project marked candidates into Redis for cross-instance reuse |
| Merchant billing management | `MerchantBillingController`, `MerchantBillingServiceImpl`, `MerchantBillingOverviewDO`, `MerchantBillingRechargeDO` | Keep  `merchant_billing_overview` and `merchant_billing_recharge` contracts on the operate surface; keep actual call charging in worker workflow nodes | 运营端需要能查询账单总览、维护支付方式/信用额度、做余额调整，但不能把 CDR 结算逻辑搬回控制台 | Next expose billing detail/audit records and reconcile with settlement workflow |
| Merchant/operate admin | merchant/operate controllers | Keep HTTP compatibility, implement by bounded modules | 管理面大，不能阻塞呼叫主链路；已从 operate FreeSWITCH 节点管理开始落地 | Merchant lifecycle, batch task/dialpad, call record, AI flow, management auth, and route-level functional permissions are now implemented. Protected management routes now require token access. AI flow now uses a Go-native persistent table. Next focus: identity provider, richer merchant workflows, and operational sync polish |
| Import/export/model jobs | merchant async consumers | Defer behind worker ports | 非呼叫实时主链路 | Implement after core call lifecycle |

## Rules For Each Slice

- Write the compatibility DTO in `internal/contracts` first.
- Add a domain service or workflow definition before transport handlers.
- Keep DB/Redis/FS/RabbitMQ in `internal/infra`.
- Do not assume  table design is final. When a table name or schema cannot support Go production clarity, concurrency, or correctness, design a replacement or supplemental schema and document source-of-truth, compatibility views/adapters, sync, backfill, cutover, and rollback.
- Hot paths must include a concurrency assessment before implementation. Direct DB queries are acceptable only after evaluating QPS, latency, locks, index coverage, transaction scope, and failure isolation.
- Add Chinese logs at entry, decision, external call, failure, and final result.
- Add focused tests for state transitions, idempotency, validation, and route compatibility.
- Update this document when a feature is kept, redesigned, deferred, or removed.

## Workflow-Orchestration Rule

所有迁移后的多步骤业务都必须拆成流程节点，并通过事件推进。消息推送、WebSocket 投影、第三方回调、CDR 发布、计费通知和报表投影也属于流程节点，必须由 workflow step、Redis Stream/RabbitMQ consumer 或 outbox publisher 触发。控制器、定时器和 MQ consumer 只能做协议适配、上下文提取、事件发布或 workflow apply，不能把完整业务流程写成私有 if/else。

## Number Selection Redesign

Number selection is a high-concurrency allocation problem and must not be migrated as scattered synchronous SQL queries.

Initial design direction:

-  tables remain source-of-truth for configuration, ownership, gateway relations, risk/blacklist data, and audit history.
- Runtime candidate pools should be preloaded or incrementally refreshed into Redis/materialized structures.
- Allocation should use atomic Redis/Lua or another atomic store for caller/gateway/task counters and idempotent claims. Current Go baseline provides `internal/infra/selection.RedisAllocator` for call-level claim/release.
- Selection pipeline should be modeled as explicit stages: eligibility, risk/blacklist filtering, gateway health, weight calculation, concurrency claim, audit, release/compensation.
- Release must be idempotent and safe when calls fail before FS originate, fail after originate, or complete normally.
- Direct DB selection is only allowed for admin/repair/low-QPS paths or after load testing proves it is safe.
- The Go implementation should expose a domain port for source-of-truth data and a separate runtime allocator port, so database schema redesign does not leak into controllers.
