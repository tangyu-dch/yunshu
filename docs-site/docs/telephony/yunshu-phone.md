---
title: 云枢声讯
order: 1
---

# 云枢声讯

云枢声讯是云枢声讯官方桌面客户端，通过 SIP 协议接入 Kamailio，由云枢声讯后端完成 CTI/ESL 业务编排。

---

## 1. 整体架构

```mermaid
graph TB
    subgraph "云枢声讯客户端"
        UI[桌面 UI]
        SIP[SIP 协议栈]
        WS[CTI WebSocket]
        Media[媒体引擎]
    end

    subgraph "云枢声讯服务端"
        Console[cc-console]
        CCCall[cc-call]
        CCWorker[cc-worker]
    end

    subgraph "通信基础设施"
        K[Kamailio]
        FS[FreeSWITCH]
        RTP[RTPEngine]
    end

    UI --> SIP
    UI --> WS
    SIP --> Media
    WS --> UI

    SIP -->|REGISTER/INVITE| K
    K -->|dispatcher| FS
    FS -->|媒体| RTP
    RTP -->|媒体| SIP

    WS -->|CTI 状态| Console
    Console -->|API| CCCall
    CCCall -->|ESL| FS
    CCCall -->|事件| CCWorker
```

---

## 2. 基本流程

```mermaid
sequenceDiagram
    participant U as 用户
    participant P as 云枢声讯
    participant A as cc-console
    participant K as Kamailio
    participant C as cc-call

    U->>P: 输入账号密码登录
    P->>A: POST /mer/auth/dialpad/login
    A->>A: 验证账号
    A-->>P: 返回 token + 分机信息
    P->>P: 获取 extension/password/domain

    P->>K: REGISTER
    K->>K: 查询 cc_res_extension
    K-->>P: 401 Unauthorized
    P->>K: REGISTER (带 auth)
    K->>K: 验证成功
    K->>C: Webhook /cti/kamailio/auth/register
    C->>C: Redis extension:status = IDLE
    K-->>P: 200 OK

    P->>C: WebSocket /cti/ws
    C-->>P: 连接成功
    P->>P: 坐席上线
```

---

## 3. 关键接口

| 功能 | 入口 | 方法 | 说明 |
| --- | --- | --- | --- |
| 云枢声讯登录 | `/mer/auth/dialpad/login` | POST | 获取访问令牌 |
| 获取分机信息 | `/mer/v1/user/dialpad/extensionInfo` | GET | 获取 SIP 注册配置 |
| CTI WebSocket | `/cti/ws` | WS | 实时状态同步 |
| SIP 注册 | Kamailio `5060/5066` | SIP | SIP REGISTER |

### 登录接口示例

**请求：**
```http
POST /mer/auth/dialpad/login
Content-Type: application/json

{
  "username": "agent001",
  "password": "password123"
}
```

**响应：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": 1001,
      "username": "agent001",
      "merchantId": 1
    },
    "extension": {
      "extensionNumber": "1001",
      "password": "sip_password",
      "domain": "yunshu.io",
      "outboundProxy": "sip.yunshu.io:5060"
    }
  }
}
```

### CTI WebSocket 消息

**坐席状态变化：**
```json
{
  "type": "extension_status",
  "data": {
    "extensionNumber": "1001",
    "status": "IDLE", // IDLE, RINGING, ANSWERED, BUSY, ACW, OFFLINE
    "timestamp": 1718000000
  }
}
```

**来电事件：**
```json
{
  "type": "incoming_call",
  "data": {
    "callId": "call-123456",
    "callerNumber": "13800138000",
    "calleeNumber": "4001234567",
    "skillGroupName": "客服一组",
    "timestamp": 1718000000
  }
}
```

---

## 4. 支持的呼叫场景

| 场景 | 说明 | 呼叫方向 |
| --- | --- | --- |
| 云枢声讯直呼 | 坐席从云枢声讯主动拨打客户号码 | 呼出 |
| 客户呼入 | 客户拨打商户 DID，系统分配坐席 | 呼入 |
| API 外呼 | 第三方或后台调用 API 发起外呼 | 呼出 |
| 批量外呼 | 系统按任务队列自动呼叫客户 | 呼出 |

---

## 5. 分机状态

### 状态枚举

```mermaid
graph LR
    OFFLINE -.->|注册成功| IDLE
    IDLE -->|收到 INVITE| RINGING
    RINGING -->|坐席摘机| ANSWERED
    RINGING -->|坐席拒接| IDLE
    ANSWERED -->|桥接客户| TALKING
    TALKING -->|挂机| ACW
    ACW -->|5s 冷却| IDLE
    IDLE -->|API/批量任务| BUSY
    BUSY -->|任务结束| IDLE
```

### Redis 存储

Redis key 格式：`extension:status:{extensionNumber}`

状态值：

| 值 | 枚举 | 说明 |
| --- | --- | --- |
| -1 | OFFLINE | 离线/未注册 |
| 0 | IDLE | 空闲 |
| 1 | BUSY | 忙碌（不可分配） |
| 2 | RINGING | 振铃中 |
| 3 | ANSWERED | 已接听 |
| 4 | TALKING | 通话中 |
| 5 | ACW | 话后处理 |

---

## 6. 拨号盘直呼流程

坐席在云枢声讯上直接输入客户号码发起呼叫。

```mermaid
sequenceDiagram
    participant U as 坐席
    participant P as 云枢声讯
    participant K as Kamailio
    participant FS as FreeSWITCH
    participant C as cc-call

    U->>P: 输入客户号码 13800138000
    U->>P: 点击拨打
    P->>P: 构建 SIP INVITE
    P->>K: INVITE sip:13800138000@yunshu.io
    K->>K: dispatcher 选择 FS
    K->>FS: 转发 INVITE
    FS->>FS: dialplan 匹配 01_yunshu_inbound.xml
    FS->>FS: answer() + park()
    FS->>C: CHANNEL_CREATE 事件

    C->>C: Sniffer 识别为分机
    C->>C: 创建 api_direct 会话
    C->>P: WebSocket 来电通知

    U->>P: 点击接听
    P->>FS: 200 OK
    FS->>C: CHANNEL_ANSWER 事件
    C->>C: 坐席已接听，开始选号
    C->>C: RuntimeSelector 选号
    C->>FS: originate 客户腿
    FS->>P: 播放回铃音
    FS->>C: CHANNEL_CREATE (客户腿)

    Note over FS,U: 客户振铃/接听

    FS->>C: CHANNEL_ANSWER (客户)
    C->>C: 客户已接听
    C->>FS: uuid_bridge
    FS->>C: CHANNEL_BRIDGE

    Note over FS,U: 通话中

    U->>P: 点击挂机
    P->>FS: BYE
    FS->>C: CHANNEL_HANGUP_COMPLETE
    C->>C: 写入 CDR outbox
    C->>C: ACW 5s 冷却
    C->>C: 恢复分机 IDLE
    C->>P: WebSocket 状态更新
```

---

## 7. 通话记录保证

只要呼叫进入云枢声讯会话生命周期，并最终收到 `CHANNEL_HANGUP_COMPLETE`，系统都会写入 CDR outbox。

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

---

## 8. 相关代码索引

| 功能 | 文件位置 |
| --- | --- |
| 拨号盘登录接口 | `internal/transport/http/console/operate/dialpad_compat_routes.go` |
| SIP 注册 Webhook | `internal/transport/http/cti/kamailio_routes.go` |
| CTI WebSocket | `internal/transport/http/cti/websocket_handler.go` |
| 分机状态管理 | `internal/domain/extension/status_service.go` |
