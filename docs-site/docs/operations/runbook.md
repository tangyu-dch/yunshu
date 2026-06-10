---
title: Runbook
order: 4
---

# Runbook

## 呼入失败

1. 检查 FreeSWITCH 是否收到 INVITE：

```bash
docker logs cc-freeswitch | grep CHANNEL_CREATE
```

2. 检查 cc-call 是否收到事件：

```bash
grep "收到 FreeSWITCH 事件" logs
```

3. 检查 DID 是否存在：

```sql
SELECT pp.phone, p.merchant_id
FROM cc_res_pool_phone pp
JOIN cc_tel_pool p ON p.id = pp.pool_id
WHERE pp.phone = '01088886666';
```

4. 检查坐席状态：

```bash
redis-cli HGETALL extension:status
```

## 云枢声讯直呼失败

1. 检查分机是否 REGISTER。
2. 检查 `api_direct` 是否被捕获。
3. 检查选号候选是否存在。
4. 检查客户 UAS/网关是否可达。

## API 外呼失败

1. 确认请求 `cc-call:8082`。
2. 确认 URL 有 `callId`。
3. 确认 userId 有绑定分机。
4. 确认商户余额和分机状态正常。

## CDR 缺失

1. 检查 session 是否收到 `CHANNEL_HANGUP_COMPLETE`。
2. 检查 outbox 是否有 `cdr:<callId>`。
3. 检查 worker 是否运行。
4. 检查 `call_cdr_record` 是否已落库。
