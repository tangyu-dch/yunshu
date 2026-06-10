---
title: 本地部署
order: 1
---

# 本地部署

## 启动基础设施

```bash
docker compose up -d mysql redis rtpengine freeswitch kamailio
```

## 启动后端

```bash
SERVICE_INSTANCE_ID=local-main go run ./cmd/cc-all -config configs/default.yaml
```

## 健康检查

```bash
curl http://127.0.0.1:8082/healthz
```

## 注意事项

macOS Docker Desktop 的 UDP 路由可能影响 SIPp UAS。云枢声讯完整端到端建议在 Linux 或 Docker 内运行 SIPp。
