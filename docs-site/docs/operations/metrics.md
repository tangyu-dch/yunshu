---
title: 监控指标
order: 3
---

# 监控指标

建议监控以下维度：

| 指标 | 说明 |
| --- | --- |
| 活跃通话数 | 当前未完成会话数量 |
| ESL 连接状态 | cc-call 到 FreeSWITCH 的连接 |
| FS 事件租约 | `cc_res_fs_lease` owner 和过期时间 |
| 分机状态 | Redis `extension:status` |
| 选号并发 | Redis 选号 claim key |
| 队列长度 | `cti:merchant:*:queue:skill_group:*` |
| Outbox 堆积 | 待投递 outbox 数量 |
| CDR 成功率 | CDR outbox 到 `call_cdr_record` 的成功率 |

## 日志字段

话务日志应包含：

- callId
- uuid
- fsAddr
- legRole
- commandId
- profile
- hangupCause
