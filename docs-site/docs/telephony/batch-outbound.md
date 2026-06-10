---
title: 批量外呼
order: 6
---

# 批量外呼

批量外呼用于任务式自动呼叫客户号码。

---

## 1. 模式

| 模式 | profile | 说明 |
| --- | --- | --- |
| 标准批量 | `batch_outbound` | 客户接听后呼坐席 |
| 预测外呼 | `batch_predictive` | 客户接听后动态找空闲坐席，无坐席可排队 |
| 协同外呼 | `batch_synergy` | 客户振铃即提前呼坐席 |

---

## 2. 关键流程

```mermaid
sequenceDiagram
    participant S as BatchScheduler
    participant C as cc-call
    participant FS as FreeSWITCH
    participant P as 坐席
    participant CW as cc-worker

    loop 定时调度
        S->>S: ClaimNextPendingBatchTel
        S->>C: StartBatchOutbound
        C->>FS: originate 客户腿
        FS->>C: CHANNEL_CREATE (客户)

        alt 标准批量/预测外呼
            Note over C: 等待客户接听
            FS->>C: CHANNEL_ANSWER (客户)
            C->>C: InboundAgentResolver 查找坐席
            alt 找到坐席
                C->>FS: originate 坐席腿
                FS->>P: INVITE
                P->>FS: 200 OK
                FS->>C: CHANNEL_ANSWER (坐席)
                C->>FS: uuid_bridge
                FS->>C: CHANNEL_BRIDGE
            else 预测外呼无坐席
                C->>C: 进入排队
            end
        else 协同外呼
            Note over C: 客户振铃即呼坐席
            FS->>C: CHANNEL_PROGRESS (客户)
            C->>FS: originate 坐席腿
        end

        Note over C,P: 通话中

        P->>FS: BYE
        FS->>C: CHANNEL_HANGUP_COMPLETE
        C->>C: 写入 CDR outbox
        C->>CW: Redis Stream 事件
        CW->>CW: CDR 持久化/计费/录音/Webhook
    end
```

**详细流程：**
```text
BatchScheduler
  → ClaimNextPendingBatchTel
  → StartBatchOutbound
  → 客户腿 originate
  → 客户应答/振铃
  → 坐席腿 originate
  → bridge
  → terminal_event
  → 下一号码调度
```

---

## 3. 状态机

### 标准批量/预测外呼 (Customer-First)

```mermaid
stateDiagram-v2
    [*] --> init
    init --> customer_validated: batch-call-requested
    customer_validated --> customer_originating: execute_originate

    state CustomerLeg {
        customer_originating --> customer_created: CHANNEL_CREATE
        customer_created --> customer_progress: CHANNEL_PROGRESS
        customer_progress --> customer_answered: CHANNEL_ANSWER
    }

    customer_answered --> agent_validated: validate_command
    agent_validated --> agent_originating: execute_originate

    state AgentLeg {
        agent_originating --> agent_created: CHANNEL_CREATE
        agent_created --> agent_progress: CHANNEL_PROGRESS
        agent_progress --> agent_answered: CHANNEL_ANSWER
    }

    agent_answered --> bridged: CHANNEL_BRIDGE
    bridged --> complete: CHANNEL_HANGUP_COMPLETE
    complete --> [*]
```

### 协同外呼 (Early Agent)

```mermaid
stateDiagram-v2
    [*] --> init
    init --> customer_validated: batch-call-requested
    customer_validated --> customer_originating: execute_originate

    state CustomerLeg {
        customer_originating --> customer_created: CHANNEL_CREATE
        customer_created --> customer_progress: CHANNEL_PROGRESS
    }

    customer_progress --> agent_validated: validate_command
    agent_validated --> agent_originating: execute_originate

    state AgentLeg {
        agent_originating --> agent_created: CHANNEL_CREATE
        agent_created --> agent_progress: CHANNEL_PROGRESS
        agent_progress --> agent_answered: CHANNEL_ANSWER
    }

    customer_progress --> customer_answered: CHANNEL_ANSWER
    agent_answered --> bridged: CHANNEL_BRIDGE
    customer_answered --> bridged: CHANNEL_BRIDGE
    bridged --> complete: CHANNEL_HANGUP_COMPLETE
    complete --> [*]
```

---

## 4. 队列

预测外呼和呼入共用 `CallQueue`：

```mermaid
graph TD
    A[客户接听] --> B{找到空闲坐席?}
    B -->|否| C[queue.Push merchantId, skillGroupId, callId]
    C --> D[向客户播放等待音]
    D --> E[等待坐席空闲]
    E --> F[坐席 ACW 结束]
    F --> G[queue.Pop]
    G --> H[originate 坐席腿]

    B -->|是| I[直接 originate 坐席腿]
```

**Redis key：**
```text
cti:merchant:{merchantId}:queue:skill_group:{skillGroupId}
```

---

## 5. 批量调度器

```mermaid
graph TD
    A[BatchScheduler] --> B[ClaimNextPendingBatchTel]
    B --> C[CAS 占用号码]
    C --> D{占用成功?}
    D -->|否| E[继续下一个]
    D -->|是| F[StartBatchOutbound]
    F --> G[更新号码状态]
    G --> H[等待完成事件]
    H --> I[调度下一号码]
```

**调度间隔：** 可配置，通常 100ms - 1s

**并发控制：**
- 商户级别并发
- 技能组级别并发
- 坐席级别并发

---

## 6. 创建批量任务 API

```http
POST /cti/batchTask/create
Host: cc-console:8080
Content-Type: application/json

{
  "merchantId": 1,
  "skillGroupId": 37,
  "name": "营销活动",
  "type": "batch_predictive",
  "tels": [
    "13800001111",
    "13800002222",
    "13800003333"
  ],
  "callerIds": ["01088886666"]
}
```

**响应示例：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "batchTaskId": 1001
  }
}
```

---

## 7. 相关代码索引

| 功能 | 文件位置 |
| --- | --- |
| 批量调度器 | `internal/domain/callflow/batch_scheduler.go` |
| ESL 工作流定义 | `internal/domain/esl/workflows.go` |
| 呼出编排 | `internal/domain/esl/originate.go` |
| 事件消费者路由 | `internal/domain/callflow/consumer.go` |
| 呼叫队列 | `internal/domain/callflow/call_queue.go` |
