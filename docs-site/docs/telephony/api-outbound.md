---
title: API 外呼
order: 5
---

# API 外呼

API 外呼适合第三方系统或业务后台主动发起呼叫。

---

## 1. API 入口

```http
POST /cti/callTask/call?callId=<call-id>
Host: cc-call:8082
Content-Type: application/json

{
  "userId": 2094,
  "callee": "13800001111"
}
```

**请求参数：**
| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| callId | string | 是 | 调用方生成的唯一呼叫 ID |
| userId | int64 | 是 | 坐席用户 ID |
| callee | string | 是 | 被叫号码 |

**响应示例：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "callId": "call-123456",
    "status": "initiated"
  }
}
```

---

## 2. 处理流程

```mermaid
sequenceDiagram
    participant T as 第三方系统
    participant C as cc-call
    participant FS as FreeSWITCH
    participant P as 云枢声讯
    participant CW as cc-worker

    T->>C: POST /cti/callTask/call?callId=xxx
    C->>C: APICallService 处理
    C->>C: 验证 userId 和分机
    C->>C: EventAPICallRequested
    C->>C: CTI workflow 启动
    C->>C: StartAPIOutbound
    C->>FS: originate 坐席腿
    FS->>P: INVITE 坐席分机
    P->>FS: 180 Ringing
    FS->>C: CHANNEL_PROGRESS (坐席)
    P->>FS: 200 OK (坐席接听)
    FS->>C: CHANNEL_ANSWER (坐席)

    C->>C: RuntimeSelector.SelectAndClaim
    C->>C: 选号成功
    C->>FS: originate 客户腿
    FS->>C: CHANNEL_CREATE (客户)

    Note over FS: 客户振铃/接听
    FS->>C: CHANNEL_PROGRESS/CHANNEL_ANSWER
    C->>C: bridge API
    C->>FS: uuid_bridge
    FS->>C: CHANNEL_BRIDGE

    Note over P: 通话中

    P->>FS: BYE
    FS->>C: CHANNEL_HANGUP_COMPLETE
    C->>C: 写入 CDR outbox
    C->>CW: Redis Stream 事件
    CW->>CW: CDR 持久化/计费/录音/Webhook
```

**详细流程：**
```text
HTTP API
  → APICallService
  → EventAPICallRequested
  → CTI workflow
  → StartAPIOutbound
  → 坐席腿 originate
  → 坐席 ready 后选号
  → 客户腿 originate
  → bridge
  → CDR outbox
```

---

## 3. 状态机

```mermaid
stateDiagram-v2
    [*] --> init
    init --> agent_validated: api-call-requested
    agent_validated --> agent_originating: execute_originate

    state AgentLeg {
        agent_originating --> agent_created: CHANNEL_CREATE
        agent_created --> agent_progress: CHANNEL_PROGRESS
        agent_progress --> agent_answered: CHANNEL_ANSWER
    }

    agent_answered --> customer_validated: validate_command
    customer_validated --> customer_originating: execute_originate

    state CustomerLeg {
        customer_originating --> customer_created: CHANNEL_CREATE
        customer_created --> customer_progress: CHANNEL_PROGRESS
        customer_created --> customer_early_media: CHANNEL_PROGRESS_MEDIA
        customer_progress --> customer_answered: CHANNEL_ANSWER
        customer_early_media --> customer_answered: CHANNEL_ANSWER
    }

    customer_progress --> bridged: CHANNEL_BRIDGE
    customer_early_media --> bridged: CHANNEL_BRIDGE
    customer_answered --> bridged: CHANNEL_BRIDGE
    bridged --> complete: CHANNEL_HANGUP_COMPLETE
    complete --> [*]
```

---

## 4. 注意事项

- **必须传 `callId`**，否则会被视为请求参数不完整。
- `userId` 必须对应有效商户用户和有效分机。
- 生产环境建议通过 `X-App-Key / X-App-Secret` 做商户鉴权。
- API 外呼是 **Agent-First** 语义：先呼坐席，再呼客户。

```mermaid
graph LR
    A[API 请求] --> B{callId 存在?}
    B -->|否| C[返回 400 参数错误]
    B -->|是| D{userId 有效?}
    D -->|否| E[返回 400 用户不存在]
    D -->|是| F{分机存在?}
    F -->|否| G[返回 400 分机不存在]
    F -->|是| H[发起呼叫]
```

---

## 5. 回调通知

可选配置 Webhook 接收呼叫状态变更：

```http
POST /your/webhook/endpoint
Content-Type: application/json

{
  "callId": "call-123456",
  "eventType": "call_initiated",
  "timestamp": 1718000000,
  "data": {
    "userId": 2094,
    "callee": "13800001111",
    "status": "ringing"
  }
}
```

**事件类型：**
| 事件 | 说明 |
| --- | --- |
| call_initiated | 呼叫已发起 |
| agent_ringing | 坐席振铃 |
| agent_answered | 坐席接听 |
| customer_ringing | 客户振铃 |
| customer_answered | 客户接听 |
| bridged | 双方通话 |
| hangup | 呼叫结束 |

---

## 6. 错误码

| code | message | 说明 |
| --- | --- | --- |
| 0 | success | 成功 |
| 400 | bad_request | 请求参数错误 |
| 401 | unauthorized | 未授权 |
| 404 | user_not_found | 用户不存在 |
| 404 | extension_not_found | 分机不存在 |
| 409 | call_already_exists | callId 已存在 |
| 500 | internal_error | 系统内部错误 |

---

## 7. SIPp 验证

```bash
bash scripts/sipp/run_e2e_tests.sh api
```

成功进入云枢声讯时应看到：
```text
API 响应: HTTP 200
```

---

## 8. 相关代码索引

| 功能 | 文件位置 |
| --- | --- |
| API 入口 | `internal/transport/http/cti/call_task_routes.go` |
| API 呼叫服务 | `internal/domain/callflow/api_call_service.go` |
| ESL 工作流定义 | `internal/domain/esl/workflows.go` |
| 呼出编排 | `internal/domain/esl/originate.go` |
| 事件消费者路由 | `internal/domain/callflow/consumer.go` |
