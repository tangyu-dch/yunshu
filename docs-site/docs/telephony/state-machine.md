---
title: 呼叫状态机
order: 7
---

# 呼叫状态机

ESL 会话状态由 FreeSWITCH 事件驱动，云枢声讯为每条呼叫维护一个完整的状态机，从通道创建到最终完成的全生命周期追踪。

## 1. 基础呼叫生命周期

![呼叫状态机](/images/state-machine.svg)

### 关键事件说明

| 事件 | 说明 | 对应 SIP 消息 |
| --- | --- | --- |
| CHANNEL_CREATE | 通道创建 | INVITE 到达 FreeSWITCH |
| CHANNEL_PROGRESS | 180 振铃 | SIP 180 Ringing |
| CHANNEL_PROGRESS_MEDIA | 183 早期媒体 | SIP 183 Session Progress |
| CHANNEL_ANSWER | 接听 | SIP 200 OK |
| CHANNEL_BRIDGE | 桥接 | 两腿合并通话 |
| CHANNEL_HANGUP | 挂断 | 任意一方挂机 |
| CHANNEL_HANGUP_COMPLETE | 最终挂断 | 通道完全销毁 |

---

## 2. 拨号盘直呼状态机 (esl_dialpad_direct)

坐席先摘机，系统再选号呼叫客户的 Agent-First 模式。

### 拨号盘直呼关键节点

| 节点 | 触发条件 | 关键动作 |
| --- | --- | --- |
| agent_answered | 坐席摘机 | ① 加载可选号码池<br>② RuntimeSelector 选号占槽<br>③ 发起客户腿 originate |
| customer_validated | 选号成功 | 验证 callerId 可用性 |
| customer_originating | 执行 originate | 向网关发送 INVITE |
| bridged | 两腿桥接 | uuid_bridge 合并通道 |

---

## 3. 客户呼入状态机 (esl_inbound)

客户先拨打 DID，系统再分配坐席的 Customer-First 模式。

### 客户呼入关键节点

| 节点 | 触发条件 | 关键动作 |
| --- | --- | --- |
| customer_answered | 客户已应答 | ① 检查 AI 开关<br>② 查询空闲坐席<br>③ 发起坐席腿 originate<br>④ 无坐席则入队等待 |
| agent_validated | 找到坐席 | 验证分机可用性 |
| agent_originating | 执行 originate | 向坐席分机发送 INVITE |
| bridged | 两腿桥接 | uuid_bridge 合并通道 |

---

## 4. 事件映射表

### FreeSWITCH 事件 → 云枢声讯状态

| FS 事件 | 云枢声讯事件 | 状态更新 |
| --- | --- | --- |
| CHANNEL_CREATE | esl.fs_event.applied | 创建会话 |
| CHANNEL_PROGRESS | esl.fs_event.applied | progress |
| CHANNEL_PROGRESS_MEDIA | esl.fs_event.applied | progress_media |
| CHANNEL_ANSWER | esl.fs_event.applied | answered |
| CHANNEL_BRIDGE | esl.fs_event.applied | bridged |
| CHANNEL_HANGUP | esl.fs_event.applied | hangup |
| CHANNEL_HANGUP_COMPLETE | esl.fs_event.applied, cdr_persisted | complete |

---

## 5. CDR 写入规则

### CDR 保证语义

> **只要呼叫进入云枢声讯会话生命周期，并最终收到 CHANNEL_HANGUP_COMPLETE，就会写入 CDR outbox。**

### CDR 保证覆盖场景

| 场景 | Profile | 测试覆盖 |
| --- | --- | --- |
| API 外呼 | CallFlowAPI | ✅ |
| 云枢声讯直呼 | CallFlowAPIDirect | ✅ |
| 客户呼入 | CallFlowInbound | ✅ |
| 批量外呼 | CallFlowBatch | ✅ |
| 预测外呼 | CallFlowPredictive | ✅ |
| 协同外呼 | CallFlowSynergy | ✅ |

---

## 6. 异常处理

### 状态回退与重试

| 异常场景 | 处理策略 |
| --- | --- |
| ESL broken pipe | 清理连接，自动重连 |
| Redis 短暂异常 | 日志告警，关键路径降级 |
| 下游 Webhook 失败 | outbox 重试，不影响通话 |
| 录音上传失败 | 录音任务重试，不影响通话 |
| Worker 停止 | outbox 保留，恢复后投递 |

---

## 7. 相关代码索引

| 功能 | 文件位置 |
| --- | --- |
| 会话状态机核心 | internal/domain/esl/session.go |
| 工作流定义 | internal/domain/esl/workflows.go |
| 事件消费路由 | internal/domain/callflow/consumer.go |
