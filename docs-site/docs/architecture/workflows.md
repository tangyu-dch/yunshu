---
title: 事件与工作流
order: 3
---

# 事件与工作流

云枢声讯使用事件驱动方式推进 CTI 和 ESL 状态机，将实时通话控制与异步业务收口解耦。

---

## 1. 总体架构

![事件与工作流](/images/workflow.svg)

---

## 2. CTI 工作流

CTI 工作流处理业务层编排逻辑。

| 工作流 ID | 说明 | 触发事件 |
| --- | --- | --- |
| `cti_api_outbound` | API 外呼 | `api-call-requested` |
| `cti_dialpad_direct` | 云枢声讯直呼 | `fs-applied` (识别为分机) |
| `cti_inbound` | 客户呼入 | `fs-applied` (识别为 DID) |
| `cti_batch_outbound` | 批量外呼 | `batch-call-requested` |

### CTI 工作流执行流程

| 步骤 | 说明 |
| --- | --- |
| 1 | 收到呼叫请求（API / 拨号盘 / 批量调度） |
| 2 | 业务校验和参数验证 |
| 3 | 触发对应 ESL 工作流 |
| 4 | 等待 ESL 状态变化 |
| 5 | 推送状态更新给客户端 |

---

## 3. ESL 工作流

ESL 工作流处理物理信令层控制。

| 工作流 ID | 说明 | 对应 CTI 工作流 |
| --- | --- | --- |
| `esl_api_outbound` | API 外呼物理信令 | `cti_api_outbound` |
| `esl_dialpad_direct` | 云枢声讯直呼物理信令 | `cti_dialpad_direct` |
| `esl_inbound` | 呼入物理信令 | `cti_inbound` |
| `esl_batch_outbound` | 批量物理信令 | `cti_batch_outbound` |

### ESL 工作流执行流程

| 步骤 | 说明 |
| --- | --- |
| 1 | 收到 `fs-applied` 事件 |
| 2 | 验证命令和参数 |
| 3 | 发送 ESL 命令给 FreeSWITCH |
| 4 | 等待 FreeSWITCH 事件响应 |
| 5 | 推进状态机到下一状态 |

---

## 4. 事件类型总览

### 事件入口列表

| 事件类型 | 说明 | 发布者 |
| --- | --- | --- |
| `api-call-requested` | API 发起外呼请求 | API Gateway |
| `batch-call-requested` | 批量调度发起呼叫 | Batch Scheduler |
| `esl.command.sent` | ESL 命令已发送 | ESL Runner |
| `esl.fs_event.applied` | FS 事件已应用 | Session Service |
| `cdr_persisted` | CDR 已持久化 | CDR Consumer |
| `billing_completed` | 计费已完成 | Billing Service |
| `recording_completed` | 录音已归档 | Recording Service |
| `push_completed` | 推送已完成 | Push Service |
| `callback_completed` | Webhook 回调已完成 | Webhook Service |

---

## 5. 事件总线设计

### 核心订阅关系

| 订阅者 | 订阅事件 | 处理逻辑 |
| --- | --- | --- |
| CTI Runner | `api-call-requested` | 启动 CTI 工作流 |
| CTI Runner | `batch-call-requested` | 启动批量 CTI 工作流 |
| ESL Runner | `esl.fs_event.applied` | 驱动 ESL 状态机 |
| Command Builder | `esl.command.sent` | 构建并发送 ESL 命令 |
| CDR Outbox | `esl.fs_event.applied` (hangup_complete) | 写入 CDR 队列 |

---

## 6. 可靠异步收口 (Outbox 模式)

### Outbox 架构

| 组件 | 职责 |
| --- | --- |
| Session Service | 将 CDR 事件写入 Redis Stream |
| Redis Stream | 可靠消息队列，支持消费者组 |
| CDR Consumer | 从 Stream 消费消息并持久化 |
| Fanout Dispatcher | 分发到计费、录音、报表、Webhook 等节点 |

### 可靠性保证

| 保证 | 实现方式 |
| --- | --- |
| 至少一次投递 | Redis Stream consumer group + ACK |
| 幂等处理 | 基于 call_uuid 去重 |
| 失败重试 | 死信队列 + 人工重试 |
| 顺序保证 | 单 partition 消费 |

---

## 7. 相关代码索引

| 功能 | 文件位置 |
| --- | --- |
| 事件定义 | internal/domain/callflow/events.go |
| 事件总线 | internal/domain/callflow/event_bus.go |
| ESL 工作流 | internal/domain/esl/workflows.go |
| CTI 工作流 | internal/domain/callflow/workflows.go |
| 事件消费者 | internal/domain/callflow/consumer.go |
| CDR Outbox | internal/domain/cdr/outbox.go |
