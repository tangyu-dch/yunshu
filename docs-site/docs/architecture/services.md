---
title: 服务边界
order: 2
---

# 服务边界

云枢声讯采用微服务架构，将不同功能划分为独立服务，职责清晰，便于扩展和维护。

---

## 总体架构图

```mermaid
graph TB
    subgraph "接入层"
        Client[客户电话]
        Phone[云枢声讯]
        Third[第三方系统]
        Admin[管理员]
    end

    subgraph "信令媒体层"
        K[Kamailio]
        FS[FreeSWITCH]
        RTP[RTPEngine]
    end

    subgraph "业务服务层"
        Console[cc-console]
        Edge[cc-edge]
        Call[cc-call]
        Worker[cc-worker]
    end

    subgraph "存储层"
        MySQL[(MySQL)]
        Redis[(Redis)]
        OSS[(对象存储)]
    end

    Client -->|SIP| K
    Phone -->|SIP| K
    Phone -->|WebSocket| Console
    Third -->|HTTP| Edge
    Admin -->|HTTP| Console

    K -->|SIP| FS
    K -->|媒体| RTP
    RTP -->|媒体| FS

    Edge -->|HTTP| Call
    Console -->|HTTP| Call
    Console -->|读写| MySQL
    Console -->|读写| Redis
    Call -->|ESL| FS
    Call -->|读写| Redis
    Call -->|写| MySQL
    Call -->|Stream| Worker
    Worker -->|读写| MySQL
    Worker -->|读写| Redis
    Worker -->|上传| OSS
```

---

## cc-console

**职责：**
- 运营后台
- 商户后台
- 云枢声讯配套接口
- 分机、号码池、技能组、网关配置

**API 范围：**

```mermaid
graph TB
    A[cc-console] --> B[商户管理]
    A --> C[坐席管理]
    A --> D[分机管理]
    A --> E[号码池管理]
    A --> F[技能组管理]
    A --> G[网关配置]
    A --> H[云枢声讯登录]
    A --> I[报表查询]
    A --> J[CDR 查询]
    A --> K[录音查询]
```

**端口：** `8080`

---

## cc-edge

**职责：**
- 外部 API 网关
- 商户鉴权
- 限流
- 反向代理

```mermaid
graph TB
    A[第三方系统] --> B[cc-edge]
    B --> C{鉴权<br/>X-App-Key/X-App-Secret}
    C -->|通过| D{限流检查}
    D -->|通过| E[路由到 cc-call]
    C -->|拒绝| F[401 Unauthorized]
    D -->|拒绝| G[429 Too Many Requests]
    E --> H[API 外呼]
    E --> I[呼叫状态查询]
    E --> J[录音下载]
```

**端口：** `8081`

---

## cc-call

**职责：**
- CTI 选号
- FreeSWITCH ESL 连接
- 呼叫会话状态机
- Kamailio webhook
- CTI WebSocket
- 呼入/呼出/批量流程编排

```mermaid
graph TB
    A[cc-call] --> B[ESL 连接池]
    A --> C[会话状态机]
    A --> D[CTI 工作流]
    A --> E[ESL 工作流]
    A --> F[选号器]
    A --> G[事件总线]
    A --> H[Kamailio Webhook]
    A --> I[CTI WebSocket]
    A --> J[CDR Outbox]

    B --> K[FreeSWITCH]
    J --> L[Redis Stream]
    L --> M[cc-worker]
```

**端口：** `8082`

---

## cc-worker

**职责：**
- Outbox 投递
- CDR 落库
- 计费流水
- 结算
- 录音任务
- 报表投影
- 下游 webhook

```mermaid
graph TB
    A[cc-worker] --> B[Outbox 消费者]
    B --> C[CDR 落库]
    B --> D[计费流水]
    B --> E[结算任务]
    B --> F[录音任务]
    B --> G[报表投影]
    B --> H[下游 Webhook]

    F --> I[上传录音到 OSS]
```

**端口：** `8083`

---

## 服务间通信

```mermaid
sequenceDiagram
    participant Console as cc-console
    participant Call as cc-call
    participant Worker as cc-worker
    participant Redis as Redis
    participant MySQL as MySQL

    Note over Console: 坐席登录
    Console->>MySQL: 查询分机信息
    Console->>Redis: 初始化坐席状态

    Note over Call: 呼叫处理
    Call->>Redis: 会话状态读写
    Call->>Redis: XADD call_center_cdr_queue

    Note over Worker: 异步处理
    Worker->>Redis: XREADGROUP call_center_cdr_queue
    Worker->>MySQL: INSERT call_cdr_record
    Worker->>MySQL: INSERT cc_biz_ledger
```

---

## 数据流向

```mermaid
graph LR
    A[FreeSWITCH] -->|ESL 事件| B[cc-call]
    B -->|状态更新| C[Redis]
    B -->|CDR| D[Redis Stream]
    D -->|消费| E[cc-worker]
    E -->|持久化| F[MySQL]
    E -->|计费| G[MySQL]
    E -->|录音| H[OSS]
    E -->|Webhook| I[第三方系统]
```

---

## 相关代码索引

| 服务 | 启动入口 |
| --- | --- |
| cc-console | `cmd/cc-console/main.go` |
| cc-edge | `cmd/cc-edge/main.go` |
| cc-call | `cmd/cc-call/main.go` |
| cc-worker | `cmd/cc-worker/main.go` |
| 一键启动（开发） | `cmd/cc-all/main.go` |
