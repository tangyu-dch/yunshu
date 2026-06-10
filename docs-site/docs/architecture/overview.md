---
title: 总体架构
order: 1
---

# 总体架构

云枢声讯采用"四服务 + 通信基础设施 + 可靠异步节点"的架构，核心目标是把实时话务控制和异步业务收口解耦。

---

## 系统架构图

![系统架构图](/images/architecture.svg)

---

## 为什么这样拆分

传统呼叫中心常见问题：
- SIP 注册、路由、媒体、业务状态混在一起，排障困难
- 呼叫状态强依赖同步调用，任意一个外部系统慢都会拖垮主流程
- 话单、计费、录音、回调经常和挂断事件耦合，失败难重试
- 批量外呼、预测外呼、呼入分配逻辑容易重复实现

云枢声讯的拆分原则：

| 层 | 关注点 |
| --- | --- |
| Kamailio | SIP 边界、注册、鉴权、dispatcher |
| FreeSWITCH | 媒体和通道控制 |
| cc-call | 业务编排、状态机、ESL 命令 |
| cc-worker | 可重试的异步副作用 |
| Redis | 热状态、并发、队列 |
| MySQL | 持久化和审计 |

---

## 数据流动

### 实时呼叫路径

实时链路：FreeSWITCH → cc-call → ESL 命令

| 步骤 | 说明 |
| --- | --- |
| 1 | FreeSWITCH 发送 CHANNEL_CREATE |
| 2 | cc-call 识别并创建会话 |
| 3 | 推进状态机执行工作流 |
| 4 | 发送 ESL 命令（originate/bridge/hangup） |

### 异步收口路径

异步链路：cc-call → Redis Stream → cc-worker

| 步骤 | 说明 |
| --- | --- |
| 1 | cc-call 写入 CDR Outbox |
| 2 | cc-worker 从 Stream 消费 |
| 3 | 分发到 CDR、计费、录音、报表、Webhook 节点 |

---

## 四个 Go 服务

| 服务 | 默认端口 | 主要职责 | 是否可水平扩展 |
| --- | --- | --- | --- |
| cc-console | 8080 | 控制台、商户后台、云枢声讯兼容 API | 可以 |
| cc-edge | 8081 | 外部 API 网关、鉴权、限流 | 可以 |
| cc-call | 8082 | CTI、ESL、通话会话、工作流 | 可以，但 FS 事件按租约单 owner |
| cc-worker | 8083 | outbox、CDR、计费、录音、报表、Webhook | 可以 |

开发时可以用 `cc-all` 单进程启动全部服务。

---

## 实时链路和异步链路

### 实时链路

实时链路必须低延迟，主要处理：
- SIP 事件
- FreeSWITCH ESL 事件
- originate / bridge / hangup
- 坐席状态
- 呼入分配
- 选号

流程：FS Event → cc-call → Workflow → ESL Command

### 异步链路

异步链路不应该阻塞通话：
- CDR 落库
- 计费
- 录音上传
- 报表投影
- 下游回调

流程：CHANNEL_HANGUP_COMPLETE → call_center_cdr_queue → cc-worker → fanout outbox nodes

---

## FreeSWITCH 事件租约

多实例 `cc-call` 不能重复消费同一 FreeSWITCH 节点事件，因此引入租约机制。

### 核心表

`cc_res_fs_lease`

| 字段 | 说明 |
| --- | --- |
| fs_addr | FreeSWITCH ESL 地址 |
| owner | 当前事件消费者 |
| lease_expiry | 租约过期时间 |

`owner` 包含 `SERVICE_INSTANCE_ID`，避免本机多个实例抢同一租约。

### 租约抢占

- 多个实例尝试获取同一 FS 的租约
- 只有一个实例成功
- 失败实例等待租约过期后重新争抢

---

## 呼叫记录必达

云枢声讯的业务规则：
> 只要呼叫进入云枢声讯会话生命周期，并最终收到 `CHANNEL_HANGUP_COMPLETE`，就必须写入 CDR outbox。

该规则由单元测试覆盖所有核心 profile：
- API 外呼
- 云枢声讯直呼
- 客户呼入
- 批量外呼
- 预测外呼
- 协同外呼

---

## 故障隔离

| 故障 | 影响范围 | 设计处理 |
| --- | --- | --- |
| 下游 Webhook 失败 | 不影响通话 | outbox 重试 |
| 录音上传失败 | 不影响通话 | 录音任务重试 |
| ESL broken pipe | 当前命令可能失败 | 清理连接并重试一次 |
| Redis 短暂异常 | 状态/选号/队列受影响 | 日志告警，部分路径降级 |
| Worker 停止 | 后置任务堆积 | outbox 保留，恢复后投递 |

---

## 相关代码索引

| 功能 | 文件位置 |
| --- | --- |
| 应用入口 | `internal/app/` |
| 服务启动 | `cmd/` |
| 配置管理 | `internal/infra/config/` |
