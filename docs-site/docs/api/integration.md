---
title: 第三方接入
order: 6
---

# 第三方接入

## API 外呼接入

```bash
curl -X POST 'http://cc-call:8082/cti/callTask/call?callId=call-001' \
  -H 'Content-Type: application/json' \
  -H 'X-App-Key: <app-key>' \
  -H 'X-App-Secret: <app-secret>' \
  -d '{"userId":2094,"callee":"13800001111"}'
```

## 回调接收

配置：

```yaml
worker:
  downstream:
    url: "https://merchant.example.com/yunshu/cdr"
    secret: "shared-secret"
```

## 幂等

调用方建议使用稳定 `callId`：

```text
业务订单号 / 客户任务号 / UUID
```

云枢声讯 内部会使用 callId 关联：

- ESL 命令
- FreeSWITCH UUID
- CDR
- 计费流水
- 回调任务

## 安全建议

- 使用 HTTPS
- 使用 AppKey/AppSecret
- 校验 HMAC 签名
- 下游回调必须幂等
