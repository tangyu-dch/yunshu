---
title: Webhook 回调
order: 4
---

# Webhook 回调

云枢声讯的回调和下游推送由 cc-worker 通过 outbox 投递。

## CDR 下游推送

配置：

```yaml
worker:
  downstream:
    url: "https://example.com/cdr"
    secret: "your-secret"
```

## 回调安全

建议使用 HMAC-SHA256 签名校验。所有失败会进入 outbox 重试，不应在主呼叫流程内阻塞。
