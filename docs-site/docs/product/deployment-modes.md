---
title: 部署模式
order: 3
---

# 部署模式

## 单商户私有化

适合企业内部客服中心或自有电销团队。

```yaml
tenant:
  mode: single
  defaultMerchantId: 1001
```

特点：

- 默认商户自动补齐
- 资源配置更简单
- 可隐藏多租户运营入口

## SaaS 多商户

适合平台型服务商。

```yaml
tenant:
  mode: multi
```

特点：

- 分机、号码池、技能组、网关按商户隔离
- 账务、余额、费率独立
- API 鉴权必须启用 AppKey/AppSecret

## 开发一体化

```bash
go run ./cmd/cc-all -config configs/default.yaml
```

## 生产拆分部署

```text
cc-console x N
cc-edge x N
cc-call x N
cc-worker x N
FreeSWITCH x N
Kamailio x N
RTPEngine x N
```

`cc-call` 多实例时必须设置唯一：

```bash
SERVICE_INSTANCE_ID=<instance-id>
```
