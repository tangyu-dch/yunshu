---
title: 客户呼入
order: 4
---

# 客户呼入

客户呼入是指外部客户拨打商户 DID，系统自动分配坐席或进入排队。

对应 ESL 工作流：`esl_inbound`

---

## 1. 完整流程

```mermaid
sequenceDiagram
    participant C as 客户
    participant K as Kamailio
    participant FS as FreeSWITCH
    participant CC as cc-call
    participant P as 云枢声讯
    participant CW as cc-worker

    C->>K: 拨打 DID 4001234567
    K->>FS: dispatcher 转发
    FS->>FS: public dialplan answer + park
    FS->>CC: CHANNEL_CREATE 事件
    CC->>CC: EventFromESL 生成 callId
    CC->>CC: SessionSniffer.IsMerchantDID(callee)
    CC->>CC: 创建 inbound 会话
    CC->>CC: EventBus 发布 esl.fs_event.applied
    CC->>CC: consumer 推进 esl_inbound 工作流

    FS->>CC: CHANNEL_ANSWER (客户)
    CC->>CC: InboundAgentResolver 查找空闲坐席

    alt 有空闲坐席
        CC->>CC: StartInboundAgentOutbound
        CC->>FS: originate 坐席腿
        FS->>P: INVITE 坐席分机
        P->>FS: 180 Ringing
        FS->>CC: CHANNEL_PROGRESS (坐席)
        CC->>CC: 向客户播放回铃音
        CC->>FS: 播放回铃音
        P->>FS: 200 OK (坐席接听)
        FS->>CC: CHANNEL_ANSWER (坐席)
        CC->>CC: BridgeInbound
        CC->>FS: uuid_bridge
        FS->>CC: CHANNEL_BRIDGE
    else 无空闲坐席
        CC->>CC: queue.Push
        CC->>CC: 向客户播放等待音
        CC->>FS: 播放等待音
        Note over CC: 30s 超时或坐席空闲后拉取
    end

    Note over C,P: 通话中

    C->>FS: BYE
    FS->>CC: CHANNEL_HANGUP_COMPLETE
    CC->>CC: 写入 CDR outbox
    CC->>CW: Redis Stream 事件
    CW->>CW: CDR 持久化/计费/录音/Webhook

    Note over CC: 坐席挂断后进入 ACW 5s
    CC->>CC: ACW 结束后恢复 IDLE
    CC->>CC: 从队列拉取等待客户
```

---

## 2. DID 识别

`SessionSniffer` 会检查被叫号码是否是商户 DID：

```mermaid
graph TD
    A[收到 CHANNEL_CREATE] --> B{会话是否存在?}
    B -->|是| C[应用事件到现有会话]
    B -->|否| D[提取 calleeNumber]
    D --> E[查询 cc_res_pool_phone<br/>JOIN cc_tel_pool]
    E --> F{找到 DID?}
    F -->|是| G[创建 inbound 会话]
    F -->|否| H[检查 callerNumber 是否是分机]
    H --> I{是分机?}
    I -->|是| J[创建 api_direct 会话]
    I -->|否| K[丢弃事件]

    G --> L[设置 Profile = inbound]
    G --> M[设置 LegRole = customer]
    G --> N[Metadata: merchantId, customerUuid, caller, callee]
```

**SQL 查询：**
```sql
SELECT pp.*, p.merchant_id
FROM cc_res_pool_phone pp
JOIN cc_tel_pool p ON p.id = pp.pool_id
WHERE pp.phone = ?
  AND pp.enable = 1
  AND pp.del_flag = 0;
```

如果命中，创建：
```text
Profile = inbound
LegRole = customer
Metadata = merchantId, customerUuid, caller, callee
```

---

## 3. 坐席分配

`InboundAgentResolver` 查询链路：

```mermaid
graph TD
    A[DID] --> B[cc_res_pool_phone]
    B --> C[cc_res_pool_phone_skill_group]
    C --> D[cc_res_skill_group]
    D --> E[cc_res_user_skill_group]
    E --> F[cc_res_mch_user]
    F --> G[cc_res_extension]
    G --> H[检查 Redis extension:status]
    H --> I{状态 == IDLE?}
    I -->|是| J[返回该坐席]
    I -->|否| K[继续查找下一个]
```

**SQL 查询：**
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

之后检查 Redis：
```text
HGET extension:status 1001
```

只有状态为 `IDLE(1)` 的分机会被分配。

---

## 4. 无坐席排队

如果 DID 关联技能组，但没有空闲坐席：

```mermaid
graph TD
    A[客户已接听] --> B{有空闲坐席?}
    B -->|否| C{有关联技能组?}
    C -->|是| D[queue.Push merchantId, skillGroupId, callId]
    D --> E[向客户播放等待音]
    E --> F[session.inQueue = true]
    F --> G[session.queueWaitPlaying = true]
    G --> H[session.skillGroupId = 37]
    H --> I[设置 30s 超时]
    I --> J{30s 后仍在队列?}
    J -->|是| K[queue.Remove]
    K --> L[挂断客户]

    C -->|否| M[播放忙音]
    M --> L
```

**Redis key：**
```text
cti:merchant:{merchantId}:queue:skill_group:{skillGroupId}
```

同时 session 写入：
```json
{
  "inQueue": true,
  "queueWaitPlaying": true,
  "skillGroupId": 37
}
```

30 秒后如果仍在队列：
```text
queue.Remove → hangup customer
```

---

## 5. ACW 后拉取

坐席挂断后不会立即变空闲，而是进入 ACW：

```mermaid
graph TD
    A[CHANNEL_HANGUP_COMPLETE agent] --> B[等待 5s]
    B --> C[SetExtensionStatus IDLE]
    C --> D[GetAgentSkillGroups userId]
    D --> E[queue.Pop merchantId, skillGroupId]
    E --> F{取到等待客户?}
    F -->|是| G[停止客户等待音]
    G --> H{场景类型?}
    H -->|呼入| I[StartInboundAgentOutbound]
    H -->|批量| J[StartBatchAgentOutbound]
    F -->|否| K[保持 IDLE]
```

**流程：**
```text
CHANNEL_HANGUP_COMPLETE(agent)
  → 等待 5s
  → SetExtensionStatus(IDLE)
  → GetAgentSkillGroups(userId)
  → queue.Pop(merchantId, skillGroupId)
```

如果取到等待客户：
- 停止客户等待音
- 起呼坐席腿
- 呼入场景走 `StartInboundAgentOutbound`
- 批量场景走 `StartBatchAgentOutbound`

---

## 6. 坐席腿路由

坐席腿必须走 Kamailio location：

```mermaid
graph LR
    A[cc-call] --> B[originate 坐席腿]
    B --> C[sofia/external/1001@sip.merchant.yunshu.com]
    C --> D[;fs_path=sip:192.168.107.2:5060]
    D --> E[X-Internal-Call: true]
    E --> F[Kamailio]
    F --> G[查找 location 表]
    G --> H[转发到坐席实际地址]
```

**SIP URI 格式：**
```text
sofia/external/1001@sip.merchant.yunshu.com;fs_path=sip:192.168.107.2:5060
```

并携带：
```text
X-Internal-Call: true
```

否则 Kamailio 会把呼叫重新 dispatcher 到 FreeSWITCH，或因域不匹配返回 404。

---

## 7. 状态机

```mermaid
stateDiagram-v2
    [*] --> init
    init --> customer_created: CHANNEL_CREATE (客户腿)

    state CustomerLeg {
        customer_created --> customer_progress: CHANNEL_PROGRESS
        customer_created --> customer_early_media: CHANNEL_PROGRESS_MEDIA
        customer_progress --> customer_answered: CHANNEL_ANSWER
        customer_early_media --> customer_answered: CHANNEL_ANSWER
        customer_created --> complete: CHANNEL_HANGUP_COMPLETE
        customer_progress --> complete: CHANNEL_HANGUP_COMPLETE
        customer_early_media --> complete: CHANNEL_HANGUP_COMPLETE
    }

    customer_progress --> agent_validated: validate_command
    customer_early_media --> agent_validated: validate_command
    customer_answered --> agent_validated: validate_command

    agent_validated --> agent_originating: execute_originate

    state AgentLeg {
        agent_originating --> agent_created: CHANNEL_CREATE
        agent_created --> agent_progress: CHANNEL_PROGRESS
        agent_created --> agent_early_media: CHANNEL_PROGRESS_MEDIA
        agent_progress --> agent_answered: CHANNEL_ANSWER
        agent_early_media --> agent_answered: CHANNEL_ANSWER
        agent_created --> complete: CHANNEL_HANGUP_COMPLETE
        agent_progress --> complete: CHANNEL_HANGUP_COMPLETE
        agent_early_media --> complete: CHANNEL_HANGUP_COMPLETE
    }

    agent_answered --> bridged: CHANNEL_BRIDGE
    bridged --> complete: CHANNEL_HANGUP_COMPLETE
    complete --> [*]
```

---

## 8. CDR

呼入只要最终收到：
```text
CHANNEL_HANGUP_COMPLETE
```

就会写入：
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

并最终落库到：
```text
call_cdr_record
```

---

## 9. 验证

```bash
bash scripts/sipp/run_e2e_tests.sh inbound
```

成功输出：
```text
PASS: 呼入 - 客户侧完整信令 (INVITE→200 OK→ACK→BYE)
```

---

## 10. 常见故障

### 呼入一直超时

检查 FreeSWITCH 是否上报 `CHANNEL_CREATE`：
```bash
docker logs cc-freeswitch | grep CHANNEL_CREATE
```

### cc-call 没有自动捕获 inbound

检查 `EventFromESL` 是否生成 callId，DID 是否存在：
```sql
SELECT pp.phone, p.merchant_id
FROM cc_res_pool_phone pp
JOIN cc_tel_pool p ON p.id = pp.pool_id
WHERE pp.phone='01088886666';
```

### 坐席腿 UNALLOCATED_NUMBER

检查 R-URI 域是否为：
```text
sip.merchant.yunshu.com
```

---

## 11. 相关代码索引

| 功能 | 文件位置 |
| --- | --- |
| ESL 工作流定义 | `internal/domain/esl/workflows.go` |
| 会话管理核心 | `internal/domain/esl/session.go` |
| 呼出编排 | `internal/domain/esl/originate.go` |
| 事件消费者路由 | `internal/domain/callflow/consumer.go` |
| 会话嗅探器 | `internal/infra/resource/session_sniffer.go` |
| 坐席分配器 | `internal/infra/resource/inbound_agent_resolver.go` |
| 呼叫队列 | `internal/domain/callflow/call_queue.go` |
