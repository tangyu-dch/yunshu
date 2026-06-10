---
title: 环境变量
order: 6
---

# 环境变量

| 变量 | 说明 |
| --- | --- |
| `ADDR` | 单服务监听地址 |
| `CONFIG` | 配置文件路径 |
| `MYSQL_DSN` | MySQL DSN |
| `SERVICE_INSTANCE_ID` | 服务实例 ID，影响 FS 事件租约 owner |
| `CC_CALL_INSTANCE_ID` | cc-call 实例 ID |
| `POD_NAME` | Kubernetes Pod 名，可作为实例 ID |
| `CALLBACK_URL` | 批量/业务回调地址 |
| `DOWNSTREAM_CDR_URL` | CDR 下游推送地址 |
| `RECORDING_UPLOAD_URL` | 录音上传地址 |
| `WORKER_BILLING_DEFAULT_RATE_PER_MIN` | 默认计费费率 |
| `SIP_CREDENTIAL_KEY` | SIP 凭证加密 key |
| `PHONE_NUMBER_KEY` | 电话号码加密 key |

## 本地推荐

```bash
SERVICE_INSTANCE_ID=local-main go run ./cmd/cc-all -config configs/default.yaml
```

## 生产推荐

```bash
SERVICE_INSTANCE_ID=${HOSTNAME}-${POD_NAME}
```

避免同一主机多个 `cc-call` 使用相同 FreeSWITCH 事件租约 owner。
    