# 云枢呼叫流程技术文档 (Call Flow Reference)

本文档详细描述 yunshu-phone（云枢软电话客户端）发起的**呼出（拨号盘直呼）**和**呼入（客户呼入）**两条核心话务信令流程。

---

## 1. 系统话务拓扑总览

```text
┌──────────────┐    SIP      ┌───────────┐  dispatcher  ┌────────────┐   ESL    ┌──────────┐
│ yunshu-phone │◄───────────►│  Kamailio  │────────────►│ FreeSWITCH │◄────────►│  cc-call  │
│ (拨号盘客户端) │  REGISTER   │  (SIP代理)  │   INVITE    │  (媒体服务器) │          │ (Go应用)  │
│              │  INVITE     │  5060/5066  │             │            │          │          │
└──────────────┘             └───────────┘             └────────────┘          └──────────┘
                                                                        │
                                                                        │ Event Bus
                                                                        ▼
                                                              ┌──────────────────┐
                                                              │ CTI / ESL Runner │
                                                              │ (工作流状态机)     │
                                                              └──────────────────┘
```

### 服务职责

| 组件 | 职责 |
|---|---|
| **Kamailio** | SIP 注册/鉴权，dispatcher 负载均衡到 FreeSWITCH，RTPEngine NAT 穿越 |
| **FreeSWITCH** | B2BUA 媒体服务器，执行 originate/bridge/hangup/playback，通过 park 等待 cc-call 编排 |
| **cc-call** | Go 话务引擎，消费 FS ESL 事件，驱动 CTI/ESL 工作流，管理会话和选号 |

### 相关代码索引

| 层级 | 文件 | 职责 |
|---|---|---|
| **ESL 工作流** | `internal/domain/esl/workflows.go` | 定义 `esl_dialpad_direct` 和 `esl_inbound` 状态机 |
| **会话管理** | `internal/domain/esl/session.go` | `ApplyEvent()` 嗅探并创建会话，驱动生命周期 |
| **起呼编排** | `internal/domain/esl/originate.go` | `StartDialpadCustomerOutbound()` / `StartInboundAgentOutbound()` |
| **事件消费** | `internal/domain/callflow/consumer.go` | 核心路由：分发 FS 事件到对应 handler |
| **会话嗅探** | `internal/infra/resource/session_sniffer.go` | `IsExtension()` / `IsMerchantDID()` 识别来电类型 |
| **坐席分配** | `internal/infra/resource/inbound_agent_resolver.go` | DID → 技能组 → 空闲坐席查询 |
| **FS 入口** | `internal/infra/fsesl/event_adapter.go` | ESL 事件 → `TelephonyEvent` 转换 |
| **命令构建** | `internal/infra/fsesl/command_builder.go` | 领域命令 → FreeSWITCH API 指令 |
| **拨号盘 API** | `internal/transport/http/console/operate/dialpad_compat_routes.go` | 拨号盘登录和 SIP 凭证接口 |

---

## 2. 呼出流程 — 拨号盘直呼 (`esl_dialpad_direct`)

### 2.1 触发方式

坐席在 yunshu-phone 拨号盘输入客户号码并发起呼叫 → SIP INVITE → Kamailio → dispatcher 转发 FreeSWITCH → dialplan `01_yunshu_inbound.xml` 匹配并 `&park()` → `CHANNEL_CREATE` 事件到达 cc-call。

### 2.2 会话自动识别

当 `CHANNEL_CREATE` 到达但会话不存在时，`SessionService.ApplyEvent()` 调用 `SessionSniffer` 自动识别来电类型：

```text
CHANNEL_CREATE 到达
  ├── 提取 callerNumber（主叫号码）
  ├── Sniffer.IsExtension(callerNumber)
  │     查询 cc_res_extension 表
  │     ├── 匹配成功 → 识别为 api_direct（坐席直呼）
  │     │     Profile = CallFlowAPIDirect
  │     │     LegRole = Agent
  │     │     Metadata: {userId, merchantId, extension, agentUuid, caller, callee}
  │     └── 不匹配 → 继续检查 calleeNumber（被叫号码）
  │           Sniffer.IsMerchantDID(calleeNumber)
  │           查询 cc_res_pool_phone → cc_tel_pool
  │           ├── 匹配 → 识别为 inbound（客户呼入）
  │           └── 不匹配 → 丢弃事件
```

**识别入口：** `internal/domain/esl/session.go:179-246`

### 2.3 ESL 状态机

**工作流 ID：** `esl_dialpad_direct`（`internal/domain/esl/workflows.go:150-189`）

```text
init ──CHANNEL_CREATE──► agent_created
  │                        ├── CHANNEL_PROGRESS ──► agent_progress
  │                        ├── CHANNEL_PROGRESS_MEDIA ──► agent_progress_media
  │                        ├── CHANNEL_ANSWER ──► agent_answered
  │                        └── CHANNEL_HANGUP_COMPLETE ──► complete
  │
  │  (坐席摘机后触发选号和客户腿起呼)
  │
agent_answered ──validate_command──► customer_validated
customer_validated ──execute_originate──► customer_originating
customer_originating ──CHANNEL_CREATE──► customer_created
  │                                      ├── CHANNEL_PROGRESS ──► customer_progress
  │                                      ├── CHANNEL_PROGRESS_MEDIA ──► customer_early_media
  │                                      ├── CHANNEL_ANSWER ──► customer_answered
  │                                      └── CHANNEL_HANGUP_COMPLETE ──► complete
  │
customer_answered / customer_progress / customer_early_media
  └── CHANNEL_BRIDGE ──► bridged
bridged ──CHANNEL_HANGUP_COMPLETE──► complete
```

### 2.4 事件处理序列

| 步骤 | 触发事件 | 处理函数 | 关键动作 |
|:---:|---|---|---|
| 1 | Phone INVITE → FS `CHANNEL_CREATE` | `SessionService.ApplyEvent()` | Sniffer 识别分机号，创建 `api_direct` 会话，`LegRole=Agent`，发布 `fs-applied` 事件 |
| 2 | `fs-applied` → ESL Runner | `consumer.go` 订阅 | 工作流从 `init` → `agent_created` |
| 3 | FS `CHANNEL_ANSWER`（坐席摘机） | `handleDialpadAgentAnswer()` | ① 检查 `customerOriginateSent` 幂等 ② `loadAPICandidates()` 加载可选号码 ③ `RuntimeSelector.SelectAndClaim()` 选号并占并发槽位 ④ `StartDialpadCustomerOutbound()` 发起客户腿 originate |
| 4 | 选号失败安全收口 | `hangupAgent()` 闭包 | 下发 `hangup` 命令切断坐席通道，释放并发槽位 |
| 5 | FS `CHANNEL_CREATE`（客户腿） | `ApplyEvent()` | UUID 映射到 `LegRole=Customer` |
| 6a | FS `CHANNEL_PROGRESS`（客户 180 振铃） | `handleDialpadCustomerProgress()` | 向坐席腿播放补振铃音（`MediaOrchestrator`） |
| 6b | FS `CHANNEL_PROGRESS_MEDIA`（客户 183 早期媒体） | `handleDialpadCustomerEarlyMedia()` | 停止补振铃，标记 `carrierEarlyMedia=true`，标记客户就绪，尝试桥接 |
| 7 | FS `CHANNEL_ANSWER`（客户接听） | `handleDialpadCustomerReady()` | 停止补振铃，标记 `customerReady=true` |
| 8 | 两腿均就绪 | `maybeBridgeDialpadDirect()` | `bridgeGuard` CAS 防重 → FS `uuid_bridge` 合并通话 |
| 9 | FS `CHANNEL_HANGUP_COMPLETE` | `ApplyEvent()` | ① 写 CDR outbox ② 释放运行时选号并发槽位 ③ 5s ACW 冷却后恢复分机 IDLE ④ 检查排队队列 |

### 2.5 关键设计特性

- **Agent-First 语义**：先呼坐席分机，坐席摘机后再选号呼客户
- **选号失败安全收口**：`hangupAgent()` 闭包确保坐席不会无限等待
- **补振铃音编排**：`MediaOrchestrator` 管理 `broadcastTime` 定时截断，运营商 183 到达自动停播
- **bridgeGuard 防重**：`sync.Map.LoadOrStore` 确保 `uuid_bridge` 只执行一次
- **ACW 冷却**：坐席挂断后 5 秒冷却期，防止二次派单；冷却后检查排队队列拉取等待呼叫
- **幂等保护**：`customerOriginateSent` / `lastEventID` 防止重复起呼和重复事件

---

## 3. 呼入流程 — 客户呼入 (`esl_inbound`)

### 3.1 触发方式

外部客户拨打商户 DID 号码 → SIP INVITE 到达 Kamailio(5060) → 鉴权通过 → dispatcher 负载均衡到 FreeSWITCH → 匹配 dialplan `01_yunshu_inbound.xml` → `answer` + `park()` → `CHANNEL_CREATE` 事件到达 cc-call。

### 3.2 会话自动识别

```text
CHANNEL_CREATE 到达
  ├── Sniffer.IsExtension(callerNumber) → false（外部客户号码不是分机号）
  └── Sniffer.IsMerchantDID(calleeNumber)
        查询 cc_res_pool_phone → cc_tel_pool
        ├── 匹配 → 识别为 inbound（客户呼入）
        │     Profile = CallFlowInbound
        │     LegRole = Customer
        │     Metadata: {merchantId, customerUuid, caller, callee}
        └── 不匹配 → 丢弃事件
```

### 3.3 ESL 状态机

**工作流 ID：** `esl_inbound`（`internal/domain/esl/workflows.go:190-231`）

```text
init ──CHANNEL_CREATE──► customer_created
  │                        ├── CHANNEL_PROGRESS ──► customer_progress
  │                        ├── CHANNEL_PROGRESS_MEDIA ──► customer_early_media
  │                        ├── CHANNEL_ANSWER ──► customer_answered
  │                        └── CHANNEL_HANGUP_COMPLETE ──► complete
  │
  │  (客户应答后触发坐席查找和起呼)
  │
customer_* ──validate_command──► agent_validated
agent_validated ──execute_originate──► agent_originating
agent_originating ──CHANNEL_CREATE──► agent_created
  │                                    ├── CHANNEL_PROGRESS ──► agent_progress
  │                                    ├── CHANNEL_PROGRESS_MEDIA ──► agent_early_media
  │                                    ├── CHANNEL_ANSWER ──► agent_answered
  │                                    └── CHANNEL_HANGUP_COMPLETE ──► complete
  │
agent_answered ──CHANNEL_BRIDGE──► bridged
bridged ──CHANNEL_HANGUP_COMPLETE──► complete
```

### 3.4 事件处理序列

| 步骤 | 触发事件 | 处理函数 | 关键动作 |
|:---:|---|---|---|
| 1 | 客户 INVITE → FS `CHANNEL_CREATE` | `SessionService.ApplyEvent()` | Sniffer 识别 DID，创建 `inbound` 会话，`LegRole=Customer`，发布 `fs-applied` 事件 |
| 2 | `fs-applied` → ESL Runner | `consumer.go` 订阅 | 工作流从 `init` → `customer_created` |
| 3 | FS `CHANNEL_ANSWER`（dialplan answer + park） | `handleInboundCustomerAnswer()` | 见下方详细分支（含无坐席安全挂断） |
| 4 | FS `CHANNEL_CREATE`（坐席腿） | `ApplyEvent()` | UUID 映射到 `LegRole=Agent` |
| 5 | FS `CHANNEL_PROGRESS`（坐席振铃） | `handleInboundAgentProgress()` + 分机状态更新 | ① 向客户播放回铃音 ② Redis `extension:status` → `Ringing` |
| 6 | FS `CHANNEL_ANSWER`（坐席摘机） | `handleInboundAgentReady()` | 停止回铃音，标记 `agentAnswered=true`，触发 `maybeBridgeInbound()` |
| 7 | 两腿均就绪 | `maybeBridgeInbound()` | `bridgeGuard` CAS 防重 → FS `uuid_bridge` 合并通话 |
| 8 | FS `CHANNEL_HANGUP_COMPLETE` | `ApplyEvent()` | 写 CDR outbox，释放资源，ACW 冷却 |

### 3.5 坐席分配链路

`handleInboundCustomerAnswer()`（`consumer.go:1883-1977`）的处理分支：

```text
客户应答 (CHANNEL_ANSWER, legRole=Customer)
  │
  ├── 检查 metadata["aiEnabled"]
  │     └── true → AIVoiceEngine.StartAIVoiceFlow()
  │           ├── 成功 → 标记 aiFlowActive，返回
  │           └── 失败 → 降级回退到人工坐席分配
  │
  ├── 检查 metadata["agentOriginateSent"] → 已发送则跳过
  │
  ├── 检查 metadata["extension"]（已有绑定分机）
  │     ├── 不为空 → 直接使用该分机
  │     └── 为空 → InboundAgentResolver.ResolveForDID(did, merchantId)
  │           │
  │           │  SQL 查询链路:
  │           │  cc_res_pool_phone → cc_res_pool_phone_skill_group
  │           │    → cc_res_skill_group → cc_res_user_skill_group
  │           │    → cc_res_mch_user → cc_res_extension
  │           │
  │           ├── 找到空闲坐席 (Redis status == IDLE)
  │           │     → StartInboundAgentOutbound() 起呼坐席分机
  │           └── 无空闲坐席
  │                 → logger.Warn(...) 并 return nil  ⚠️ 客户被留在 park 中
  │
  └── 坐席起呼成功
        → 标记 agentOriginateSent, 发布 EventESLCommandSent
```

### 3.6 坐席分配查询（InboundAgentResolver）

**代码位置：** `internal/infra/resource/inbound_agent_resolver.go:41-95`

```sql
SELECT u.id AS user_id, u.merchant_id, u.seat_number, e.extension_number
FROM cc_res_pool_phone pp
INNER JOIN cc_res_pool_phone_skill_group ppsg ON ppsg.pool_phone_id = pp.id
INNER JOIN cc_res_skill_group sg ON sg.id = ppsg.skill_group_id AND sg.enable = 1 AND sg.del_flag = 0
INNER JOIN cc_res_user_skill_group usg ON usg.skill_group_id = sg.id
INNER JOIN cc_res_mch_user u ON u.id = usg.user_id AND u.enable = 1 AND u.del_flag = 0
INNER JOIN cc_res_extension e ON e.user_id = u.id AND e.enable = 1 AND e.del_flag = 0
WHERE pp.phone = ? AND pp.enable = 1 AND pp.del_flag = 0 AND u.merchant_id = ?
ORDER BY u.id ASC
```

查询后逐个检查 Redis `extension:status`，返回第一个 `IDLE` 的坐席。

---

## 4. 两条流程对比

| 维度 | 呼出（拨号盘直呼） | 呼入（客户呼入） |
|---|---|---|
| **发起方** | 坐席（yunshu-phone） | 外部客户 |
| **触发方式** | 坐席在拨号盘拨号 | 客户拨打商户 DID |
| **会话识别** | Sniffer 匹配 caller → `cc_res_extension` | Sniffer 匹配 callee → `cc_res_pool_phone` |
| **Profile** | `CallFlowAPIDirect` | `CallFlowInbound` |
| **ESL 工作流** | `esl_dialpad_direct` | `esl_inbound` |
| **Leg A（先呼）** | 坐席分机（Agent-First） | 客户（Customer-First） |
| **Leg B（后呼）** | 客户电话（经选号网关出局） | 坐席分机（内部 Sofia 注册） |
| **选号** | `RuntimeSelector` 动态选号，占用并发槽位 | 无需选号，直接呼分机 |
| **坐席分配** | 拨号坐席本身 | `InboundAgentResolver` 按 DID→技能组查找 |
| **AI 话术** | 不涉及 | 客户应答时可触发 AI Voice Engine |
| **关键入口函数** | `handleDialpadAgentAnswer()` | `handleInboundCustomerAnswer()` |
| **桥接函数** | `maybeBridgeDialpadDirect()` | `maybeBridgeInbound()` |
| **补振铃音** | ✅ 完整（MediaOrchestrator + broadcastTime） | ✅ 回铃音（`handleInboundAgentProgress`） |
| **无坐席处理** | N/A（单次呼出） | ✅ 入队等待 / 超时挂断 / 降级安全挂断 |
| **ACW 冷却** | ✅ 5s + 排队拉取 | ✅ 5s + 拉取呼入排队客户 |

---

## 5. 已修复项

### 5.1 ✅ 已修复 — 呼入：无空闲坐席时进入排队，超时安全挂断

**原问题：** `handleInboundCustomerAnswer` 中，当 `InboundAgentResolver` 找不到可用坐席时，客户被无限 park 或只能被立即忙音挂断。

**修复方案：**
1. `InboundAgentResolver` 查询 DID 关联坐席时同步返回 `skillGroupId`。
2. 无空闲坐席但存在技能组时，复用现有 `cti.CallQueue`：`queue.Push(merchantId, skillGroupId, callId)`。
3. 向客户腿播放等待音（默认 `local_stream://default`）。
4. 设置 `inQueue=true`、`queueWaitPlaying=true`、`skillGroupId`，使客户中途挂机清理和 ACW 后拉取逻辑自动生效。
5. 30 秒仍未被坐席拉取时，使用 `queue.Remove()` 原子确认仍在队列中，然后自动挂断客户腿。
6. 如果队列不可用或 DID 没有关联技能组，则降级为播放忙音提示并安全挂断。

### 5.2 ✅ 已修复 — 呼入：客户等待坐席接听时播放回铃音

**原问题：** 呼入 consumer 只处理了 `CHANNEL_ANSWER` 事件，坐席振铃时客户侧无任何声音提示。

**修复方案：**
1. 在呼入事件路由中增加 `CHANNEL_PROGRESS` / `CHANNEL_PROGRESS_MEDIA` 的坐席侧事件处理。
2. 新增 `handleInboundAgentProgress()` 函数：通过 `MediaOrchestrator` 向客户腿播放标准回铃音（`tone_stream://%(1000,4000,440,480);loops=-1`）。
3. 更新 `handleInboundAgentReady()` 增加 `mediaRegistry` 参数：坐席接听时停止回铃音。

### 5.3 ✅ 已修复 — Session 重复 load-modify-save 写覆盖消除

**原问题：** `handleDialpadAgentAnswer` 和 `StartDialpadCustomerOutbound` 都对同一 session 做 load-modify-save，迁移到 Redis/DB 后会出现字段级写覆盖。

**修复方案：** 修改 `StartDialpadCustomerOutbound`（`originate.go`）：
- 移除函数内部的 `session.Metadata` 写入和 `Store.Save()` 调用
- 保留 `CreateFromOriginate()` 仅用于注册客户腿 UUID 到 session UUIDs 映射
- session 的 metadata 字段（`customerOriginateSent`、`selectedCaller` 等）由 `handleDialpadAgentAnswer` 统一设置并一次性保存

### 5.4 ✅ 已修复 — 呼入：坐席 ACW 后自动拉取呼入排队客户

**原问题：** ACW 5s 冷却后的 `queue.Pop()` 使用的是批量外呼的多租户队列，但呼入客户从未 `queue.Push()`，所以坐席空闲后拉不到呼入客户。

**修复方案：** 呼入无空闲坐席时复用同一个队列键 `(merchantId, skillGroupId)` 入队。已有 ACW 逻辑在坐席冷却结束后调用 `batchRepo.GetAgentSkillGroups()` 和 `queue.Pop()`，因此可自然拉取呼入排队客户、停止等待音并起呼坐席腿。

---

## 6. 其余呼叫流程概览

除 yunshu-phone 的拨号盘直呼和客户呼入外，系统还支持以下呼叫流程：

| 流程 | ESL 工作流 | 语义 | 触发方式 | 状态 |
|---|---|---|---|---|
| API 外呼 | `esl_api_outbound` | Agent-First | REST API `POST /cti/callTask/call` | ✅ 完整 |
| 批量外呼 | `esl_batch_outbound` | Customer-First | 批量调度器 CAS 分配号码 | ✅ 完整 |
| 批量预测外呼 | `esl_batch_predictive` | Customer-First | 批量调度器 + 排队队列 | ✅ 完整 |
| 批量协同外呼 | `esl_batch_synergy` | Customer-First（振铃即起呼坐席） | 批量调度器 | ✅ 完整 |
| 拨号盘直呼 | `esl_dialpad_direct` | Agent-First | yunshu-phone 拨号 | ✅ 完整 |
| 客户呼入 | `esl_inbound` | Customer-First | 外部客户拨打 DID | ✅ 完整 |

---

## 7. Kamailio SIP 注册流程

yunshu-phone 通过 Kamailio 完成 SIP 注册，这是呼出和呼入的前置条件：

```text
yunshu-phone                     Kamailio                          cc-call
    │                               │                                │
    │──── REGISTER ────────────────►│                                │
    │                               │── auth_db 查询 cc_res_extension │
    │                               │   (ha1b 带域哈希比对)            │
    │◄─── 401 Unauthorized ────────│                                │
    │                               │                                │
    │──── REGISTER (带 auth) ─────►│                                │
    │                               │── 写入 cc_res_location          │
    │                               │── HTTP Webhook ────────────────►│
    │                               │   POST /cti/kamailio/auth/register
    │◄─── 200 OK ─────────────────│                                │
    │                               │                 更新 Redis extension:status = IDLE
```

**Kamailio 配置要点（`configs/kamailio/kamailio.cfg`）：**
- `auth_db` 模块：`calculate_ha1=0`，`use_domain=1`，`password_column_2=ha1b`
- `dispatcher` 模块：从 `cc_res_freeswitch` 表加载 FS 节点列表
- WebSocket 支持：端口 5066 供 WebRTC 软电话使用

**FreeSWITCH 入口拨号计划（`docker/freeswitch/conf/dialplan/public/01_yunshu_inbound.xml`）：**
```xml
<extension name="yunshu_inbound">
  <condition field="destination_number" expression="^.*$">
    <action application="answer"/>
    <action application="park"/>
  </condition>
</extension>
```
所有经 Kamailio 转发到 FreeSWITCH 的 INVITE 都会被 answer + park，等待 cc-call 通过 ESL 编排后续流程。
