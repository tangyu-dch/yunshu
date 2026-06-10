---
title: WebSocket
order: 3
---

# WebSocket

## CTI WebSocket

```http
GET /cti/ws?merchantId=1001&taskId=1
```

用于批量外呼任务状态推送。服务端订阅 Redis：

```text
cti_websocket_push_event
```

并读取 Redis 投影 hash 后推送给前端。

## ASR WebSocket

FreeSWITCH `mod_audio_stream` 可将 PCM 音频推送到 云枢声讯 ASR 服务，用于 AI 语音流程。
