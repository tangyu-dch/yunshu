---
title: 数据库表
order: 7
---

# 数据库表

## 话务资源

| 表 | 说明 |
| --- | --- |
| `cc_res_extension` | 分机账号、SIP 鉴权 |
| `cc_res_location` | Kamailio usrloc 位置表 |
| `cc_res_freeswitch` | Kamailio dispatcher 目标 |
| `cc_res_freeswitch_node` | FreeSWITCH ESL 节点 |
| `cc_res_pool_phone` | 号码池号码 |
| `cc_res_skill_group` | 技能组 |
| `cc_res_user_skill_group` | 用户技能组关系 |

## 呼叫记录

| 表 | 说明 |
| --- | --- |
| `call_cdr_record` | CDR 主记录 |
| `cc_biz_ledger` | 计费流水 |
| `call_billing_settlement_job` | 结算任务 |
| `cc_biz_recording` | 录音任务 |
| `call_report_projection` | 报表投影 |
| `call_downstream_push_job` | 下游推送任务 |

## Outbox

| 表 | 说明 |
| --- | --- |
| `message_outbox` | 可靠异步任务表 |

## Kamailio 注意事项

`cc_res_location.methods` 需要允许 NULL，以兼容 Kamailio usrloc 写回。

```sql
ALTER TABLE cc_res_location MODIFY methods INT DEFAULT NULL;
```
