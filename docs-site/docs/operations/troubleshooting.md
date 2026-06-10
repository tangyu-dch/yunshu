---
title: 日志与排障
order: 2
---

# 日志与排障

## FreeSWITCH ESL broken pipe

现象：

```text
write: broken pipe
```

处理：

- 确认只运行一个本地 `cc-all/cc-call`
- 设置 `SERVICE_INSTANCE_ID`
- 新版本会自动清理死连接并重连一次

## 云枢声讯无法完整端到端

如果客户 UAS 报：

```text
Unable to send UDP message: No route to host
```

通常是本地 Docker Desktop UDP 路由问题。建议：

- Linux 服务器验证
- Docker 网络内运行 SIPp UAS
- 检查 `LOCAL_IP`

## 呼入无事件进入云枢声讯

检查：

- `EventFromESL` 是否能生成 callId
- FreeSWITCH 是否上报 `CHANNEL_CREATE`
- `SessionSniffer` 是否能识别 DID

## 坐席腿 UNALLOCATED_NUMBER

通常是 Kamailio location 域不匹配。

检查：

```sql
SELECT username, domain, contact FROM cc_res_location WHERE username='1001';
```

确保 R-URI 使用：

```text
1001@sip.merchant.yunshu.com
```
