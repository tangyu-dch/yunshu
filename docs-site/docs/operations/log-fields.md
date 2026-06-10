---
title: 日志字段
order: 5
---

# 日志字段

话务日志建议统一包含以下字段。

| 字段 | 说明 |
| --- | --- |
| callId | 业务通话 ID |
| uuid | FreeSWITCH 通道 UUID |
| fsAddr | FreeSWITCH ESL 地址 |
| legRole | agent/customer |
| profile | api_outbound/api_direct/inbound/batch_outbound |
| commandId | ESL 命令幂等 ID |
| eventName | FreeSWITCH 事件名 |
| hangupCause | 挂断原因 |
| sipHangupDisposition | SIP 挂断方向 |
| merchantId | 商户 ID |
| userId | 坐席用户 ID |
| extension | 分机号 |

## 示例

```text
callId=xxx uuid=yyy fsAddr=192.168.107.6:8021 legRole=agent eventName=CHANNEL_ANSWER profile=inbound
```
