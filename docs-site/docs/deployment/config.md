---
title: 配置说明
order: 5
---

# 配置说明

主要配置文件：

```text
configs/default.yaml
```

## 关键项

```yaml
service:
  name: cc-call
  addr: :8080
  instanceId: local-main

mysql:
  dsn: root:db123456@tcp(127.0.0.1:3306)/yunshu?charset=utf8mb4&parseTime=True&loc=Local

redis:
  addrs:
    - 127.0.0.1:63790

freeswitch:
  kamailioAddr: "192.168.107.2:5060"
  eventLeaseTTL: 30s
```

## 环境变量

| 变量 | 说明 |
| --- | --- |
| ADDR | 覆盖服务监听地址 |
| SERVICE_INSTANCE_ID | 覆盖服务实例 ID |
| CC_CALL_INSTANCE_ID | cc-call 实例 ID |
| POD_NAME | Kubernetes 实例 ID |
| MYSQL_DSN | 覆盖 MySQL DSN |
