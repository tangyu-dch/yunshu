---
title: 云枢声讯呼出
order: 3
---

# 云枢声讯呼出

云枢声讯呼出是指坐席通过云枢声讯主动拨打客户电话。

对应 ESL 工作流：`esl_dialpad_direct`

---

## 1. 业务语义

云枢声讯直呼属于 **Agent-First** 语义：

```mermaid
graph LR
    A[坐席发起 SIP INVITE] --> B[系统识别主叫为坐席分机]
    B --> C[选号并发起客户腿外呼]
    C --> D[客户振铃]
    D --> E[客户接听]
    E --> F[桥接坐席腿和客户腿]
```

详细流程：
1. 坐席先发起 SIP INVITE。
2. 云枢声讯识别主叫为坐席分机。
3. 系统再进行号码选择和客户腿外呼。
4. 客户接听后桥接坐席腿和客户腿。

---

## 2. 完整流程

```mermaid
sequenceDiagram
    participant U as 坐席
    participant P as 云枢声讯
    participant K as Kamailio
    participant FS as FreeSWITCH
    participant C as cc-call
    participant CW as cc-worker

    U->>P: 输入客户号码并拨打
    P->>K: SIP INVITE
    K->>FS: dispatcher 转发
    FS->>FS: public dialplan answer + park
    FS->>C: CHANNEL_CREATE 事件
    C->>C: EventFromESL 生成 callId
    C->>C: SessionSniffer.IsExtension(caller)
    C->>C: 创建 api_direct 会话
    C->>C: consumer 推进 esl_dialpad_direct

    U->>P: 坐席摘机
    P->>FS: 200 OK
    FS->>C: CHANNEL_ANSWER 事件

    C->>C: RuntimeSelector.SelectAndClaim
    C->>C: StartDialpadCustomerOutbound
    C->>FS: originate 客户腿

    FS->>C: CHANNEL_CREATE (客户腿)
    Note over FS: 客户振铃
    FS->>C: CHANNEL_PROGRESS
    C->>C: MediaOrchestrator 播放补振铃
    C->>FS: 播放回铃音给坐席

    Note over FS: 客户接听
    FS->>C: CHANNEL_ANSWER (客户)
    C->>C: BridgeDialpadDirect
    C->>FS: uuid_bridge
    FS->>C: CHANNEL_BRIDGE

    Note over U,P: 通话中

    U->>P: 坐席挂机
    P->>FS: BYE
    FS->>C: CHANNEL_HANGUP_COMPLETE
    C->>C: 写入 CDR outbox
    C->>CW: Redis Stream 事件
    CW->>CW: CDR 持久化/计费/录音/Webhook
```

---

## 3. 会话识别

当物理呼叫没有云枢声讯 callId 时，系统使用 FreeSWITCH `Unique-ID` 兜底。

`SessionSniffer` 会先检查主叫是否为分机：

```mermaid
graph TD
    A[收到 CHANNEL_CREATE] --> B{会话是否存在?}
    B -->|是| C[应用事件到现有会话]
    B -->|否| D[提取 callerNumber]
    D --> E[查询 cc_res_extension]
    E --> F{找到分机?}
    F -->|是| G[创建 api_direct 会话]
    F -->|否| H[检查 calleeNumber]
    H --> I{是商户 DID?}
    I -->|是| J[创建 inbound 会话]
    I -->|否| K[丢弃事件]

    G --> L[设置 Profile = api_direct]
    G --> M[设置 LegRole = agent]
    G --> N[Metadata: userId, merchantId, extension, caller, callee]
```

**SQL 查询：**
```sql
SELECT * FROM cc_res_extension
WHERE extension_number = callerNumber
  AND enable = 1
  AND del_flag = 0;
```

---

## 4. 选号

云枢声讯直呼使用和 API 外呼相同的候选号码源：

```mermaid
graph TD
    A[坐席已接听] --> B[CandidateSource.CandidatesForUser userId]
    B --> C[获取用户关联的号码池]
    C --> D[RuntimeSelector.SelectAndClaim]
    D --> E{选号成功?}
    E -->|是| F[占用并发槽位]
    E -->|否| G[安全挂断坐席]
    F --> H[发起客户腿 originate]
```

选号会考虑：
- 号码是否启用
- 网关是否启用
- 技能组绑定
- 黑白名单
- 盲区规则
- 号码/网关并发

**选号入口：**
```
RuntimeSelector.SelectAndClaim(callId, merchantId, userId, callee)
```

---

## 5. 客户腿 originate

客户腿通过所选网关或 IP 直连呼出：

```mermaid
sequenceDiagram
    participant C as cc-call
    participant FS as FreeSWITCH
    participant G as 网关
    participant P as 客户

    C->>C: StartDialpadCustomerOutbound
    C->>C: 构建 originate 命令
    C->>FS: originate sofia/gateway/gw-name/13800138000
    FS->>G: SIP INVITE
    G->>P: 拨打客户

    P->>G: 180 Ringing
    G->>FS: 180 Ringing
    FS->>C: CHANNEL_PROGRESS

    P->>G: 200 OK
    G->>FS: 200 OK
    FS->>C: CHANNEL_ANSWER
```

**写入 metadata：**
```json
{
  "customerOriginateSent": true,
  "selectedCaller": "01088886666",
  "selectedGatewayId": "44",
  "customerUuid": "..."
}
```

---

## 6. 补振铃音

客户侧 180/183 到达后：

```mermaid
graph TD
    A[收到客户腿事件] --> B{事件类型?}
    B -->|CHANNEL_PROGRESS| C[向坐席播放补振铃音]
    B -->|CHANNEL_PROGRESS_MEDIA| D[认为早期媒体已到达<br/>停止补铃<br/>尝试桥接]
    B -->|CHANNEL_ANSWER| E[停止补铃<br/>标记客户就绪<br/>尝试桥接]

    C --> F[MediaOrchestrator 管理]
    D --> F
    E --> F
    F --> G[broadcastTime 定时截断]
```

- **180**：可向坐席侧播放补振铃
- **183**：认为客户侧早期媒体已到达，停止补铃并尝试桥接

媒体编排由 `MediaOrchestrator` 管理。

---

## 7. 状态机

```mermaid
stateDiagram-v2
    [*] --> init
    init --> agent_created: CHANNEL_CREATE (坐席腿)

    state AgentLeg {
        agent_created --> agent_progress: CHANNEL_PROGRESS
        agent_created --> agent_progress_media: CHANNEL_PROGRESS_MEDIA
        agent_progress --> agent_answered: CHANNEL_ANSWER
        agent_progress_media --> agent_answered: CHANNEL_ANSWER
        agent_created --> complete: CHANNEL_HANGUP_COMPLETE
        agent_progress --> complete: CHANNEL_HANGUP_COMPLETE
        agent_progress_media --> complete: CHANNEL_HANGUP_COMPLETE
    }

    agent_answered --> customer_validated: validate_command
    customer_validated --> customer_originating: execute_originate

    state CustomerLeg {
        customer_originating --> customer_created: CHANNEL_CREATE
        customer_created --> customer_progress: CHANNEL_PROGRESS
        customer_created --> customer_early_media: CHANNEL_PROGRESS_MEDIA
        customer_progress --> customer_answered: CHANNEL_ANSWER
        customer_early_media --> customer_answered: CHANNEL_ANSWER
        customer_created --> complete: CHANNEL_HANGUP_COMPLETE
        customer_progress --> complete: CHANNEL_HANGUP_COMPLETE
        customer_early_media --> complete: CHANNEL_HANGUP_COMPLETE
        customer_answered --> complete: CHANNEL_HANGUP_COMPLETE
    }

    customer_progress --> bridged: CHANNEL_BRIDGE
    customer_early_media --> bridged: CHANNEL_BRIDGE
    customer_answered --> bridged: CHANNEL_BRIDGE
    bridged --> complete: CHANNEL_HANGUP_COMPLETE
    complete --> [*]
```

如果 FreeSWITCH public dialplan 只 park 坐席腿，系统也可以从 `CHANNEL_CREATE` 触发客户腿起呼。

---

## 8. CDR

无论客户腿是否成功，只要会话最终收到 `CHANNEL_HANGUP_COMPLETE`，都会写入：

```mermaid
graph LR
    A[CHANNEL_HANGUP_COMPLETE] --> B{会话存在?}
    B -->|是| C[写入 call_center_cdr_queue]
    B -->|否| D[记录日志但不写入]
    C --> E[cc-worker 消费]
    E --> F[写入 call_cdr_record]
    E --> G[触发计费]
    E --> H[触发录音归档]
    E --> I[触发 Webhook]
```

例如客户腿失败、坐席取消、选号失败，都应有通话记录。

---

## 9. 验证

```bash
bash scripts/sipp/run_e2e_tests.sh dialpad
```

如果本机出现 UDP 路由问题：

```text
Unable to send UDP message: No route to host
```

建议使用：

```bash
SIPP_UAS_MODE=docker bash scripts/sipp/run_e2e_tests.sh dialpad
```

或在 Linux 服务器部署后验证。

---

## 10. 常见故障

### 未进入 api_direct

检查事件别名：
- `callerNumber`
- `calleeNumber`
- `Caller-Caller-ID-Number`
- `Caller-Destination-Number`

### 没有发起客户腿

检查：
- 是否捕获到 `api_direct`
- 分机是否有 userId
- CandidateSource 是否有候选号码
- 风控表是否存在
- 选号是否成功

### 客户 UAS 不回包

本地 Docker Desktop 常见，需要容器内 UAS 或服务器环境。

---

## 11. 相关代码索引

| 功能 | 文件位置 |
| --- | --- |
| ESL 工作流定义 | `internal/domain/esl/workflows.go` |
| 会话管理核心 | `internal/domain/esl/session.go` |
| 呼出编排 | `internal/domain/esl/originate.go` |
| 事件消费者路由 | `internal/domain/callflow/consumer.go` |
| 会话嗅探器 | `internal/infra/resource/session_sniffer.go` |
| 运行时选号器 | `internal/infra/selection/runtime_selector.go` |
| 媒体编排器 | `internal/domain/esl/media_orchestrator.go` |
